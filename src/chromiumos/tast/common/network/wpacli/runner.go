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
		return errors.Errorf("failed to detect 'OK' in the output of 'wpa_cli bssid_ignore clear', output: %s", string(cmdOut))
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
		return errors.Errorf("failed to detect 'OK' in the output of 'wpa_cli set %s %s', output: %s", string(prop), val, string(cmdOut))
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
		return errors.Errorf("failed to detect 'OK' in the output of 'wpa_cli remove_network all', output: %s", string(cmdOut))
	}
	return nil
}

// addNetwork adds a wpa_supplicant network and returns the network ID.
func (r *Runner) addNetwork(ctx context.Context) (int, error) {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("add_network")...)
	if err != nil {
		return -1, errors.Wrap(err, "failed running wpa_cli add_network")
	}
	lines := strings.Split(string(cmdOut), "\n")
	if len(lines) < 2 {
		return -1, errors.Wrap(err, "invalid output of 'wpa_cli add_network' commmand")
	}
	netID, err := strconv.Atoi(lines[1])
	if err != nil {
		return -1, err
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
		return errors.Errorf("failed to detect 'OK' in the output of 'wpa_cli set_network %d %s %s', output: %s", networkID, variable, value, string(cmdOut))
	}
	return nil
}

// NetworkSSID returns the ssid of the network with ID network_id.
func (r *Runner) NetworkSSID(ctx context.Context, networkID int, iface string) (string, error) {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("-i", iface, "get_network", strconv.Itoa(networkID), "ssid")...)
	if err != nil {
		return "", errors.Wrapf(err, "failed running wpa_cli get_network %d ssid", networkID)
	}

	return strings.Trim(string(cmdOut), "\""), nil
}

// statusMap returns a generated status key/value map from the output of the WiFi interface.
func (r *Runner) statusMap(ctx context.Context, iface string) (map[string]string, error) {
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
	statusMap, err := r.statusMap(ctx, iface)
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

// scan requests new BSS scan.
func (r *Runner) scan(ctx context.Context) error {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("scan")...)
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli scan")
	}
	if !strings.Contains(string(cmdOut), "OK") {
		return errors.Errorf("failed to detect 'OK' in the output of 'wpa_cli scan', output: %s", string(cmdOut))
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

// WaitForConnected waits for the network association.
func (r *Runner) WaitForConnected(ctx context.Context, ssid, iface string) error {
	associationTimeout := 30 * time.Second
	pollingIntervalSeconds := time.Second
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		isAssoc, err := r.isAssociated(ctx, ssid, iface)
		if err != nil {
			return errors.Wrap(err, "failed to check the associated status")
		}
		if !isAssoc {
			return errors.New("failed status is not assciated")
		}
		return nil
	}, &testing.PollOptions{Timeout: associationTimeout, Interval: pollingIntervalSeconds}); err != nil {
		return err
	}

	return nil
}

// Interfaces returns a list of the available interfaces:.
func (r *Runner) Interfaces(ctx context.Context) ([]string, error) {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("interface")...)
	if err != nil {
		return nil, errors.Wrap(err, "failed running wpa_cli interface")
	}
	var interfaces []string
	startParsing := false
	for _, line := range strings.Split(string(cmdOut), "\n") {
		// Start parsing after the line "Available interfaces:".
		if strings.Contains(line, "Available interfaces:") {
			startParsing = true
			continue
		}
		if startParsing {
			interfaces = append(interfaces, line)
		}
	}

	return interfaces, nil
}

// IsConnected checks that the network is connected.
func (r *Runner) IsConnected(ctx context.Context, iface string) (bool, error) {
	statusMap, err := r.statusMap(ctx, iface)
	if err != nil {
		return false, err
	}
	statusWPAState, ok := statusMap["wpa_state"]
	if !ok {
		return false, errors.Errorf("the wpa_state key was not found in the status map %t", statusMap)
	}
	if statusWPAState == "CONNECTED" {
		return true, nil
	}

	return false, nil
}

// ScanNetwork scans for specific network with SSID and returns nil if the network id found
// in the scan results.
func (r *Runner) ScanNetwork(ctx context.Context, dutConn *ssh.Conn, ssid string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	const wpaMonitorStopTimeout = 2 * time.Second
	wpaMonitor := new(WPAMonitor)
	stop, ctx, err := wpaMonitor.StartWPAMonitor(timeoutCtx, dutConn, wpaMonitorStopTimeout)
	if err != nil {
		return errors.Wrap(err, "faled to start wpa monitor")
	}
	defer stop()

	if err := r.scan(ctx); err != nil {
		return errors.Wrap(err, "failed to trigger scan")
	}

	for {
		event, err := wpaMonitor.WaitForEvent(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to wait for ScanResultsEvent event")
		}
		if event == nil { // timeout
			return errors.Wrap(err, "timeout waiting for the ScanResultsEvent")
		}
		if _, scanSuccess := event.(*ScanResultsEvent); scanSuccess {
			return nil
		}
	}

	results, err := r.scanResults(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the scan results")
	}
	for _, network := range results {
		if network["SSID"] == ssid {
			return nil
		}
	}

	return errors.Errorf("failed the SSID=%s not found in the scan results", ssid)
}
