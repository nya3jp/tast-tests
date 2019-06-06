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

// bssDataFactory allows the construction of valid BSS responses from parsed lines.
type bssDataFactory struct {
	data                *bssData
	supportedSecurities []string
}

// timedScanData contains the BSS responses from an `iw scan` and its execution time.
type timedScanData struct {
	Time    float64
	BSSList []*bssData
}

// Runner stores metadata to allow its methods to invoke commands on `iw` in a concise manner.
type Runner struct {
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
	out, err := testexec.CommandContext(ctx, shellCommand).Output()
	return string(out), err
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

// NewBSSDataFactory handles the creation of a bssDataFactory struct.
// It contains the logic to build the struct by parsing line outputs.
// It can also return a pointer to its internal bssData struct if it is
// constructed properly.
func NewBSSDataFactory() bssDataFactory {
	return bssDataFactory{data: &bssData{}, supportedSecurities: []string{}}
}

// parse will update the bssDataFactory's internal bssData struct fields from
// the output of a iw `scan`.
func (fac bssDataFactory) parse(line string) {
	if strings.HasPrefix(line, "freq:") {
		fac.data.Frequency, _ = strconv.Atoi(strings.Split(line, " ")[1])
	}
	if strings.HasPrefix(line, "signal:") {
		fac.data.Signal, _ = strconv.ParseFloat(strings.Split(line, " ")[1], 64)
	}
	if strings.HasPrefix(line, "SSID:") {
		fac.data.SSID = strings.SplitN(line, ": ", 2)[1]
	}
	if strings.HasPrefix(line, "* secondary channel offset") {
		fac.data.HT = htTable[strings.TrimSpace(strings.Split(line, ":")[1])]
	}
	if strings.HasPrefix(line, "WPA") {
		fac.supportedSecurities = append(fac.supportedSecurities, "WPA")
	}
	if strings.HasPrefix(line, "RSN") {
		fac.supportedSecurities = append(fac.supportedSecurities, "WPA2")
	}
}

// create returns the pointer to the bssData struct within the factory as well
// as a flag that indicates the integrity of the struct
// The struct is "ok" if and only if all fields within the bssData struct differ
// from default values.
func (fac bssDataFactory) create() (*bssData, bool) {
	ok := true
	fac.data.Security = determineSecurity(fac.supportedSecurities)
	if fac.data.BSS == "" || fac.data.Frequency == 0 && fac.data.SSID == "" ||
		fac.data.Security == "" || fac.data.HT == "" || fac.data.Signal == 0 {
		ok = false
	}
	return fac.data, ok
}

// parseScanResults parses the output of `scan` and `scan dump` commands into
// a slice of bssData structs.
func (iwr Runner) parseScanResults(output string) []*bssData {
	var bssList = []*bssData{}
	bssDataFact := NewBSSDataFactory()
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		r := regexp.MustCompile("BSS ([0-9a-f:]+)")
		if r.MatchString(line) {
			if bssDataFact.data.BSS != "" {
				data, ok := bssDataFact.create()
				if !ok {
					iwr.s.Fatal("Bad BSS response constructed in parseScanResults")
				}
				bssList = append(bssList, data)
				bssDataFact = NewBSSDataFactory()
			}
			matchGroup := r.FindStringSubmatch(line)
			bssDataFact.data.BSS = matchGroup[1]
		}
		bssDataFact.parse(line)
	}
	data, ok := bssDataFact.create()
	if !ok {
		iwr.s.Fatal("Bad BSS response constructed in parseScanResults")
	}
	bssList = append(bssList, data)
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
	if status, _ := testexec.GetWaitStatus(err); int(status) != 0 {
		iwr.s.Fatal(fmt.Sprintf("Error in scan. scan exit status: %d", status))
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
