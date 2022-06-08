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
		return errors.Errorf("failed 'OK' not detected in the output of 'wpa_cli bssid_ignore clear', output: %s", string(cmdOut))
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
		return errors.Errorf("failed 'OK' not detected in the output of 'wpa_cli set %s %s', output: %s", string(prop), val, string(cmdOut))
	}
	return nil
}

// RemoveAllNetworks removes all saved network profiles.
func (r *Runner) RemoveAllNetworks(ctx context.Context) error {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("remove_network", "all")...)
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli remove_network all")
	}
	if !strings.Contains(string(cmdOut), "OK") {
		return errors.Errorf("failed 'OK' not detected in the output of 'wpa_cli remove_network all', output: %s", string(cmdOut))
	}
	return nil
}

// addNetwork adds a wpa_supplicant network and returns the network ID.
func (r *Runner) addNetwork(ctx context.Context) (int, error) {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("add_network")...)
	if err != nil {
		return 0, errors.Wrap(err, "failed running wpa_cli add_network")
	}
	lines := strings.Split(string(cmdOut), "\n")
	if len(lines) < 2 {
		return 0, errors.Wrap(err, "failed invalid output of 'wpa_cli add_network' commmand")
	}
	netID, err := strconv.Atoi(lines[1])
	if err != nil {
		return 0, err
	}
	return netID, nil
}

// setNetwork sets a wpa_supplicant network variable.
func (r *Runner) setNetwork(ctx context.Context, networkID int, variable, value string) error {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("set_network", strconv.Itoa(networkID), variable, value)...)
	if err != nil {
		return errors.Wrapf(err, "failed running wpa_cli set_network %d %s %s", networkID, variable, value)
	}
	if !strings.Contains(string(cmdOut), "OK") {
		return errors.Errorf("failed 'OK' not detected in the output of 'wpa_cli set_network %d %s %s', output: %s", networkID, variable, value, string(cmdOut))
	}
	return nil
}

// GetNetworkSSID returns the ssid of the network with ID network_id.
func (r *Runner) GetNetworkSSID(ctx context.Context, networkID int, iface string) (string, error) {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("-i", iface, "get_network", strconv.Itoa(networkID), "ssid")...)
	if err != nil {
		return "", errors.Wrapf(err, "failed running wpa_cli get_network %d ssid", networkID)
	}

	return strings.Trim(string(cmdOut), "\""), nil
}

// getStatusMap returns a generated status key/value map from the output of the WiFi interface.
func (r *Runner) getStatusMap(ctx context.Context, iface string) (map[string]string, error) {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("-i", iface, "status")...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed running wpa_cli -i %s status", iface)
	}
	statusMap := make(map[string]string)
	for _, line := range strings.Split(string(cmdOut), "\n") {
		if strings.Contains(line, "=") {
			statusMap[strings.Split(line, "=")[0]] = strings.Split(line, "=")[1]
		}
	}

	return statusMap, nil
}

// isAssociated checks that the network is associated to a given SSID.
func (r *Runner) isAssociated(ctx context.Context, ssid, iface string) (bool, error) {
	statusMap, err := r.getStatusMap(ctx, iface)
	if err != nil {
		return false, err
	}
	statusSSID, ok := statusMap["ssid"]
	if !ok {
		return false, errors.Errorf("the ssid key was not found in the status map %t", statusMap)
	}
	statusWPAState, ok := statusMap["wpa_state"]
	if !ok {
		return false, errors.Errorf("the wpa_state key was not found in the status map %t", statusMap)
	}
	if (statusSSID == ssid) && (statusWPAState == "COMPLETED") {
		return true, nil
	}

	return false, nil
}

// isConnected checks that the network is connected to |ssid| and have an IP address.
func (r *Runner) isConnected(ctx context.Context, ssid, iface string) (bool, error) {
	statusMap, err := r.getStatusMap(ctx, iface)
	if err != nil {
		return false, err
	}
	statusSSID, ok := statusMap["ssid"]
	if !ok {
		return false, errors.Errorf("the ssid key was not found in the status map %t", statusMap)
	}
	if _, ok := statusMap["ip_address"]; !ok {
		return false, errors.Errorf("the ip_address key was not found in the status map %t", statusMap)
	}
	if statusSSID == ssid {
		return true, nil
	}

	return false, nil
}

// scan requests new BSS scan.
func (r *Runner) scan(ctx context.Context) error {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("scan")...)
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli scan")
	}
	if !strings.Contains(string(cmdOut), "OK") {
		return errors.Errorf("failed 'OK' not detected in the output of 'wpa_cli scan', output: %s", string(cmdOut))
	}
	return nil
}

// scanResults returns latest scan results.
func (r *Runner) scanResults(ctx context.Context) ([]map[string]string, error) {
	var networks []map[string]string
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("scan_results")...)
	if err != nil {
		return networks, errors.Wrap(err, "failed running wpa_cli scan_results")
	}
	scanResultsPattern := strings.Join([]string{
		"(?P<BSSID>[0-9a-f:]{17})", // BSSID
		"(?P<Frequency>[0-9]+)",    // Frequency
		"(?P<SignalLevel>-[0-9]+)", // Signal level
		"(?P<EncryptionTypes>.*)",  // Encryption types
		"(?P<SSID>.*)"},            // SSID
		"\t")
	compRegEx := regexp.MustCompile(scanResultsPattern)
	for _, line := range strings.Split(string(cmdOut), "\n") {
		if match := compRegEx.FindStringSubmatch(line); match != nil {
			paramsMap := make(map[string]string)
			for i, name := range compRegEx.SubexpNames() {
				paramsMap[name] = match[i]
			}
			networks = append(networks, paramsMap)
		}
	}
	return networks, nil
}

// WaitForConnected waits for the network association and configuration.
func (r *Runner) WaitForConnected(ctx context.Context, ssid, iface string) error {
	associationTimeout := 30 * time.Second
	configurationTimeout := 30 * time.Second
	pollingIntervalSeconds := time.Second
	err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		isAssoc, err := r.isAssociated(ctx, ssid, iface)
		if err != nil {
			testing.ContextLogf(ctx, "Failed to check the associated status (%v), Retrying", err)
		}
		if !isAssoc {
			return errors.New("failed status is not assciated")
		}
		return err
	}, &testing.PollOptions{Timeout: associationTimeout, Interval: pollingIntervalSeconds})
	if err != nil {
		return err
	}

	err = testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		isConn, err := r.isConnected(ctx, ssid, iface)
		if err != nil {
			testing.ContextLogf(ctx, "Failed to check the connetced status (%v), Retrying", err)
		}
		if !isConn {
			return errors.New("failed status is not connected")
		}
		return err
	}, &testing.PollOptions{Timeout: configurationTimeout, Interval: pollingIntervalSeconds})
	if err != nil {
		return err
	}

	return nil
}

// P2PGroupAdd add a new P2P group (local end as GO).
func (r *Runner) P2PGroupAdd(ctx context.Context) error {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("p2p_group_add")...)
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli p2p_group_add")
	}
	if !strings.Contains(string(cmdOut), "OK") {
		return errors.Errorf("failed to expect 'OK' in wpa_cli p2p_group_add output: %s", string(cmdOut))
	}
	return nil
}

// P2PGroupAddPersistent connect to a P2P GO device.
func (r *Runner) P2PGroupAddPersistent(ctx context.Context) error {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("p2p_group_add", "persistent=0")...)
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli p2p_group_add persistent=0")
	}
	if !strings.Contains(string(cmdOut), "OK") {
		return errors.Errorf("failed to expect 'OK' in wpa_cli p2p_group_add persistent=0 output: %s", string(cmdOut))
	}
	return nil
}

// P2PGroupRemove removes P2P group interface (local end as GO).
func (r *Runner) P2PGroupRemove(ctx context.Context, iface string) error {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("p2p_group_remove", iface)...)
	if err != nil {
		return errors.Wrapf(err, "failed running wpa_cli p2p_group_remove %s", iface)
	}
	if !strings.Contains(string(cmdOut), "OK") {
		return errors.Errorf("failed to expect 'OK' in wpa_cli p2p_group_remove %s output: %s", iface, string(cmdOut))
	}
	return nil
}

// P2PFlush flush P2P state.
func (r *Runner) P2PFlush(ctx context.Context) error {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("p2p_flush")...)
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli p2p_flush")
	}
	if !strings.Contains(string(cmdOut), "OK") {
		return errors.Errorf("failed to expect 'OK' in wpa_cli p2p_flush output: %s", string(cmdOut))
	}
	return nil
}

// P2PGetGOPassphrase get the passphrase for a group (GO only).
func (r *Runner) P2PGetGOPassphrase(ctx context.Context, iface string) (string, error) {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("p2p_get_passphrase", iface)...)
	if err != nil {
		return "", errors.Wrapf(err, "failed running wpa_cli p2p_get_passphrase %s", iface)
	}
	lines := strings.Split(string(cmdOut), "\n")
	return strings.TrimSuffix(lines[1], "\n"), nil
}

// P2PAddGONetwork adds the GO network in the client device.
func (r *Runner) P2PAddGONetwork(ctx context.Context, ssid, passphrase string) error {
	networkID, err := r.addNetwork(ctx)
	if err != nil {
		return err
	}
	if err := r.setNetwork(ctx, networkID, "ssid", strconv.Quote(ssid)); err != nil {
		return err
	}
	if err := r.setNetwork(ctx, networkID, "psk", strconv.Quote(passphrase)); err != nil {
		return err
	}
	if err := r.setNetwork(ctx, networkID, "disabled", "2"); err != nil {
		return err
	}

	return nil
}

// P2PScanNetwork scans for the p2p network.
func (r *Runner) P2PScanNetwork(ctx context.Context, ssid string) error {
	discoveryTimeout := 120 * time.Second
	pollingIntervalSeconds := time.Second
	if err := r.scan(ctx); err != nil {
		return err
	}
	foundSSID := false
	err := testing.Poll(ctx, func(ctx context.Context) error {
		results, err := r.scanResults(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get the scan results")
		}
		for _, network := range results {
			if network["SSID"] == ssid {
				foundSSID = true
			}
		}
		if foundSSID {
			return nil
		}
		if err := r.scan(ctx); err != nil {
			return errors.Wrap(err, "failed to run scan")
		}
		return errors.Errorf("failed the SSID=%s not found in the latest scan results", ssid)
	}, &testing.PollOptions{Timeout: discoveryTimeout, Interval: pollingIntervalSeconds})
	if err != nil {
		return err
	}

	return nil
}
