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

// StartSoftAP creates a soft AP on DUT.
func (r *Runner) StartSoftAP(ctx context.Context, freq uint32, ssid string, key_mgmt string, psk string) error {
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

	cmdOut, err = r.cmd.Output(ctx, "sudo", sudoWPACLI("set_network", id, "mode", "2")...)
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli set_network mode")
	}
	if !strings.Contains(string(cmdOut), "OK") {
		return errors.Errorf("failed to expect 'OK' in wpa_cli set_network mode output: %s", string(cmdOut))
	}

	cmdOut, err = r.cmd.Output(ctx, "sudo", sudoWPACLI("set_network", id, "frequency", strconv.FormatUint(uint64(freq), 10))...)
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli set_network frequency")
	}
	if !strings.Contains(string(cmdOut), "OK") {
		return errors.Errorf("failed to expect 'OK' in wpa_cli set_network frequency output: %s", string(cmdOut))
	}

	cmdOut, err = r.cmd.Output(ctx, "sudo", sudoWPACLI("set_network", id, "ssid", fmt.Sprintf("\"%s\"", ssid))...)
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli set_network ssid")
	}
	if !strings.Contains(string(cmdOut), "OK") {
		return errors.Errorf("failed to expect 'OK' in wpa_cli set_network ssid output: %s", string(cmdOut))
	}

	cmdOut, err = r.cmd.Output(ctx, "sudo", sudoWPACLI("set_network", id, "key_mgmt", key_mgmt)...)
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli set_network key_mgmt")
	}
	if !strings.Contains(string(cmdOut), "OK") {
		return errors.Errorf("failed to expect 'OK' in wpa_cli set_network key_mgmt output: %s", string(cmdOut))
	}

	if psk != "" {
		cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("set_network", id, "psk", psk)...)
		if err != nil {
			return errors.Wrap(err, "failed running wpa_cli set_network psk")
		}
		if !strings.Contains(string(cmdOut), "OK") {
			return errors.Errorf("failed to expect 'OK' in wpa_cli set_network psk output: %s", string(cmdOut))
		}
	}

	cmdOut, err = r.cmd.Output(ctx, "sudo", sudoWPACLI("select_network", id)...)
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli select_network")
	}
	if !strings.Contains(string(cmdOut), "OK") {
		return errors.Errorf("failed to expect 'OK' in wpa_cli select_network output: %s", string(cmdOut))
	}

	err = r.waitForStatus(ctx, "COMPLETE")
	if err != nil {
		return errors.Wrap(err, "cannot start soft AP")
	}

	return nil
}

// StopSoftAP stops the soft AP on DUT.
func (r *Runner) StopSoftAP(ctx context.Context) error {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("remove_network", "all")...)
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli remove_network")
	}
	if !strings.Contains(string(cmdOut), "OK") {
		return errors.Errorf("failed to expect 'OK' in wpa_cli remove_network output: %s", string(cmdOut))
	}

	err = r.waitForStatus(ctx, "INACTIVE")
	if err != nil {
		return errors.Wrap(err, "cannot stop soft AP")
	}

	return nil
}

func (r *Runner) waitForStatus(ctx context.Context, status string) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("status")...)
		if err != nil {
			return errors.Wrap(err, "failed running wpa_cli status")
		}
		if strings.Contains(string(cmdOut), "wpa_state="+status) {
			return nil // success
		}
		return errors.Errorf("wpa_supplicant does not match expected status %s", status)
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return err
	}
	return nil
}
