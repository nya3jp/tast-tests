// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package iw contains local Tast tests that exercise the Chrome OS network stack.
package iw

/*
This file serves as a wrapper to allow tast tests to query 'iw' for network device characteristics.
iw_runner.go is the analog of {@link iw_runner.py} in the autotest suite.
*/

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// iw command formatting constants
const (
	iwTimeCommandFormat      = "(time -p %s) 2>&1"
	iwTimeCommandOutputStart = "real"
)

// High throughput modes
const (
	ht20      = "HT20"
	ht40Above = "HT40+"
	ht40Below = "HT40-"
)

// High throughput lookup table
var htTable = map[string]string{
	"no secondary": ht20,
	"above":        ht40Above,
	"below":        ht40Below,
}

// iw security options
const (
	securityOpen  = "open"
	securityWep   = "wep"
	securityWpa   = "wpa"
	securityWpa2  = "wpa2"
	securityMixed = "mixed"
)

// BssData struct that contains contents pertaining to the BSS
type BssData struct {
	Bss       string
	Frequency int
	Ssid      string
	Security  string
	Ht        string
	Signal    float64
}

// TimedScanData struct that contains time of execution and BSS contents
type TimedScanData struct {
	Time    float64
	BssList []*BssData
}

/*
Parses link or station dump output for link key value pairs.

Link or station dump information is in the format below:

Connected to 74:e5:43:10:4f:c0 (on wlan0)
      SSID: PMKSACaching_4m9p5_ch1
      freq: 5220
      RX: 5370 bytes (37 packets)
      TX: 3604 bytes (15 packets)
      signal: -59 dBm
      tx bitrate: 13.0 MBit/s MCS 1

      bss flags:      short-slot-time
      dtim period:    5
      beacon int:     100

@param linkInformation: string containing the raw link or station dump
    information as reported by iw. Note that this parsing assumes a single
    entry, in the case of multiple entries (e.g. listing stations from an
    AP, or listing mesh peers), the entries must be split on a per
    peer/client basis before this parsing operation.
@return a map containing all the link key/value pairs.

*/
func getAllLinkKeys(linkInformation string) map[string]string {
	linkKeyValuePairs := make(map[string]string)
	r := regexp.MustCompile("^[[:space:]]+(.*):[[:space:]]+(.*)$")
	for _, linkKey := range strings.Split(linkInformation, "\n") {
		if r.MatchString(linkKey) {
			matchGroup := r.FindStringSubmatch(linkKey)
			linkKeyValuePairs[matchGroup[1]] = matchGroup[2]
		}
	}
	return linkKeyValuePairs
}

/*
Get the BSSID that |interfaceName| is associated with.

See doc for getAllLinkKeys() for expected format of the station or link
information entry.

@param linkInformation: string containing the raw link or station dump
    information as reported by iw. Note that this parsing assumes a single
    entry, in the case of multiple entries (e.g. listing stations from an AP
    or listing mesh peers), the entries must be split on a per peer/client
    basis before this parsing operation.
@param interfaceName: string name of interface (e.g. 'wlan0').
@param stationDump: boolean indicator of whether the link information is
    from a 'station dump' query. If False, it is assumed the string is from
    a 'link' query.
@return string bssid of the current association, or None if no matching
    association information is found.

*/
func extractBssid(linkInformation string, interfaceName string, stationDump bool) string {

	// We're looking for a line like this when parsing the output of a 'link'
	// query:
	// Connected to 04:f0:21:03:7d:bb (on wlan0)
	// We're looking for a line like this when parsing the output of a
	// 'station dump' query:
	// Station 04:f0:21:03:7d:bb (on mesh-5000mhz)
	identifier := func() string {
		if stationDump {
			return "Station"
		}
		return "Connected to"
	}()
	searchRe := regexp.MustCompile(fmt.Sprintf(`%s ([0-9a-fA-F:]{17}) \(on %s\)`,
		identifier, interfaceName))
	matchGroup := searchRe.FindStringSubmatch(linkInformation)
	if len(matchGroup) == 0 {
		return ""
	}
	return matchGroup[1]
}

/*
Runs a shell command over ssh and reports the binary output.

clientCommandExec runs in a blocking fashion and will not return until the shell command
	itself terminates.
@param shellCommand: string containing the shell command to be sent to the DUT. An example of
	a valid string is "ls -lat"
@return bytestream output of stdout from the DUT.
*/
func clientCommandExec(ctx context.Context, shellCommand string) ([]byte, error) {
	out, err := testexec.CommandContext(ctx, shellCommand).Output()
	return out, err
}

/*
Runner stores metadata to allow its methods to invoke commands on `iw` in a concise manner.

Test code should only have to interface with `iw` through the methods exposed by Runner
*/
type Runner struct {
	Run func(context.Context, string) ([]byte, error) // Function alias that will determine how commands are executed whether the test
	// 	is a client test or a remote test (TODO b/972833).
	HostAddr  string          // Host address for remote tests (TODO b/972833).
	iwCommand string          // Path to invoke `iw`. By default, we expect iw to be in $PATH, so this value should be `iw`.
	LogID     int             // Id for logging.
	s         *testing.State  // Test State
	ctx       context.Context // Test Context
}

/*
NewRunner is a factory to create Runners..
*/
func NewRunner(contxt context.Context, state *testing.State) *Runner {
	return &Runner{
		Run:       clientCommandExec,
		HostAddr:  "",
		iwCommand: "iw",
		s:         state,
		ctx:       contxt,
	}
}

/*
Parse the output of the 'scan' and 'scan dump' commands.

Here is an example of what a single network would look like for
the input parameter.  Some fields have been removed in this example:
  BSS 00:11:22:33:44:55(on wlan0)
  freq: 2447
  beacon interval: 100 TUs
  signal: -46.00 dBm
  Information elements from Probe Response frame:
  SSID: my_open_network
  Extended supported rates: 24.0 36.0 48.0 54.0
  HT capabilities:
  Capabilities: 0x0c
  HT20
  HT operation:
  * primary channel: 8
  * secondary channel offset: no secondary
  * STA channel width: 20 MHz
  RSN: * Version: 1
  * Group cipher: CCMP
  * Pairwise ciphers: CCMP
  * Authentication suites: PSK
  * Capabilities: 1-PTKSA-RC 1-GTKSA-RC (0x0000)

@param output: bytestream command output.

@returns a slice of BssData struct pointers.

*/
func (iwr Runner) parseScanResults(output []byte) []*BssData {
	var bssList = []*BssData{}
	mainBss := BssData{}
	supportedSecurities := []string{}
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		r := regexp.MustCompile("BSS ([0-9a-f:]+)")
		if r.MatchString(line) {
			if mainBss.Bss != "" {
				mainBss.Security = determineSecurity(supportedSecurities)
				bssList = append(bssList, &BssData{mainBss.Bss, mainBss.Frequency, mainBss.Ssid,
					mainBss.Security, mainBss.Ht, mainBss.Signal})
				mainBss = BssData{}
				supportedSecurities = nil
			}
			matchGroup := r.FindStringSubmatch(line)
			mainBss.Bss = matchGroup[1]
		}
		if strings.HasPrefix(line, "freq:") {
			mainBss.Frequency, _ = strconv.Atoi(strings.Split(line, " ")[1])
		}
		if strings.HasPrefix(line, "signal:") {
			mainBss.Signal, _ = strconv.ParseFloat(strings.Split(line, " ")[1], 64)
		}
		if strings.HasPrefix(line, "SSID:") {
			mainBss.Ssid = strings.SplitN(line, ": ", 2)[1]
		}
		if strings.HasPrefix(line, "* secondary channel offset") {
			mainBss.Ht = htTable[strings.TrimSpace(strings.Split(line, ":")[1])]
		}
		if strings.HasPrefix(line, "WPA") {
			supportedSecurities = append(supportedSecurities, "WPA")
		}
		if strings.HasPrefix(line, "RSN") {
			supportedSecurities = append(supportedSecurities, "WPA2")
		}
	}
	mainBss.Security = determineSecurity(supportedSecurities)
	bssList = append(bssList, &mainBss)
	return bssList
}

/*
Parse the scan time in seconds from the output of the 'time -p "scan"' command.

 'time -p' Command output format is below:
 real     0.01
 user     0.01
 sys      0.00

@param output: bytestream command output
@returns float64 time in seconds

*/
func (iwr Runner) parseScanTime(output []byte) float64 {
	outputLines := strings.Split(string(output), "\n")
	for lineNum, line := range outputLines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, iwTimeCommandOutputStart) &&
			strings.HasPrefix(outputLines[lineNum+1], "user") &&
			strings.HasPrefix(outputLines[lineNum+2], "sys") {
			delimiter := func(c rune) bool {
				return c == ' '
			}
			toRet, _ := strconv.ParseFloat(strings.FieldsFunc(line, delimiter)[1], 64)
			return toRet
		}
	}
	iwr.s.Log(fmt.Sprintf("outputlines: %s", string(output)))
	iwr.s.Fatal("Could not parse scan time")
	return 0
}

/*
Determines security from the given list of supported securities.

@param supportedSecurities: slice of supported securitices from scan.

@return SECURITY profile string
*/
func determineSecurity(supportedSecurities []string) string {
	if len(supportedSecurities) == 0 {
		return securityOpen
	} else if len(supportedSecurities) == 1 {
		return supportedSecurities[0]
	} else {
		return securityMixed
	}
}

/*
TimedScan runs a scan on a specified interface and frequencies (if applicable). Returns scan time and BSS list from SSIDs.

@param iface: the interface to run the iw command against
@param frequencies: list of int frequencies in Mhz to scan.
@param ssids: list of string SSIDs to send probe requests for.

@returns TimedScanData struct containing the total scan time and the recovered BssList
*/
func (iwr Runner) TimedScan(iface string, frequencies []int, ssids []string) *TimedScanData {
	var buffer bytes.Buffer
	freqParam := ""
	ssidParam := ""

	var bssList []*BssData
	if len(frequencies) > 0 {
		for _, freq := range frequencies {
			buffer.WriteString(fmt.Sprintf(" freq %d", freq))
		}
		freqParam = buffer.String()
		buffer.Reset()
	}
	if len(ssids) > 0 {
		for _, ssid := range ssids {
			buffer.WriteString(fmt.Sprintf(" ssid %s", string(ssid)))
		}
		ssidParam = buffer.String()
		buffer.Reset()
	}
	iwCommand := fmt.Sprintf("%s dev %s scan%s%s", iwr.iwCommand, iface,
		freqParam, ssidParam)
	command := fmt.Sprintf(iwTimeCommandFormat, iwCommand)
	scanOut, err := iwr.Run(iwr.ctx, command)
	iwr.s.Log(fmt.Sprintf("shellCommand %s", command))
	if status, _ := testexec.GetWaitStatus(err); int(status) != 0 {
		iwr.s.Log(fmt.Sprintf("scan exit status: %d", status))
		return &TimedScanData{0, nil}
	}
	if len(scanOut) < 0 {
		iwr.s.Fatal("Missing scan parse time")
	}
	if strings.HasPrefix(string(scanOut), iwTimeCommandOutputStart) {
		iwr.s.Log("Empty scan result")
		bssList = []*BssData{nil}
	} else {
		bssList = iwr.parseScanResults(scanOut)
	}
	scanTime := iwr.parseScanTime(scanOut)
	return &TimedScanData{scanTime, bssList}
}
