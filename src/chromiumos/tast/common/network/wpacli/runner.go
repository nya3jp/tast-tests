// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpacli

import (
	"bufio"
	"context"
	"fmt"
	"net"
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

// Scan triggers a scan sequence in wpa_supplicant.
func (r *Runner) Scan(ctx context.Context) error {
	return r.runSimpleCmd(ctx, "scan")
}

// FetchANQP triggers ANQP request for each compatible BSS found during the last scan.
func (r *Runner) FetchANQP(ctx context.Context) error {
	return r.runSimpleCmd(ctx, "fetch_anqp")
}

// runSimpleCmd runs a wpa_cli command without argument.
func (r *Runner) runSimpleCmd(ctx context.Context, cmd string) error {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI(cmd)...)
	if err != nil {
		return errors.Wrapf(err, "failed running wpa_cli %q", cmd)
	}
	if !strings.Contains(string(cmdOut), "OK") {
		return errors.Errorf("failed to get 'OK' in wpa_cli %q output: %s", cmd, string(cmdOut))
	}
	return nil
}

// BSS fetches from wpa_supplicant all the known information about a given BSSID.
func (r *Runner) BSS(ctx context.Context, addr net.HardwareAddr) (map[string]string, error) {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("bss", addr.String())...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed running wpa_cli 'bss %s'", addr)
	}

	bss := make(map[string]string)
	s := bufio.NewScanner(strings.NewReader(string(cmdOut)))
	for s.Scan() {
		line := s.Text()
		if strings.Contains(line, "=") {
			elems := strings.Split(line, "=")
			bss[elems[0]] = elems[1]
		}
	}
	return bss, nil
}
