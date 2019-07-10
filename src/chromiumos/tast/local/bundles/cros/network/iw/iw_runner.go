// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package iw contains utility functions to wrap around the iw program.
package iw

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

var htTable = map[string]string{
	"no secondary": "HT20",
	"above":        "HT40+",
	"below":        "HT40-",
}

const (
	securityOpen  = "open"
	securityWEP   = "wep"
	securityWPA   = "wpa"
	securityWPA2  = "wpa2"
	securityMixed = "mixed"
)

// Band contains supported wireless band attributes.
type Band struct {
	Num            int
	FrequencyFlags map[int][]string
	McsIndices     []int
}

// BSSData contains contents pertaining to a BSS response.
type BSSData struct {
	BSS       string
	Frequency int
	SSID      string
	Security  string
	HT        string
	Signal    float64
}

// NetDev contains interface attributes from `iw dev`.
type NetDev struct {
	PhyNum         int
	IfName, IfType string
}

// Phy contains phy# attributes.
type Phy struct {
	Name                                      string
	Bands                                     []Band
	Modes, Commands, Features                 []string
	RxAntenna, TxAntenna                      int
	MaxScanSSIDs                              int
	SupportVHT, SupportHT2040, SupportHT40SGI bool
}

// ChannelConfig contains the configuration data for a radio config.
type ChannelConfig struct {
	Number, Freq, Width, Center1Freq int
}

type sectionAttributes struct {
	bands                                     []Band
	phyModes, phyCommands                     []string
	supportVHT, supportHT2040, supportHT40SGI bool
}

// TimedScanData contains the BSS responses from an `iw scan` and its execution time.
type TimedScanData struct {
	Time    time.Duration
	BSSList []*BSSData
}

// GetInterfaceAttributes gets a single interface's attributes.
func GetInterfaceAttributes(ctx context.Context, iface string) (*NetDev, error) {
	var matchIfs []*NetDev
	ifs, err := ListInterfaces(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "ListInterfaces failed")
	}
	for _, val := range ifs {
		if val.IfName == iface {
			matchIfs = append(matchIfs, val)
		}
	}
	if len(matchIfs) == 0 {
		return nil, errors.Errorf("could not find interface named %s", iface)
	}
	return matchIfs[0], nil
}

// ListInterfaces yields all the attributes (NetDev) for each interface.
func ListInterfaces(ctx context.Context) ([]*NetDev, error) {
	out, err := testexec.CommandContext(ctx, "iw", "dev").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "ListInterfaces failed")
	}
	var interfaces []*NetDev

	matches, splits, err := splitTextOnRegexMatches(`phy#([0-9]+)`, string(out))
	if err != nil {
		return nil, errors.Wrap(err, "could not parse netDev")
	}
	for i, phy := range matches {
		ifaces, err := extractMatch(`\s*Interface (.*)`, splits[i])
		if err != nil {
			return nil, errors.Wrap(err, "could not parse interface")
		}
		for i, iface := range ifaces {
			netdev, err := newNetDev(phy, iface, splits[i])
			if err != nil {
				return nil, errors.Wrap(err, "could not extract interface attributes")
			}
			interfaces = append(interfaces, netdev)
		}
	}
	return interfaces, nil
}

// ListPhys will return a Phy struct for each phy on the DUT.
func ListPhys(ctx context.Context) ([]*Phy, error) {
	out, err := testexec.CommandContext(ctx, "iw", "list").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "iw list failed")
	}

	matches, splits, err := splitTextOnRegexMatches(`Wiphy (.*)`, string(out))
	if err != nil {
		return nil, errors.Wrap(err, "could not parse phys")
	}
	var phys []*Phy
	for i, m := range matches {
		phy, err := newPhy(m, splits[i])
		if err != nil {
			return nil, errors.Wrap(err, "could not extract phy attributes")
		}
		phys = append(phys, phy)
	}
	return phys, nil
}

// TimedScan runs a scan on a specified interface and frequencies (if applicable).
// A channel map for valid frequencies can be found in
// third_party/autotest/files/server/cros/network/hostap_config.py
// The frequency slice is used to whitelist which frequencies/bands to scan on.
// The SSIDs slice will filter the results of the scan to those that pertain
// to the whitelisted SSIDs (although this doesn't seem to work on some devices).
func TimedScan(ctx context.Context, iface string,
	frequencies []int, ssids []string) (*TimedScanData, error) {
	args := []string{"dev", iface, "scan"}
	for _, freq := range frequencies {
		args = append(args, "freq", strconv.Itoa(freq))
	}
	for _, ssid := range ssids {
		args = append(args, "ssid", ssid)
	}
	startTime := time.Now()
	out, err := testexec.CommandContext(ctx, "iw", args...).Output()
	scanTime := time.Since(startTime)
	if err != nil {
		return nil, errors.Wrap(err, "iw scan failed")
	}
	scanOut := string(out)
	bssList, err := parseScanResults(scanOut)
	if err != nil {
		return nil, err
	}
	return &TimedScanData{scanTime, bssList}, nil
}

// ScanDump returns a list of BSSData from a scan dump.
func ScanDump(ctx context.Context, iface string) ([]*BSSData, error) {
	out, err := testexec.CommandContext(ctx, "iw", "dev", iface, "scan",
		"dump").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "scan dump failed")
	}
	return parseScanResults(string(out))
}

// GetLinkValue gets the specified link value from the iw link output.
func GetLinkValue(ctx context.Context, iface string, iwLinkKey string) (string, error) {
	res, err := testexec.CommandContext(ctx, "iw", "dev", iface, "link").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get link information from interface %s", iface)
	}
	kvs := getAllLinkKeys(string(res))
	out := kvs[iwLinkKey]
	if out == "" {
		return "", errors.Errorf("could not extract link value from link information with link key %s: %v", iwLinkKey, kvs)
	}
	return out, nil
}

// GetOperatingMode gets the interface's operating mode.
func GetOperatingMode(ctx context.Context, iface string) (string, error) {
	out, err := testexec.CommandContext(ctx, "iw", "dev", iface,
		"info").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to get interface information")
	}
	supportedDevModes := []string{"AP", "monitor", "managed"}
	m, err := extractMatch(`(?m)^\s*type (.*)$`, string(out))
	if err != nil {
		return "", errors.Wrap(err, "failed to parse operating mode")
	}
	opMode := m[0]
	for _, v := range supportedDevModes {
		if v == opMode {
			return opMode, nil
		}
	}
	return "", errors.Wrapf(err, "unsupported operating mode %s found for interface: %s", opMode, iface)
}

// GetRadioConfig gets the radio configuration from the interface's information.
func GetRadioConfig(ctx context.Context, iface string) (*ChannelConfig, error) {
	out, err := testexec.CommandContext(ctx, "iw", "dev", iface, "info").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get interface information")
	}
	m, err := extractMatch(`(?m)^\s*channel (\d+) \((\d+) MHz\), width: (\d{2}) MHz, center1: (\d+) MHz`, string(out))
	if err != nil {
		return nil, errors.Wrap(err, "failed to pase radio config")
	}
	number, err := strconv.Atoi(m[0])
	if err != nil {
		return nil, errors.New("could not parse number")
	}
	freq, err := strconv.Atoi(m[1])
	if err != nil {
		return nil, errors.New("could not parse freq")
	}
	width, err := strconv.Atoi(m[2])
	if err != nil {
		return nil, errors.New("could not parse width")
	}
	center1Freq, err := strconv.Atoi(m[3])
	if err != nil {
		return nil, errors.New("could not parse center1Freq")
	}
	return &ChannelConfig{
		Number:      number,
		Freq:        freq,
		Width:       width,
		Center1Freq: center1Freq,
	}, nil
}

// GetRegulatoryDomain gets the regulatory domain code.
func GetRegulatoryDomain(ctx context.Context) (string, error) {
	out, err := testexec.CommandContext(ctx, "iw", "reg", "get").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to get regulatory domain")
	}
	r := regexp.MustCompile(`(?m)^country (..):`)
	if m := r.FindStringSubmatch(string(out)); m != nil {
		return m[1], nil
	}
	return "", errors.New("could not find regulatory domain")
}

// SetTxPower sets the wireless interface's transmit power.
func SetTxPower(ctx context.Context, iface string, mode string, power int) error {
	if err := testexec.CommandContext(ctx, "iw", "dev", iface, "set",
		"txpower", mode, strconv.Itoa(power)).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to set txpower")
	}
	return nil
}

// SetFreq sets the wireless interface's LO center freq.
// Interface should be in monitor mode before scanning.
func SetFreq(ctx context.Context, iface string, freq int) error {
	if err := testexec.CommandContext(ctx, "iw", "dev", iface, "set",
		"freq", strconv.Itoa(freq)).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to set freq")
	}
	return nil
}

// SetAntennaBitmap sets the antenna bitmap.
func SetAntennaBitmap(ctx context.Context, phy string, txBitmap int, rxBitmap int) error {
	if err := testexec.CommandContext(ctx, "iw", "phy", phy, "set",
		"antenna", strconv.Itoa(txBitmap), strconv.Itoa(rxBitmap)).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to set Antenna bitmap")
	}
	return nil
}

// determineSecurity determines the security level of a connection based on the
// number of supported securities.
func determineSecurity(secs []string) string {
	if len(secs) == 0 {
		return securityOpen
	} else if len(secs) == 1 {
		return secs[0]
	} else {
		return securityMixed
	}
}

// getAllLinkKeys parses `link` or `station dump` output into key value pairs.
func getAllLinkKeys(out string) map[string]string {
	kv := make(map[string]string)
	r := regexp.MustCompile(`^\s+(.*):\s+(.*)$`)
	for _, line := range strings.Split(out, "\n") {
		m := r.FindStringSubmatch(line)
		if m != nil {
			kv[m[1]] = m[2]
		}
	}
	return kv
}

// newBSSData is a factory method which constructs a BSSData from individual
// scan entries.
// bssMatch is the is the BSSID line from the scan.
// dataMatch is the corresponding metadata associated with the BSS entry.
func newBSSData(bssMatch string, dataMatch string) (*BSSData, error) {
	// Handle BSS.
	bssFields := strings.Fields(bssMatch)
	if len(bssFields) != 2 {
		return nil, errors.New("unexpected pattern for BSS match")
	}
	bss := bssFields[1]

	// Handle Frequency.
	m := regexp.MustCompile(`freq: (\d+)`).FindStringSubmatch(dataMatch)
	if m == nil {
		return nil, errors.New("freq field not found")
	}
	freq, err := strconv.Atoi(m[1])
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse freq field")
	}

	// Handle Signal Strength.
	sigMatch := regexp.MustCompile(`signal:.*`).FindString(dataMatch)
	sig, err := strconv.ParseFloat(strings.Fields(sigMatch)[1], 64)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse signal strength field")
	}

	// Handle SSID.
	ssidMatch := regexp.MustCompile(`SSID:.*`).FindString(dataMatch)
	//TODO(crbug.com/992083): elegantly handle hidden SSIDs
	if len(ssidMatch) == len("SSID:") || len(ssidMatch) == 0 {
		return nil, errors.New("could not valid SSID")
	}
	ssid := strings.TrimSpace(ssidMatch[len("SSID:")+1 : len(ssidMatch)])

	// Handle high throughput setting.
	htMatch := regexp.MustCompile(
		`\* secondary channel offset.*`).FindString(dataMatch)
	htSplits := strings.Split(htMatch, ":")
	var ht string
	if len(htSplits) == 2 {
		htTemp, ok := htTable[strings.TrimSpace(htSplits[1])]
		if !ok {
			return nil, errors.Errorf("invalid HT entry parsed %s",
				strings.TrimSpace(htSplits[1]))
		}
		ht = htTemp
	} else {
		// Default high throughput value if the section is not advertised.
		ht = htTable["no secondary"]
	}

	// Handle Security.
	var secs []string
	if strings.Contains(dataMatch, "WPA") {
		secs = append(secs, "WPA")
	}
	if strings.Contains(dataMatch, "RSN") {
		secs = append(secs, "RSN")
	}
	sec := determineSecurity(secs)
	return &BSSData{
		BSS:       bss,
		Frequency: freq,
		SSID:      ssid,
		Security:  sec,
		HT:        ht,
		Signal:    sig}, nil
}

// newNetDev is an internal factory method that constructs a NetDev struct
// from each phy listed in `iw dev`.
func newNetDev(phystr, ifName, dataMatch string) (*NetDev, error) {
	// Parse phy number.
	m, err := extractMatch(`phy#([0-9]+)`, phystr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse phy number")
	}
	phy, err := strconv.Atoi(m[0])
	if err != nil {
		return nil, errors.Wrapf(err, "could not convert str %q to int", m[0])
	}

	// Parse ifType

	m, err = extractMatch(`\s*type ([a-zA-Z]+)`, dataMatch)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse ifType")
	}

	ifType := m[0]
	return &NetDev{PhyNum: phy, IfName: ifName, IfType: ifType}, nil
}

// newPhy is a factory method that constructs a Phy struct from `iw list` output.
func newPhy(phyMatch string, dataMatch string) (*Phy, error) {
	// Phy name handling.
	m, err := extractMatch(`Wiphy (.*)`, phyMatch)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse name")
	}
	name := m[0]

	// Antennae handling.
	hexToInt := func(str string) (int, error) {
		res, err := strconv.ParseInt(strings.TrimPrefix(str, "0x"), 16, 64)
		if err != nil {
			return 0, errors.Wrap(err, "could not parse hex string")
		}
		return int(res), nil
	}
	m, err = extractMatch(`\s*Available Antennas: TX (\S+) RX (\S+)`, dataMatch)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse antennas")
	}

	txAntenna, err := hexToInt(m[0])
	if err != nil {
		return nil, err
	}
	rxAntenna, err := hexToInt(m[1])
	if err != nil {
		return nil, err
	}

	// Device Support handling.
	var phyFeatures []string
	matches := regexp.MustCompile(`\s*Device supports (.*)\.`).FindAllStringSubmatch(dataMatch, -1)
	for _, m := range matches {
		phyFeatures = append(phyFeatures, m[1])
	}

	// Max Scan SSIDs handling.
	m, err = extractMatch(`\s*max # scan SSIDs: (\d+)`, dataMatch)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse max scan SSIDs")
	}
	maxScanSSIDs, err := strconv.Atoi(m[0])
	if err != nil {
		return nil, errors.Wrapf(err, "could not convert maxScanSSIDs %s to string", m[0])
	}

	// Handle parsing attributes that need to be handled on a section by section level.
	// Sections are defined as blocks of text that are delimited by level 1 indent lines.
	attrs, err := parseSectionSpecificAttributes(dataMatch)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse all sections in parseSectionSpecificAttributes")
	}

	return &Phy{
		Name:           name,
		Bands:          attrs.bands,
		Modes:          attrs.phyModes,
		Commands:       attrs.phyCommands,
		Features:       phyFeatures,
		RxAntenna:      rxAntenna,
		TxAntenna:      txAntenna,
		MaxScanSSIDs:   maxScanSSIDs,
		SupportVHT:     attrs.supportVHT,
		SupportHT2040:  attrs.supportHT2040,
		SupportHT40SGI: attrs.supportHT40SGI,
	}, nil
}

// parseScanResults parses the output of `scan` and `scan dump` commands into
// a slice of BSSData structs.
func parseScanResults(output string) ([]*BSSData, error) {
	matches, splits, err := splitTextOnRegexMatches(`BSS ([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}`, output)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse scan results")
	}
	var bssList []*BSSData
	for i, m := range matches {
		data, err := newBSSData(m, splits[i])
		if err != nil {
			return nil, err
		}
		bssList = append(bssList, data)
	}
	return bssList, nil
}

func parseBand(attrs *sectionAttributes, sectionName string, contents string) error {
	// This parser constructs a Band datastructure for the phy.
	m, err := extractMatch(`^Band (\d+):$`, sectionName)
	if err != nil {
		return errors.Wrap(err, "failed to parse band")
	}
	num, err := strconv.Atoi(m[0])
	if err != nil {
		return errors.New("could not convert num to string")
	}
	currentBand := Band{num, make(map[int][]string), nil}
	// Band rate handling.
	matches := regexp.MustCompile(`HT TX/RX MCS rate indexes supported: .*\n`).FindAllStringSubmatch(contents, -1)
	for _, m := range matches {
		rateStr := strings.TrimSpace(strings.Split(m[0], ":")[1])
		for _, piece := range strings.Split(rateStr, ",") {
			if strings.Contains(piece, "-") {
				res := strings.SplitN(piece, "-", 2)
				begin, _ := strconv.Atoi(res[0])
				end, _ := strconv.Atoi(res[1])
				for i := begin; i < end+1; i++ {
					currentBand.McsIndices = append(currentBand.McsIndices, i)
				}

			} else {
				val, _ := strconv.Atoi(piece)
				currentBand.McsIndices = append(currentBand.McsIndices, val)
			}
		}
	}

	// Band channel info handling.
	r := regexp.MustCompile(`(?P<frequency>\d+) MHz (\[\d+\])(?: \(([0-9.]+ dBm)\))?(?: \((?P<flags>[a-zA-Z, ]+)\))?`)
	matches = r.FindAllStringSubmatch(contents, -1)
	var frequency int
	for _, m := range matches {
		for i, tag := range r.SubexpNames() {
			if string(tag) == "frequency" {
				frequency, err = strconv.Atoi(m[i])
				if err != nil {
					return errors.Wrapf(err, "could not convert frequency %s to string", m[i])
				}
			} else if string(tag) == "flags" {
				flags := strings.Split(string(m[i]), ",")
				if len(flags) > 0 && flags[0] != "" {
					currentBand.FrequencyFlags[frequency] = flags
				}

			}
		}
	}
	attrs.bands = append(attrs.bands, currentBand)
	return nil
}

func parseThroughput(attrs *sectionAttributes, sectionName string, contents string) error {
	// This parser evaluates the throughput capabilities of the phy.
	if strings.Contains(contents, "VHT Capabilities") {
		attrs.supportVHT = true
	}
	if strings.Contains(contents, "HT20/HT40") {
		attrs.supportHT2040 = true
	}
	if strings.Contains(contents, "RX HT40 SGI") {
		attrs.supportHT40SGI = true
	}
	return nil
}
func parseIfaceModes(attrs *sectionAttributes, sectionName string, contents string) error {
	// This parser checks the supported interface modes for the phy.
	matches := regexp.MustCompile(`\* (\w+)`).FindAllStringSubmatch(contents, -1)
	for _, m := range matches {
		attrs.phyModes = append(attrs.phyModes, m[1])
	}
	return nil
}
func parsePhyCommands(attrs *sectionAttributes, sectionName string, contents string) error {
	// This parser checks the Phy's supported commands.
	matches := regexp.MustCompile(`\* (\w+)`).FindAllStringSubmatch(contents, -1)
	for _, m := range matches {
		attrs.phyCommands = append(attrs.phyCommands, m[1])
	}
	return nil
}

var parsers = []struct {
	prefix string
	parse  func(attrs *sectionAttributes, sectionName string, contents string) error
}{
	{
		prefix: "Band",
		parse:  parseBand,
	},
	{
		prefix: "Band",
		parse:  parseThroughput,
	},
	{
		prefix: "Supported interface modes",
		parse:  parseIfaceModes,
	},
	{
		prefix: "Supported commands",
		parse:  parsePhyCommands,
	},
}

func parseSectionSpecificAttributes(output string) (*sectionAttributes, error) {
	attrs := sectionAttributes{}
	matches, splits, err := splitTextOnRegexMatches(`(?m)^\t(\w.*):\s*$`, output)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse sections")
	}
	// The following for loop will loop through sections with each parser,
	// check if the parser is usable, and then parse the section.
	// Results are stored in the struct.
	for i, m := range matches {
		m = strings.TrimSpace(m)
		for _, parser := range parsers {
			if strings.HasPrefix(m, parser.prefix) {
				if err := parser.parse(&attrs, m, splits[i]); err != nil {
					return nil, err
				}
			}
		}
	}
	return &attrs, nil
}

func extractMatch(regex, text string) ([]string, error) {
	r := regexp.MustCompile(regex)
	m := r.FindStringSubmatch(text)
	if len(m) != r.NumSubexp()+1 {
		return nil, errors.New("could not parse MatchGroup")
	}
	return m[1:], nil
}

func splitTextOnRegexMatches(regex, text string) (matches []string, sections []string, err error) {
	r := regexp.MustCompile(regex)
	matches = r.FindAllString(text, -1)
	sections = r.Split(text, -1)
	if len(sections) != len(matches)+1 {
		return nil, nil, errors.New("unexpected number of matches")
	}
	sections = sections[1:]
	return matches, sections, nil
}
