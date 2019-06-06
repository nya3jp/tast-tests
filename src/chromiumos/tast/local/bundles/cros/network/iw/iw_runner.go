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

	"chromiumos/tast/errors"
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
	"above":        ht40Above,
	"below":        ht40Below,
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
	BSS       string
	Frequency int
	SSID      string
	Security  string
	HT        string
	Signal    float64
}

// timedScanData contains the BSS responses from an `iw scan` and its execution time.
type timedScanData struct {
	Time    float64
	BSSList []*bssData
}

// runner stores metadata to allow its methods to invoke commands on `iw` in a concise manner.
type runner struct {
	Run func(context.Context, string) (string, error) // Function alias that will determine how commands are executed whether the test
	// 	is a client test or a remote test (TODO b/972833).
	HostAddr  string          // Host address for remote tests (TODO b/972833).
	iwCommand string          // Path to invoke `iw`. By default, we expect iw to be in $PATH, so this value should be `iw`.
	LogID     int             // ID for logging.
	s         *testing.State  // Test State
	ctx       context.Context // Test Context
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

// extractBSSID parses `link` or `station dump` output and gets the BSSID associated
// with the appropriate interface name.
// If there is no BSSID associated with the wanted interface, extractBSSID will
// return an empty string.
func extractBSSID(linkInformation string, interfaceName string, stationDump bool) string {
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
// clientCommandExec runs in a blocking fashion and will not return until the
// command terminates.
func clientCommandExec(ctx context.Context, shellCommand string) (string, error) {
	out, err := testexec.CommandContext(ctx, "sh", "-c", shellCommand).Output()
	return string(out), err
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

// NewRunner is a factory to create initialize runner struct.
func NewRunner(contxt context.Context, state *testing.State) *runner {
	return &runner{
		Run:       clientCommandExec,
		HostAddr:  "",
		iwCommand: "iw",
		s:         state,
		ctx:       contxt,
	}
}

// newBSSData is a factory method which constructs a bssData from individual
// scan entries.
// bssMatch is the is the BSSID line from the scan.
// dataMatch is the corresponding metadata associated with the BSS entry.
func (iwr runner) newBSSData(bssMatch string, dataMatch string) (*bssData, error) {
	var bss, ssid, sec, ht string
	var freq int
	var sig float64
	supportedSecurities := []string{}

	// Handle BSS.
	bssFields := strings.Fields(bssMatch)
	if len(bssFields) != 2 {
		return nil, errors.New("Unexpected pattern for BSS match")
	}
	bss = bssFields[1]

	// Handle Frequency.
	freqMatch := regexp.MustCompile(`freq:.*`).FindString(dataMatch)
	freq, err := strconv.Atoi(strings.Fields(freqMatch)[1])
	if err != nil {
		return nil, err
	}

	// Handle Signal Strength.
	sigMatch := regexp.MustCompile(`signal:.*`).FindString(dataMatch)
	sig, err = strconv.ParseFloat(strings.Fields(sigMatch)[1], 64)
	if err != nil {
		return nil, err
	}

	// Handle SSID.
	ssidMatch := regexp.MustCompile(`SSID:.*`).FindString(dataMatch)
	if len(ssidMatch) == len("SSID:") || len(ssidMatch) == 0 {
		return nil, errors.New("Could not valid SSID")
	}
	ssid = strings.TrimSpace(ssidMatch[len("SSID:")+1 : len(ssidMatch)])

	// Handle High Throughput.
	htMatch := regexp.MustCompile(
		`\* secondary channel offset.*`).FindString(dataMatch)
	htSplits := strings.Split(htMatch, ":")
	if len(htSplits) != 2 {
		return nil, errors.New("Unexpected pattern for High Throughput")
	}
	ht, ok := htTable[strings.TrimSpace(htSplits[1])]
	if !ok {
		return nil, errors.New(fmt.Sprintf("Invalid HT entry parsed %s",
			strings.TrimSpace(htSplits[1])))
	}

	// Handle Security.
	if supported, _ := regexp.MatchString(`WPA`, dataMatch); supported {
		supportedSecurities = append(supportedSecurities, "WPA")
	}
	if supported, _ := regexp.MatchString(`RSN`, dataMatch); supported {
		supportedSecurities = append(supportedSecurities, "RSN")
	}
	sec = determineSecurity(supportedSecurities)
	return &bssData{bss, freq, ssid, sec, ht, sig}, nil
}

// parseScanResults parses the output of `scan` and `scan dump` commands into
// a slice of bssData structs.
func (iwr runner) parseScanResults(output string) ([]*bssData, error) {
	var bssList = []*bssData{}
	re := regexp.MustCompile(`BSS (\d\d:)*(\d\d)`)
	matches := re.FindAllString(output, -1)
	splits := re.Split(output, -1)
	if len(splits) != len(matches)+1 {
		return nil, errors.New("Unexpected number of matches")
	}
	for i, m := range matches {
		data, err := iwr.newBSSData(m, splits[i+1])
		if err != nil {
			return nil, err
		}
		bssList = append(bssList, data)
	}
	return bssList, nil
}

// parseScanTime parses the total (real) run time of a `time -p "scan"` command.
func (iwr runner) parseScanTime(output string) (float64, error) {
	outputLines := strings.Split(output, "\n")
	for lineNum, line := range outputLines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, iwTimeCommandOutputStart) &&
			lineNum+2 < len(outputLines) &&
			strings.HasPrefix(outputLines[lineNum+1], "user") &&
			strings.HasPrefix(outputLines[lineNum+2], "sys") {
			fields := strings.Fields(line)
			if len(fields) != 2 {
				return 0, errors.New("Unexpected fields size")
			}
			toRet, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				return 0, nil
			}
			return toRet, nil
		}
	}
	iwr.s.Log(fmt.Sprintf("outputlines: %s", output))
	return 0, errors.New("Could not parse scan time")
}

// TimedScan runs a scan on a specified interface and frequencies (if applicable).
func (iwr runner) TimedScan(iface string, frequencies []int, ssids []string) (*timedScanData, error) {
	var buffer bytes.Buffer
	var bssList []*bssData

	for _, freq := range frequencies {
		buffer.WriteString(fmt.Sprintf(" freq %d", freq))
	}
	freqParam := buffer.String()
	buffer.Reset()
	for _, ssid := range ssids {
		buffer.WriteString(fmt.Sprintf(" ssid %s", ssid))
	}
	ssidParam := buffer.String()
	buffer.Reset()

	iwCommand := fmt.Sprintf("%s dev %s scan%s%s", iwr.iwCommand, iface,
		freqParam, ssidParam)
	command := fmt.Sprintf(iwTimeCommandFormat, iwCommand)
	scanOut, err := iwr.Run(iwr.ctx, command)
	if status, _ := testexec.GetWaitStatus(err); int(status) != 0 {

		return nil, errors.New(fmt.Sprintf("Error in scan. scan exit status: %d", status))
	}
	if len(scanOut) <= 0 {
		return nil, errors.New("Missing scan parse time")
	}
	if strings.HasPrefix(scanOut, iwTimeCommandOutputStart) {
		iwr.s.Log("Empty scan result")
		bssList = []*bssData{nil}
	} else {
		bssList, err = iwr.parseScanResults(scanOut)
		if err != nil {
			return nil, err
		}
	}
	scanTime, err := iwr.parseScanTime(scanOut)
	if err != nil {
		return nil, err
	}
	return &timedScanData{scanTime, bssList}, nil
}
