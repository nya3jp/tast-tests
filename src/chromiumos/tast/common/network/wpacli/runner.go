// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpacli

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/network/cmd"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Runner contains methods involving wpa_cli command.
type Runner struct {
	cmd cmd.Runner
}

// NewRunner creates a new wpa_cli command utility runner.
func NewRunner(c cmd.Runner) *Runner {
	return &Runner{cmd: c}
}

// sudoWPACLI returns a sudo command args that runs wpa_cli with args under sudo.
func sudoWPACLI(args ...string) []string {
	ret := []string{"-u", "wpa", "-g", "wpa", "wpa_cli"}
	for _, arg := range args {
		ret = append(ret, arg)
	}
	return ret
}

// Ping runs "wpa_cli -i iface ping" command and expects to see PONG.
func (r *Runner) Ping(ctx context.Context, iface string) ([]byte, error) {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("-i", iface, "ping")...)
	if err != nil {
		return cmdOut, errors.Wrapf(err, "failed running wpa_cli -i %s ping", iface)
	}
	if !strings.Contains(string(cmdOut), "PONG") {
		return cmdOut, errors.New("failed to see 'PONG' in wpa_cli ping output")
	}
	return cmdOut, nil
}

// ClearBSSIDIgnore clears the BSSID_IGNORE list on DUT.
func (r *Runner) ClearBSSIDIgnore(ctx context.Context) error {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("bssid_ignore", "clear")...)
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli bssid_ignore clear")
	}
	if !strings.Contains(string(cmdOut), "OK") {
		return errors.Errorf("failed to expect 'OK' in wpa_cli bssid_ignore clear output: %s", string(cmdOut))
	}
	return nil
}

// Property is a global property for wpa_supplicant
type Property string

const (
	// PropertyNonPrefChan indicates to the AP which channels are not preferred
	PropertyNonPrefChan Property = "non_pref_chan"
)

// NonPrefChan is a single non-preferred channel
type NonPrefChan struct {
	OpClass uint8
	Channel uint8
	Pref    uint8
	Reason  uint8
}

// SerializeNonPrefChans serializes a list of NonPrefChan objects into a wpa_supplicant-recognizable string
func SerializeNonPrefChans(chans ...NonPrefChan) string {
	var s string
	for _, n := range chans {
		s += fmt.Sprintf("%d:%d:%d:%d ", n.OpClass, n.Channel, n.Pref, n.Reason)
	}
	return s
}

// Set sets a specified global wpa_supplicant property to a specified value
func (r *Runner) Set(ctx context.Context, prop Property, val string) error {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("set", string(prop), val)...)
	if err != nil {
		return errors.Wrapf(err, "failed running wpa_cli set %s %s", string(prop), val)
	}
	if !strings.Contains(string(cmdOut), "OK") {
		return errors.Errorf("failed to get 'OK' in wpa_cli set output: %s", string(cmdOut))
	}
	return nil
}

// run runs a specific command and checks for expected response.
func (r *Runner) run(ctx context.Context, expected string, opts ...string) error {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI(opts...)...)
	if err != nil {
		return errors.Wrapf(err, "failed running wpa_cli %s", strings.Join(opts, " "))
	}
	if !strings.Contains(string(cmdOut), expected) {
		return errors.Errorf("failed to get %q in wpa_cli %s output: %s", expected, strings.Join(opts, " "), string(cmdOut))
	}
	return nil
}

// TDLSDiscover runs tdls_discover command.
func (r *Runner) TDLSDiscover(ctx context.Context, mac string) error {
	return r.run(ctx, "OK", "tdls_discover", mac)
}

// TDLSSetup runs tdls_setup command.
func (r *Runner) TDLSSetup(ctx context.Context, mac string) error {
	return r.run(ctx, "OK", "tdls_setup", mac)
}

// TDLSTeardown runs tdls_teardown command.
func (r *Runner) TDLSTeardown(ctx context.Context, mac string) error {
	return r.run(ctx, "OK", "tdls_teardown", mac)
}

// TDLSLinkStatus runs tdls_link_status command.
func (r *Runner) TDLSLinkStatus(ctx context.Context, mac string) error {
	return r.run(ctx, "TDLS link status: connected", "tdls_link_status", mac)
}

// StartSoftAP creates a soft AP on DUT.
func (r *Runner) StartSoftAP(ctx context.Context, freq uint32, ssid, key_mgmt, psk string) error {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("add_network")...)
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli add_network")
	}
	re := regexp.MustCompile(`(?m)^(\d+)`)
	substr := re.FindStringSubmatch(string(cmdOut))
	if len(substr) < 2 {
		return errors.New("no network id found")
	}
	id := substr[1]

	// mode: IEEE 802.11 operation mode, 0 = infrastructure, 1 = IBSS, 2 = AP
	err = r.run(ctx, "OK", "set_network", id, "mode", "2")
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli set_network mode")
	}

	err = r.run(ctx, "OK", "set_network", id, "frequency", strconv.FormatUint(uint64(freq), 10))
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli set_network frequency")
	}

	err = r.run(ctx, "OK", "set_network", id, "ssid", fmt.Sprintf("\"%s\"", ssid))
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli set_network ssid")
	}

	err = r.run(ctx, "OK", "set_network", id, "key_mgmt", key_mgmt)
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli set_network key_mgmt")
	}

	if psk != "" {
		err := r.run(ctx, "OK", "set_network", id, "psk", psk)
		if err != nil {
			return errors.Wrap(err, "failed running wpa_cli set_network psk")
		}

		// WPA2-PSK and WPA3-SAE both use RSN protocol.
		err = r.run(ctx, "OK", "set_network", id, "proto", "RSN")
		if err != nil {
			return errors.Wrap(err, "failed running wpa_cli set_network proto")
		}
	}

	err = r.run(ctx, "OK", "select_network", id)
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli select_network")
	}

	err = r.waitForStatus(ctx, "COMPLETE")
	if err != nil {
		return errors.Wrap(err, "cannot start soft AP")
	}

	return nil
}

// StopSoftAP stops the soft AP on DUT.
func (r *Runner) StopSoftAP(ctx context.Context) error {
	err := r.run(ctx, "OK", "remove_network", "all")
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli remove_network")
	}

	err = r.waitForStatus(ctx, "INACTIVE")
	if err != nil {
		return errors.Wrap(err, "cannot stop soft AP")
	}

	return nil
}

func (r *Runner) waitForStatus(ctx context.Context, status string) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return r.run(ctx, "wpa_state="+status, "status")
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return err
	}
	return nil
}
