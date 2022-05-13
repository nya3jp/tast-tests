// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpacli

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/common/network/cmd"
	"chromiumos/tast/errors"
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
