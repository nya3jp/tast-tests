// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package iw contains utility functions to wrap around the iw program.
package iw

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

const (
	iwTimeCommandFormat      = "(time -p %s) 2>&1"
	iwTimeCommandOutputStart = "real"
)

const (
	ht20      = "HT20"
	ht40Above = "HT40+"
	ht40Below = "HT40-"
)

var htTable = map[string]string{
	"no secondary": ht20,
	"above":        ht40Above, "below": ht40Below,
}

const (
	securityOpen  = "open"
	securityWep   = "wep"
	securityWpa   = "wpa"
	securityWpa2  = "wpa2"
	securityMixed = "mixed"
)

// bssData contains contents pertaining to a BSS response.
type bssData struct {
	bss       string
	frequency int
	ssid      string
	security  string
	ht        string
	signal    float64
}

// timedScanData contains the BSS responses from an `iw scan` and its execution time.
type timedScanData struct {
	time    float64
	bssList []*bssData
}

// getAllLinkKeys parses `link` or `station dump` output into key value pairs.
func getAllLinkKeys(linkInformation string) map[string]string {
	linkKeyValuePairs := make(map[string]string)
	r := regexp.MustCompile(`^\s+(.*):\s+(.*)$`)
	for _, linkKey := range strings.Split(linkInformation, "\n") {
		if r.MatchString(linkKey) {
			matchGroup := r.FindStringSubmatch(linkKey)
			linkKeyValuePairs[matchGroup[1]] = matchGroup[2]
		}
	}
	return linkKeyValuePairs
}

// extractBssid parses `link` or `station dump` output and gets the BSSID associated
// with the appropriate interface name.
// If there is no BSSID associated with the wanted interface, extractBssid will
// return an empty string.
func extractBssid(linkInformation string, interfaceName string, stationDump bool) string {
	identifier := "Connected to"
	if stationDump {
		identifier = "Station"
	}
	searchRe := regexp.MustCompile(fmt.Sprintf(`%s ([0-9a-fA-F:]{17}) \(on %s\)`,
		identifier, interfaceName))
	matchGroup := searchRe.FindStringSubmatch(linkInformation)
	if len(matchGroup) == 0 {
		return ""
	}
	return matchGroup[1]
}

// clientCommandExec executes a shell command on the DUT and returns its output.
// clientCommandExec runs ina blocking fashion and will not return until the
// command terminates.
func clientCommandExec(ctx context.Context, shellCommand string) (string, error) {
	out, err := testexec.CommandContext(ctx, shellCommand).Output()
	return string(out), err
}

// Runner stores metadata to allow its methods to invoke commands on `iw` in a concise manner.
type Runner struct {
	Run func(context.Context, string) (string, error) // Function alias that will determine how commands are executed whether the test
	// 	is a client test or a remote test (TODO b/972833).
	HostAddr  string          // Host address for remote tests (TODO b/972833).
	iwCommand string          // Path to invoke `iw`. By default, we expect iw to be in $PATH, so this value should be `iw`.
	LogID     int             // Id for logging.
	s         *testing.State  // Test State
	ctx       context.Context // Test Context
}

// NewRunner is a factory to create initialize Runner struct.
func NewRunner(contxt context.Context, state *testing.State) *Runner {
	return &Runner{
		Run:       clientCommandExec,
		HostAddr:  "",
		iwCommand: "iw",
		s:         state,
		ctx:       contxt,
	}
}

// parseScanResults parses the output of `scan` and `scan dump` commands into
// a slice of bssData structs.
func (iwr Runner) parseScanResults(output string) []*bssData {
	var bssList = []*bssData{}
	mainBss := bssData{}
	supportedSecurities := []string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		r := regexp.MustCompile("BSS ([0-9a-f:]+)")
		if r.MatchString(line) {
			if mainBss.bss != "" {
				mainBss.security = determineSecurity(supportedSecurities)
				bssList = append(bssList, &bssData{mainBss.bss, mainBss.frequency,
					mainBss.ssid, mainBss.security, mainBss.ht, mainBss.signal})
				mainBss = bssData{}
				supportedSecurities = nil
			}
			matchGroup := r.FindStringSubmatch(line)
			mainBss.bss = matchGroup[1]
		}
		if strings.HasPrefix(line, "freq:") {
			mainBss.frequency, _ = strconv.Atoi(strings.Split(line, " ")[1])
		}
		if strings.HasPrefix(line, "signal:") {
			mainBss.signal, _ = strconv.ParseFloat(strings.Split(line, " ")[1], 64)
		}
		if strings.HasPrefix(line, "SSID:") {
			mainBss.ssid = strings.SplitN(line, ": ", 2)[1]
		}
		if strings.HasPrefix(line, "* secondary channel offset") {
			mainBss.ht = htTable[strings.TrimSpace(strings.Split(line, ":")[1])]
		}
		if strings.HasPrefix(line, "WPA") {
			supportedSecurities = append(supportedSecurities, "WPA")
		}
		if strings.HasPrefix(line, "RSN") {
			supportedSecurities = append(supportedSecurities, "WPA2")
		}
	}
	mainBss.security = determineSecurity(supportedSecurities)
	bssList = append(bssList, &mainBss)
	return bssList
}

// parseScanTime parses the total (real) run time of a `time -p "scan"` command.
func (iwr Runner) parseScanTime(output string) float64 {
	outputLines := strings.Split(output, "\n")
	for lineNum, line := range outputLines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, iwTimeCommandOutputStart) &&
			lineNum+2 < len(outputLines) &&
			strings.HasPrefix(outputLines[lineNum+1], "user") &&
			strings.HasPrefix(outputLines[lineNum+2], "sys") {
			fields := strings.Fields(line)
			if len(fields) != 2 {
				iwr.s.Fatal("Unexpected fields size.")
			}
			toRet, _ := strconv.ParseFloat(fields[1], 64)
			return toRet
		}
	}
	iwr.s.Log(fmt.Sprintf("outputlines: %s", output))
	iwr.s.Fatal("Could not parse scan time")
	return 0
}

// determineSecurity determines the security level of a connection based on the
// number of supported securities.
func determineSecurity(supportedSecurities []string) string {
	if len(supportedSecurities) == 0 {
		return securityOpen
	} else if len(supportedSecurities) == 1 {
		return supportedSecurities[0]
	} else {
		return securityMixed
	}
}

// TimedScan runs a scan on a specified interface and frequencies (if applicable).
func (iwr Runner) TimedScan(iface string, frequencies []int, ssids []string) *timedScanData {
	var buffer bytes.Buffer
	freqParam := ""
	ssidParam := ""

	var bssList []*bssData
	if len(frequencies) > 0 {
		for _, freq := range frequencies {
			buffer.WriteString(fmt.Sprintf(" freq %d", freq))
		}
		freqParam = buffer.String()
		buffer.Reset()
	}
	if len(ssids) > 0 {
		for _, ssid := range ssids {
			buffer.WriteString(fmt.Sprintf(" ssid %s", ssid))
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
		return &timedScanData{0, nil}
	}
	if len(scanOut) < 0 {
		iwr.s.Fatal("Missing scan parse time")
	}
	if strings.HasPrefix(scanOut, iwTimeCommandOutputStart) {
		iwr.s.Log("Empty scan result")
		bssList = []*bssData{nil}
	} else {
		bssList = iwr.parseScanResults(scanOut)
	}
	scanTime := iwr.parseScanTime(scanOut)
	return &timedScanData{scanTime, bssList}
}
