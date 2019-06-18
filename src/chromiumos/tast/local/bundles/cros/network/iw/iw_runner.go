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

type devMode string

const (
	devModeAP        devMode = "AP"
	devModeIBSS      devMode = "IBSS"
	devModeMonitor   devMode = "monitor"
	devModeMeshPoint devMode = "mesh point"
	devModeStation   devMode = "managed"
)

var supportedDevModes []devMode = []devMode{devModeAP, devModeIBSS,
	devModeMonitor, devModeMeshPoint, devModeStation}
type Band struct {
	Num int
	Frequencies []int
	FrequencyFlags map[int]string
	McsIndicies []int
}

// bssData contains contents pertaining to a BSS response.
type bssData struct {
	BSS       string
	Frequency int
	SSID      string
	Security  string
	HT        string
	Signal    float64
}

type channelConfig struct {
	number, freq, width, center1Freq int
}

type NetDev struct {
	phy, ifName, ifType string
}

type Phy struct {
	Name, Bands, Modes,Commands,Features,MaxScanSSIDs, AvailTXAntennas, AvailRXAntennas, SupportsSettingAntennaMask, SupportVHT
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

// newBSSData is a factory method which constructs a bssData from individual
// scan entries.
// bssMatch is the is the BSSID line from the scan.
// dataMatch is the corresponding metadata associated with the BSS entry.
func (iwr Runner) newBSSData(bssMatch string, dataMatch string) *bssData {
	var bss, ssid, sec, ht string
	var freq int
	var sig float64
	supportedSecurities := []string{}

	// BSS handling
	bss = strings.Fields(bssMatch)[1]

	// Frequency handling
	freqMatch := regexp.MustCompile(`freq:.*`).FindAllString(dataMatch, -1)[0]
	freq, err := strconv.Atoi(strings.Fields(freqMatch)[1])
	if err != nil {
		iwr.s.Fatal(
			fmt.Sprintf("Failed to convert matched frequency line to int: %s",
				freqMatch))
	}

	// Signal strength handling
	sigMatch := regexp.MustCompile(`signal:.*`).FindAllString(dataMatch, -1)[0]
	sig, err = strconv.ParseFloat(strings.Fields(sigMatch)[1], 64)
	if err != nil {
		iwr.s.Fatal(
			fmt.Sprintf("Failed to convert matched signal line to float64: %s",
				sigMatch))
	}

	// SSID handling
	ssidMatch := regexp.MustCompile(`SSID:.*`).FindAllString(dataMatch, -1)[0]
	if len(ssidMatch) == len("SSID:") {
		iwr.s.Fatal("Empty SSID")
	}
	ssid = strings.TrimSpace(ssidMatch[len("SSID:")+1 : len(ssidMatch)])

	// High Throughput handling
	htMatch := regexp.MustCompile(
		`\* secondary channel offset.*`).FindAllString(dataMatch, -1)[0]
	ht, ok := htTable[strings.TrimSpace(strings.Split(htMatch, ":")[1])]
	if !ok {
		iwr.s.Fatal(fmt.Sprintf("Invalid HT entry parsed %s",
			strings.TrimSpace(strings.Split(htMatch, ":")[1])))
	}

	// Security handling
	if supported, _ := regexp.MatchString(`WPA`, dataMatch); supported {
		supportedSecurities = append(supportedSecurities, "WPA")
	}
	if supported, _ := regexp.MatchString(`RSN`, dataMatch); supported {
		supportedSecurities = append(supportedSecurities, "RSN")
	}
	sec = determineSecurity(supportedSecurities)
	return &bssData{bss, freq, ssid, sec, ht, sig}
}

// parseScanResults parses the output of `scan` and `scan dump` commands into
// a slice of bssData structs.
func (iwr Runner) parseScanResults(output string) []*bssData {
	var bssList = []*bssData{}
	re := regexp.MustCompile(`BSS (\d\d:)*(\d\d)`)
	matches := re.FindAllString(output, -1)
	splits := re.Split(output, -1)
	if len(splits) != len(matches)+1 {
		iwr.s.Fatal("Unexpected number of matches")
	}
	for i, m := range matches {
		bssList = append(bssList, iwr.newBSSData(m, splits[i+1]))
	}
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
			toRet, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				iwr.s.Fatal(fmt.Sprintf("Failed to convert matched signal "+
					"line to float64: %s", fields[1]))
			}
			return toRet
		}
	}
	iwr.s.Log(fmt.Sprintf("outputlines: %s", output))
	iwr.s.Fatal("Could not parse scan time")
	return 0
}

// TimedScan runs a scan on a specified interface and frequencies (if applicable).
func (iwr Runner) TimedScan(iface string, frequencies []int, ssids []string) *timedScanData {
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
		iwr.s.Fatal(fmt.Sprintf("Error in scan. scan exit status: %d", status))
	}
	if len(scanOut) <= 0 {
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

func (iwr Runner) Scan(iface string, frequencies []int, ssids []string) []*bssData {
	return iwr.TimedScan(iface, frequencies, ssids).BSSList
}

func (iwr Runner) addInterface(phy string, iface, string, ifaceType string) {
	_, err := iwr.Run(iwr.ctx, fmt.Sprintf("%s phy %s interface add %s type %s", iwr.iwCommand, phy, iface, ifaceType))
	if err != nil {
		iwr.s.Fatal("addInterface did not terminate properly.")
	}
}

func (iwr Runner) disconnectStation(iface string) {
	_, err := iwr.Run(iwr.ctx, fmt.Sprintf("%s dev %s disconnect", iwr.iwCommand, iface))
	if err != nil {
		iwr.s.Fatal("disconnectStation did not terminate properly.")
	}
}

func (iwr Runner) getLinkValue(iface string, iwLinkKey string) string {
	out, err := iwr.Run(iwr.ctx, fmt.Sprintf("%s dev %s link", iwr.iwCommand, iface))
	if status, _ := testexec.GetWaitStatus(err); int(status) != 0 {
		// There exists a race condition where a mac80211 based driver is
		// 'associated' with an SSID but not the BSS. This causes the iw to
		// return an error code (-2) when attempting to retrieve information
		// specific to the BSS. This does not happe nin the mwifiex drivers.
		return ""
	}
	actualValue := getAllLinkKeys(out)[iwLinkKey]
	return actualValue
}

func (iwr Runner) getOperatingMode(iface string) string {
	out, err := iwr.Run(iwr.ctx, fmt.Sprintf("%s dev %s info", iwr.iwCommand, iface))
	if err != nil {
		iwr.s.Fatal("Could not get Operating Mode.")
	}
	r := regexp.MustCompile(`^\s*type (.*)$`)
	for _, line := range strings.Split(out, "\n") {
		if r.MatchString(line) {
			matchGroup := r.FindStringSubmatch(line)
			operatingMode := matchGroup[1]
			for _, v := range supportedDevModes {
				if v == devMode(operatingMode) {
					return operatingMode
				}
			}
			iwr.s.Fatal(fmt.Sprintf("Unsupported operating mode %s found for"+
				" interface: %s.", operatingMode, iface))
		}
	}
	return ""
}

func (iwr Runner) getRadioConfig(iface string) channelConfig {
	out, err := iwr.Run(iwr.ctx, fmt.Sprintf("%s dev %s info", iwr.iwCommand, iface))
	if err != nil {
		iwr.s.Fatal("Could not get Radio Config.")
	}
	r := regexp.MustCompile(`^\s*channel ([0-9]+) \(([0-9]+) MHz\), width: ([2,4,8]0) MHz, center1: ([0-9]+) MHz`)
	for _, line := range strings.Split(out, "\n") {
		if r.MatchString(line) {
			matchGroup := r.FindStringSubmatch(line)
			var number, freq, width, center1Freq int
			if val, err := strconv.Atoi(matchGroup[1]); err != nil {
				number = val
				iwr.s.Fatal("Could not parse number.")
			}
			if val, err := strconv.Atoi(matchGroup[2]); err != nil {
				freq = val
				iwr.s.Fatal("Could not parse freq.")
			}
			if val, err := strconv.Atoi(matchGroup[3]); err != nil {
				width = val
				iwr.s.Fatal("Could not parse width.")
			}
			if val, err := strconv.Atoi(matchGroup[4]); err != nil {
				center1Freq = val
				iwr.s.Fatal("Could not parse center1Freq.")
			}
			return channelConfig{number, freq, width, center1Freq}
		}
	}
	iwr.s.Fatal("Could not find radio config.")
	return channelConfig{}
}

func (iwr Runner) ibssJoin(iface string, ssid string, frequency int) {
	iwr.Run(iwr.ctx, fmt.Sprintf("%s dev %s ibss join %s %d", iwr.iwCommand, iface, ssid, frequency))
}

func (iwr Runner) ibssLeave(iface string) {
	iwr.Run(iwr.ctx, fmt.Sprintf("%s dev %s ibss leave", iwr.iwCommand, iface))
}

func (iwr Runner) removeInterface(iface string) {
	iwr.Run(iwr.ctx, fmt.Sprintf("%s dev %s del", iwr.iwCommand, iface))
}

func (iwr Runner) scanDump(iface string) []*bssData {
	out, err := iwr.Run(iwr.ctx, fmt.Sprintf("%s dev %s scan dump", iwr.iwCommand, iface))
	if err != nil {
		status, _ := testexec.GetWaitStatus(err)
		iwr.s.Fatal(fmt.Sprintf("Scan dump failed with error code %d.", int(status)))
	}
	return iwr.parseScanResults(out)
}

func (iwr Runner) setTxPower(iface string, power string) {
	iwr.Run(iwr.ctx, fmt.Sprintf("%s dev %s set txpower %s", iwr.iwCommand, iface, power))
}

func (iwr Runner) setFreq(iface string, freq int) {
	iwr.Run(iwr.ctx, fmt.Sprintf("%s dev %s set f req %d", iwr.iwCommand, iface, freq))
}

func (iwr Runner) setRegulatoryDomain(domainString string) {
	iwr.Run(iwr.ctx, fmt.Sprintf("%s reg set %s", iwr.iwCommand, domainString))
}

func (iwr Runner) getRegulatoryDomain() string {
	out, err := iwr.Run(iwr.ctx, fmt.Sprintf("%s reg get", iwr.iwCommand))
	if err != nil {
		status, _ := testexec.GetWaitStatus(err)
		iwr.s.Fatal(fmt.Sprintf("getRegulatoryDomain failed with error code %d.", int(status)))
	}
	r := regexp.MustCompile(`^country (..):`)
	for _, line := range strings.Split(out, "\n") {
		if r.MatchString(line) {
			matchGroup := r.FindStringSubmatch(line)
			return matchGroup[1]
		}
	}
	iwr.s.Fatal("Could not find Regulatory Domain.")
	return ""
}

func (iwr Runner) setAntennaBitmap(phy string, txBitmap int, rxBitmap int) {
	iwr.Run(iwr.ctx, fmt.Sprintf("%s phy %s set antenna %d %d", iwr.iwCommand, phy, txBitmap, rxBitmap))
}

func (iwr Runner) vhtSupported() bool {
	out, err := iwr.Run(iwr.ctx, fmt.Sprintf("%s list", iwr.iwCommand))
	if err != nil {
		iwr.s.Fatal("Could not successfully check if VHT supported.")
	}
	return strings.Contains(out, "VHT Capabilities")
}

func (iwr Runner) getFragmentationThreshold(phy string) int {
	out, err := iwr.Run(iwr.ctx, fmt.Sprintf("%s phy %s info", iwr.iwCommand, phy))
	if err != nil {
		status, _ := testexec.GetWaitStatus(err)
		iwr.s.Fatal(fmt.Sprintf("getFragmentationThreshold failed with error code %d.", int(status)))
	}
	r := regexp.MustCompile(`^\s+Fragmentation threshold:\s+([0-9]+)$`)
	for _, line := range strings.Split(out, "\n") {
		if r.MatchString(line) {
			matchGroup := r.FindStringSubmatch(line)
			if len(matchGroup) != 2 {
				iwr.s.Fatal("Unexpected input when parsing thresh.")
			}
			thresh, err := strconv.Atoi(matchGroup[1])
			if err != nil {
				iwr.s.Fatal("Could not parse threshold.")
			}
			return thresh
		}
	}
	iwr.s.Fatal("Could not find threshold.")
	return 0
}

func (iwr Runner) NewNetDev(phyMatch string, dataMatch string) *NetDev {
	var phy, ifName, ifType string
	// PHY handling
	phyMatch := regexp.MustCompile(`phy#([0-9]+)`)
	matchGroup := phyMatch.FindStringSubmatch(phyMatch)
	if len(matchGroup) != 2 {
		iwr.s.Fatal("Unexpected input when parsing phy.")
	}
	phy := matchGroup[1]

	// ifName handling
	ifNameMatch := regexp.MustCompile(`[\s]*Interface (.*)`)
	matchGroup = ifNameMatch.FindStringSubmatch(dataMatch)
	if len(matchGroup) != 2 {
		iwr.s.Fatal("Unexpected input when parsing ifName.")
	}
	ifName = matchGroup[1]

	// ifType handling
	ifTypeMatch := regexp.MustCompile(`[\s]*type ([a-zA-Z]+)`)
	matchGroup = ifTypeMatch.FindStringSubmatch(dataMatch)
	if len(matchGroup) != 2 {
		iwr.s.Fatal("Unexpected input when parsing ifType.")
	}
	ifType = matchGroup[1]
	return &NetDev{phy, ifName, ifType}
}

func (iwr Runner) listInterfaces() []*NetDev {
	out, err := iwr.Run(iwr.ctx, fmt.Sprintf("%s dev", iwr.iwCommand))
	if err != nil {
		status, _ := testexec.GetWaitStatus(err)
		iwr.s.Fatal(fmt.Sprintf("listInterfaces failed with error code %d.", int(status)))
	}
	interfaces := []*NetDev{}
	r := regexp.MustCompile(`phy#([0-9]+)`)
	matches := r.FindAllString(out, -1)
	splits := r.Split(out, -1)
	if len(splits) != len(matches)+1 {
		iwr.s.Fatal("Unexpected number of matches")
	}
	for i, m := range matches {
		interfaces = append(interfaces, iwr.NewNetDev(m, splits[i+1]))
	}
	return interfaces
}

func (iwr Runner) getInterface(iface string) *NetDev {
	matchingInterfaces := []*NetDev
	for _, val := range iwr.listInterfaces() {
		if val.ifName == iface {
			matchingInterfaces = append(matchingInterfaces, val)
		}
	}
	if len(matchingInterfaces) != 1 {
		iwr.s.Fatal(fmt.Sprintf("Could not find interface named %s", iface))
	}
	return matchingInterfaces[0]
}

func (iwr Runner) frequencySupported(frequency int) {
	phys := iwr.listPhys()
	for _, phy := range phys {
		for _, band := range bands {
			for _, bandFreq := range band.frequencies {
				if frequency == bandFreq {
					return true
				}
			}
		}
	}
	return false
}
func (iwr Runner) NewPhy(phyMatch string, dataMatch string) *Phy {
	var  name string, bands, modes, commands, features, maxScanSSIDs, availTXAntennas, availRXAntennas, supportsSettingAntennaMask, supportVHT
	currentSection := ""
	currentBand :=  Band{}
	phyModes := []string
	phyCommands := []string
	phyFeatures := []string
	rxAntennas := []int
	txAntennas := []int
	mcsIndices := []int
	// PHY handling
	nameMatch := regexp.MustCompile(`Wiphy (.*)`)
	matchGroup := phyMatch.FindStringSubmatch(phyMatch)
	if len(matchGroup) != 2 {
		iwr.s.Fatal("Unexpected input when parsing name.")
	}
	name := matchGroup[1]

	// Current Section handling
	sectionMatch := regexp.MustCompile(`\s*(\w.*):\s*$`)
	matchGroup = sectionMatch.FindStringSubmatch(dataMatch)
	if len(matchGroup) != 2 {
		iwr.s.Fatal("Could not find section when parsing data")
	}
	currentSection = matchGroup[1]
		
	// Band handling
	bandMatch := regexp.MustCompile(`Band (\d+)`)
	matchGroup = bandMatch.FindStringSubmatch(currentSection)
	if len(matchGroup) != 2 {
		iwr.s.Fatal("Unexpected input when parsing band.")
	}
	num, err := strconv.Atoi(matchGroup[1])
	if err != nil {
		iwr.s.Fatal("Could not convert num to string")
	currentBand = Band{num, []int{}, make(map[int]string), []int{}}
	
	// Max Scan SSIDs handling
	maxScanSSIDsMatch :=  regexp.MustCompile(`\s*max # scan SSIDs: (\d+)`)
	matchGroup = maxScanSSIDsMatch.FindStringSubmatch(dataMatch)
	if len(matchGroup) != 2 {
		iwr.s.Fatal("Could not find SSID when parsing data.")
	}
	maxScanSSIDs = matchGroup[1]

	// Phy modes handling
	if currentSection == "Supported interface modes" && name != "" {
		modeMatch := regexp.MustCompile(`\* (\w+)`)
		matchGroups = modeMatch.FindAllStringSubmatch(dataMatch, -1)
		for _, matchGroup : range matchGroups {
			if len(matchGroup) != 2 {
				iwr.s.Fatal("Unexpected input when parsing phy modes.")
			}
			phyModes = append(phyModes, matchGroup[1])
		}
	}
	// Phy commands handling
	if currentSection == "Supported commands" && name != "" {
		commandsMatch := regexp.MustCompile(`\* (\w+)`)
		matchGroups = commandsMatch.FindAllStringSubmatch(dataMatch, -1)
		for _, matchGroup : range matchGroups {
			if len(matchGroup) != 2 {
				iwr.s.Fatal("Unexpected input when parsing phy commands.")
			}
			phyCommands = append(phyCommands, matchGroup[1])
		}

	}

	// VHT Support handling
	if currentSection != "" && strings.HasPrefix(currentSection, "VHT Capabilities") {
		supportVHT = true
	}

	// Antennae handling	
	availAntennaMatch := regexp.MustCompile(`\s*Available Antennas: TX (\S+) RX (\S+)`)
	matchGroups = availAntennaMatch.FindAllStringSubmatch(dataMatch, -1)
	for _, matchGroup : range matchGroups {
		if len(matchGroup) != 3 {
			iwr.s.Fatal("Unexpected input when parsing antennas.")
		}
		txAntenna, err := strconv.Atoi(matchGroup[1])
		if err != nil {
			iwr.s.Fatal("Could not convert txAntenna to int.")
		}
		rxAntenna, err := strconv.Atoi(matchGroup[2])
		if err != nil {
			iwr.s.Fatal("Could not convert rxAntenna to int.")
		}
		txAntennas = append(txAntennas, txAntenna)
		rxAntennas = append(rxAntennas, rxAntenna)
	}

	// Device Support handling
	deviceSupportMatch := regexp.MustCompile(`\s*Device supports (.*)\.`)
	matchGroups = deviceSupportMatch.FindAllStringSubmatch(dataMatch, -1)
	for _, matchGroup : range matchGroups {
		if len(matchGroup) != 2 {
			iwr.s.Fatal("Unexpected input when parsing device support.")
		}
		deviceSupport = append(deviceSupport, matchGroup[1])
	}

	// Channel Info handling
	chanInfoMatch := regexp.MustCompile(`?P<frequency>\d+) MHz (?P<chan_num>\[\d+\])(?: \((?P<tx_power_limit>[0-9.]+ dBm)\))?(?: \((?P<flags>[a-zA-Z, ]+)\))?`)
	matchGroup = chanInfoMatch. //TODO


	// Rate handling
	rateMatch := regexp.MustCompile(`HT TX/RX MCS rate indexes supported: .*`)
	matchGroups = rateMatch.FindAllStringSubmatch(dataMatch, -1)
	for _, matchGroup : range matchGroups {
		if len(matchGroup) != 2 {
			iwr.s.Fatal("Unexpected input when parsing rate.")
		}
		rateStr := strings.TrimSpace(strings.Split(matchGroup[1],":")[1])
		for _, piece := strings.Split(rateStr,",") {
			if strings.Contains(piece,"-") {
				res := strings.split("-")	
				if len(res) != 2 {
					iwr.s.Fatal("Unexpected number of dashes in token.")
				}
				begin, _:= strconv.Atoi(res[0])
				end, _:= strconv.Atoi(res[1])
				for i:= begin; i < end +1; i++ {
					mcsIndices = append(mcsIndices, i)
				}

			} else {
				val, _ = strconv.Atoi(piece)
				mcsIndices = append(mcsIndices,val)
			}
		}
	}
}

func (iwr Runner) listPhys() []*Phy {
	out, err := iwr.Run(iwr.ctx, fmt.Sprintf("%s list", iwr.iwCommand))
	if err != nil {
		status, _ := testexec.GetWaitStatus(err)
		iwr.s.Fatal(fmt.Sprintf("listPhys failed with error code %d.", int(status)))
	}
	phys := []*Phy{}
	r := regexp.MustCompile(`Wiphy (.*)`)
	matches := r.FindAllString(out, -1)
	splits := r.Split(out, -1)
	if len(splits) != len(matches)+1 {
		iwr.s.Fatal("Unexpected number of matches")
	}
	for i, m := range matches {
		phys = append(phys, iwr.NewNetDev(m, splits[i+1]))
	}
	return phys
}
//func getStationDump(iface string) ??{
//TODO
//}

