// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpacli

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/network/cmd"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
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

// AddToBSSIDIgnore adds the passed BSSID into BSSID_IGNORE list on DUT.
func (r *Runner) AddToBSSIDIgnore(ctx context.Context, bssid string) error {
	cmdOut, err := r.cmd.Output(ctx, "sudo", sudoWPACLI("bssid_ignore", bssid)...)
	if err != nil {
		return errors.Wrap(err, "failed running wpa_cli bssid_ignore")
	}
	if !strings.Contains(string(cmdOut), "OK") {
		// Sample output of successful command:
		/*
			Selected interface 'wlan0'
			Ok
		*/
		return errors.Errorf("failed to detect 'OK' in the output of 'wpa_cli bssid_ignore', output: %s", string(cmdOut))
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

// RemoveAllNetworks removes all saved network profiles.
func (r *Runner) RemoveAllNetworks(ctx context.Context) error {
	return r.run(ctx, "OK", "remove_network", "all")
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
	return r.run(ctx, "OK", "set_network", strconv.Itoa(networkID), variable, value)
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

// Scan requests new BSS scan.
func (r *Runner) Scan(ctx context.Context) error {
	return r.run(ctx, "OK", "scan")
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

// checkScanResults checks if the latest scan results has a network with ssid that match the passed ssid.
func (r *Runner) checkScanResults(ctx context.Context, ssid string) error {
	results, err := r.scanResults(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the scan results")
	}
	for _, network := range results {
		if network["SSID"] == ssid {
			return nil
		}
	}

	return errors.Errorf("failed to find SSID=%s in the scan results", ssid)
}

// DiscoverNetwork scans for specific network with SSID and returns nil if the network is found
// in the scan results.
func (r *Runner) DiscoverNetwork(ctx context.Context, dutConn *ssh.Conn, ssid string) error {
	const wpaMonitorStopTimeout = 2 * time.Second
	wpaMonitor := new(WPAMonitor)
	stop, ctx, err := wpaMonitor.StartWPAMonitor(ctx, dutConn, wpaMonitorStopTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to start wpa monitor")
	}
	defer stop()

	if err := r.checkScanResults(ctx, ssid); err == nil {
		return nil
	}

	const waitForScanResultsTimeout = 10 * time.Second
	scanOnce := func(ctx context.Context, ssid string) error {
		if err := r.Scan(ctx); err != nil {
			// Don't fail if the scan command failed due to the radio being busy.
			if !strings.Contains(fmt.Sprint(err), "FAIL-BUSY") {
				return errors.Wrap(err, "failed to trigger scan")
			}
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			event, err := wpaMonitor.WaitForEvent(ctx)
			if err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to wait for ScanResultsEvent"))
			}
			if event == nil { // timeout
				return testing.PollBreak(errors.New("timed out waiting for ScanResultsEvent"))
			}
			if _, scanSuccess := event.(*ScanResultsEvent); scanSuccess {
				return nil
			}

			return errors.New("no ScanResultsEvent found")
		}, &testing.PollOptions{Timeout: waitForScanResultsTimeout}); err != nil {
			return err
		}

		if err := r.checkScanResults(ctx, ssid); err != nil {
			return err
		}

		return nil
	}

	var scanErr error
	const retryScan = 2
	for scan := 0; scan < retryScan; scan++ {
		err := scanOnce(ctx, ssid)
		if err == nil {
			return nil
		}
		scanErr = err
	}

	return scanErr
}

// P2PGroupAdd add a new P2P group (local end as GO).
func (r *Runner) P2PGroupAdd(ctx context.Context) error {
	return r.run(ctx, "OK", "p2p_group_add")
}

// P2PGroupAddPersistent connects to a P2P GO device.
func (r *Runner) P2PGroupAddPersistent(ctx context.Context) error {
	// persistent=0: Specify a restart of a persistent group (connect to an existing persistent group).
	return r.run(ctx, "OK", "p2p_group_add", "persistent=0")
}

// P2PGroupRemove removes P2P group interface (local end as GO).
func (r *Runner) P2PGroupRemove(ctx context.Context, iface string) error {
	return r.run(ctx, "OK", "p2p_group_remove", iface)
}

// P2PFlush flush P2P state.
func (r *Runner) P2PFlush(ctx context.Context) error {
	return r.run(ctx, "OK", "p2p_flush")
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
	// disabled=2: Indicate special network block use as a P2P persistent group information.
	if err := r.setNetwork(ctx, networkID, "disabled", "2"); err != nil {
		return err
	}

	return nil
}

func (r *Runner) fetchANQP(ctx context.Context) error {
	return r.run(ctx, "OK", "fetch_anqp")
}

// FetchANQP triggers ANQP request for each compatible BSS found during the last scan.
func (r *Runner) FetchANQP(ctx context.Context, dutConn *ssh.Conn, bssid string) error {
	const wpaMonitorStopTimeout = 2 * time.Second
	wpaMonitor := new(WPAMonitor)
	stop, ctx, err := wpaMonitor.StartWPAMonitor(ctx, dutConn, wpaMonitorStopTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to start wpa monitor")
	}
	defer stop()

	if err := r.fetchANQP(ctx); err != nil {
		return errors.Wrap(err, "failed to trigger ANQP fetch")
	}

	const waitForANQPQueryDoneTimeout = 30 * time.Second
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		event, err := wpaMonitor.WaitForEvent(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to wait for ANQPQueryDoneEvent"))
		}
		if event == nil { // timeout
			return testing.PollBreak(errors.New("timed out waiting for ANQPQueryDoneEvent"))
		}
		e, ok := event.(*ANQPQueryDoneEvent)
		if !ok {
			return errors.New("no ANQPQueryDoneEvent found")
		}
		if e.Addr != bssid {
			return errors.New("ANQPQueryDoneEvent for another BSSID")
		}
		if !e.Success {
			return testing.PollBreak(errors.Errorf("ANQP query failed: %v", e))
		}
		return nil
	}, &testing.PollOptions{Timeout: waitForANQPQueryDoneTimeout}); err != nil {
		return err
	}
	return nil
}

// StartSoftAP creates a soft AP on DUT.
func (r *Runner) StartSoftAP(ctx context.Context, freq uint32, ssid, keyMgmt, psk, cipher string) error {
	id, err := r.addNetwork(ctx)
	if err != nil {
		return err
	}

	// mode: IEEE 802.11 operation mode, 0 = infrastructure, 1 = IBSS, 2 = AP
	if err := r.setNetwork(ctx, id, "mode", "2"); err != nil {
		return errors.Wrap(err, "failed running wpa_cli set_network mode")
	}
	if err := r.setNetwork(ctx, id, "frequency", strconv.FormatUint(uint64(freq), 10)); err != nil {
		return errors.Wrap(err, "failed running wpa_cli set_network frequency")
	}
	if err := r.setNetwork(ctx, id, "ssid", fmt.Sprintf("\"%s\"", ssid)); err != nil {
		return errors.Wrap(err, "failed running wpa_cli set_network ssid")
	}
	if err := r.setNetwork(ctx, id, "key_mgmt", keyMgmt); err != nil {
		return errors.Wrap(err, "failed running wpa_cli set_network key_mgmt")
	}

	if psk != "" {
		if err := r.setNetwork(ctx, id, "psk", fmt.Sprintf("\"%s\"", psk)); err != nil {
			return errors.Wrap(err, "failed running wpa_cli set_network psk")
		}

		// WPA2-PSK and WPA3-SAE both use RSN protocol.
		if err := r.setNetwork(ctx, id, "proto", "RSN"); err != nil {
			return errors.Wrap(err, "failed running wpa_cli set_network proto")
		}
	}

	if cipher != "" {
		if err := r.setNetwork(ctx, id, "pairwise", cipher); err != nil {
			return errors.Wrapf(err, "failed running wpa_cli set_network pairwise %s", cipher)
		}
		if err := r.setNetwork(ctx, id, "group", cipher); err != nil {
			return errors.Wrapf(err, "failed running wpa_cli set_network group %s", cipher)
		}
	}

	if strings.Contains(keyMgmt, "SAE") {
		if err := r.setNetwork(ctx, id, "ieee80211w", "2"); err != nil {
			return errors.Wrap(err, "failed running wpa_cli set_network ieee80211w")
		}
	}

	if err := r.run(ctx, "OK", "select_network", strconv.Itoa(id)); err != nil {
		return errors.Wrap(err, "failed running wpa_cli select_network")
	}

	if err := r.waitForStatus(ctx, "COMPLETE"); err != nil {
		return errors.Wrap(err, "cannot start soft AP")
	}

	return nil
}

// StopSoftAP stops the soft AP on DUT.
func (r *Runner) StopSoftAP(ctx context.Context) error {
	if err := r.RemoveAllNetworks(ctx); err != nil {
		return errors.Wrap(err, "failed running wpa_cli remove_network")
	}

	if err := r.waitForStatus(ctx, "INACTIVE"); err != nil {
		return errors.Wrap(err, "cannot stop soft AP")
	}

	return nil
}

func (r *Runner) waitForStatus(ctx context.Context, status string) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return r.run(ctx, "wpa_state="+status, "status")
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 2 * time.Second}); err != nil {
		return err
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
