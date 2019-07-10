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
	McsIndicies    []int
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
	Phy            int
	IfName, IfType string
}

// Phy contains phy# attributes.
type Phy struct {
	Name                                      string
	Bands                                     []Band
	Modes, Commands, Features                 []string
	RxAntennas, TxAntennas                    []int
	MaxScanSSIDs                              int
	SupportVHT, SupportHT2040, SupportHT40SGI bool
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
	matchIfs := []*NetDev{}
	ifs, err := ListInterfaces(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "ListInterfaces failed")
	}
	for _, val := range ifs {
		if val.IfName == iface {
			matchIfs = append(matchIfs, val)
		}
	}
	if len(matchIfs) != 1 {
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
	interfaces := []*NetDev{}

	ms, s, err := splitTextOnRegexMatches(`phy#([0-9]+)`, string(out))
	if err != nil {
		return nil, errors.Wrap(err, "could not parse netDev")
	}
	for i, phy := range ms {
		ifaces, sections, err := splitTextOnRegexMatches(`[\s]*Interface (.*)`, s[i])
		if err != nil {
			return nil, errors.Wrap(err, "could not parse interface")
		}
		for i, iface := range ifaces {
			netdev, err := newNetDev(phy, iface, sections[i])
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

	ms, s, err := splitTextOnRegexMatches(`Wiphy (.*)`, string(out))
	if err != nil {
		return nil, errors.Wrap(err, "could not parse phys")
	}
	var phys []*Phy
	for i, m := range ms {
		phy, err := newPhy(m, s[i])
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
	if len(ssidMatch) == len("SSID:") || len(ssidMatch) == 0 {
		return nil, errors.New("could not valid SSID")
	}
	ssid := strings.TrimSpace(ssidMatch[len("SSID:")+1 : len(ssidMatch)])

	// Handle high throughput setting.
	htMatch := regexp.MustCompile(
		`\* secondary channel offset.*`).FindString(dataMatch)
	htSplits := strings.Split(htMatch, ":")
	if len(htSplits) != 2 {
		return nil, errors.New("unexpected pattern for high throughput setting")
	}
	ht, ok := htTable[strings.TrimSpace(htSplits[1])]
	if !ok {
		return nil, errors.Errorf("invalid HT entry parsed %s",
			strings.TrimSpace(htSplits[1]))
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
	m := regexp.MustCompile(`phy#([0-9]+)`).FindStringSubmatch(phystr)
	if len(m) != 2 {
		return nil, errors.New("unexpected input when parsing phy number")
	}
	phy, err := strconv.Atoi(m[1])
	if err != nil {
		return nil, errors.Wrapf(err, "could not convert str %s to int", m[1])
	}

	// Parse ifType
	m = regexp.MustCompile(`[\s]*type ([a-zA-Z]+)`).FindStringSubmatch(dataMatch)
	if len(m) != 2 {
		return nil, errors.New("unexpected input when parsing ifType")
	}
	ifType := m[1]
	return &NetDev{Phy: phy, IfName: ifName, IfType: ifType}, nil
}

// newPhy is a factory method that constructs a Phy struct from `iw list` output.
func newPhy(phyMatch string, dataMatch string) (*Phy, error) {
	// Phy name handling.
	m := regexp.MustCompile(`Wiphy (.*)`).FindStringSubmatch(phyMatch)
	if len(m) != 2 {
		return nil, errors.New("unexpected input when parsing name")
	}
	name := m[1]

	// Antennae handling.
	var rxAntennas, txAntennas []int
	hexToInt := func(str string) (int, error) {
		res, err := strconv.ParseInt(strings.Replace(str, "0x", "", -1), 16, 64)
		if err != nil {
			return 0, errors.Wrap(err, "could not parse hex string")
		}
		return int(res), nil
	}
	m = regexp.MustCompile(`\s*Available Antennas: TX (\S+) RX (\S+)`).FindStringSubmatch(dataMatch)
	if len(m) != 3 {
		return nil, errors.New("unexpected input when parsing antennas")
	}
	ant1, err := hexToInt(m[1])
	if err != nil {
		return nil, err
	}
	ant2, err := hexToInt(m[2])
	if err != nil {
		return nil, err
	}
	txAntennas = append(txAntennas, ant1)
	rxAntennas = append(rxAntennas, ant2)

	// Device Support handling.
	var phyFeatures []string
	ms := regexp.MustCompile(`\s*Device supports (.*)\.`).FindAllStringSubmatch(dataMatch, -1)
	for _, m := range ms {
		if len(m) != 2 {
			return nil, errors.New("unexpected input when parsing device support")
		}
		phyFeatures = append(phyFeatures, m[1])
	}

	// Max Scan SSIDs handling.
	m = regexp.MustCompile(`\s*max # scan SSIDs: (\d+)`).FindStringSubmatch(dataMatch)
	if len(m) != 2 {
		return nil, errors.New("could not find max scan SSIDs when parsing data")
	}
	maxScanSSIDs, err := strconv.Atoi(m[1])
	if err != nil {
		return nil, errors.Wrapf(err, "could not convert maxScanSSIDs %s to string", m[1])
	}

	// Handle parsing attributes that need to be handled on a section by section level.
	// Sections are defined as blocks of text that are delimited by level 1 indent lines.
	sectionAttrs, err := parseSectionSpecificAttributes(dataMatch)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse all sections in parseSectionSpecificAttributes")
	}

	return &Phy{
		Name:           name,
		Bands:          sectionAttrs.bands,
		Modes:          sectionAttrs.phyModes,
		Commands:       sectionAttrs.phyCommands,
		Features:       phyFeatures,
		RxAntennas:     rxAntennas,
		TxAntennas:     txAntennas,
		MaxScanSSIDs:   maxScanSSIDs,
		SupportVHT:     sectionAttrs.supportVHT,
		SupportHT2040:  sectionAttrs.supportHT2040,
		SupportHT40SGI: sectionAttrs.supportHT40SGI,
	}, nil
}

// parseScanResults parses the output of `scan` and `scan dump` commands into
// a slice of BSSData structs.
func parseScanResults(output string) ([]*BSSData, error) {
	ms, s, err := splitTextOnRegexMatches(`BSS ([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}`, output)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse scan results")
	}
	var bssList []*BSSData
	for i, m := range ms {
		data, err := newBSSData(m, s[i])
		if err != nil {
			return nil, err
		}
		bssList = append(bssList, data)
	}
	return bssList, nil
}

type parser struct {
	prefix string
	parse  func(attrs *sectionAttributes, sectionName string, contents string) error
}

func parseSectionSpecificAttributes(output string) (*sectionAttributes, error) {
	sectionAttrs := sectionAttributes{}
	parsers := []parser{
		{
			prefix: "Band",
			parse: func(attrs *sectionAttributes, sectionName string, contents string) error {
				m := regexp.MustCompile(`^Band (\d+):$`).FindStringSubmatch(sectionName)
				if len(m) != 2 {
					return errors.New("unexpected input when parsing band")
				}
				num, err := strconv.Atoi(m[1])
				if err != nil {
					return errors.New("could not convert num to string")
				}
				currentBand := Band{num, make(map[int][]string), []int{}}
				// Band rate handling.
				ms := regexp.MustCompile(`^HT TX/RX MCS rate indexes supported: .*\n`).FindAllStringSubmatch(contents, -1)
				for _, m := range ms {
					if len(m) != 1 {
						return errors.Errorf("unexpected input when parsing rates %d", len(m))
					}
					rateStr := strings.TrimSpace(strings.Split(m[0], ":")[1])
					for _, piece := range strings.Split(rateStr, ",") {
						if strings.Contains(piece, "-") {
							res := strings.SplitN(piece, "-", 2)
							begin, _ := strconv.Atoi(res[0])
							end, _ := strconv.Atoi(res[1])
							for i := begin; i < end+1; i++ {
								currentBand.McsIndicies = append(currentBand.McsIndicies, i)
							}

						} else {
							val, _ := strconv.Atoi(piece)
							currentBand.McsIndicies = append(currentBand.McsIndicies, val)
						}
					}
				}

				// Band channel info handling.
				r := regexp.MustCompile(`(?P<frequency>\d+) MHz (?P<chan_num>\[\d+\])(?: \((?P<tx_power_limit>[0-9.]+ dBm)\))?(?: \((?P<flags>[a-zA-Z, ]+)\))?`)
				ms = r.FindAllStringSubmatch(contents, -1)
				var frequency int
				for _, m := range ms {
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
			},
		},
		{
			prefix: "Band",
			parse: func(attrs *sectionAttributes, sectionName string, contents string) error {
				// Rate support handling.
				if !attrs.supportVHT && strings.Contains(contents, "VHT Capabilities") {
					attrs.supportVHT = true
				}
				if !attrs.supportHT2040 && strings.Contains(contents, "HT20/HT40") {
					attrs.supportHT2040 = true
				}
				if !attrs.supportHT40SGI && strings.Contains(contents, "RX HT40 SGI") {
					attrs.supportHT40SGI = true
				}
				return nil
			},
		},
		{
			prefix: "Supported interface modes",
			parse: func(attrs *sectionAttributes, sectionName string, contents string) error {
				// Phy modes handling.
				ms := regexp.MustCompile(`\* (\w+)`).FindAllStringSubmatch(contents, -1)
				for _, m := range ms {
					if len(m) != 2 {
						return errors.New("unexpected input when parsing phy modes")
					}
					attrs.phyModes = append(attrs.phyModes, m[1])
				}
				return nil
			},
		},
		{
			prefix: "Supported commands",
			parse: func(attrs *sectionAttributes, sectionName string, contents string) error {
				// Phy commands handling.
				ms := regexp.MustCompile(`\* (\w+)`).FindAllStringSubmatch(contents, -1)
				for _, m := range ms {
					if len(m) != 2 {
						return errors.New("unexpected input when parsing supported phy commands")
					}
					attrs.phyCommands = append(attrs.phyCommands, m[1])
				}
				return nil
			},
		},
	}
	ms, s, err := splitTextOnRegexMatches(`(?m)^\t(\w.*):\s*$`, output)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse sections")
	}
	// The following for loop will loop through sections with each parser,
	// check if the parser is usable, and then parse the section.
	// Results are stored in the struct.
	for i, m := range ms {
		m = strings.TrimSpace(m)
		for _, parser := range parsers {
			if strings.HasPrefix(m, parser.prefix) {
				if err := parser.parse(&sectionAttrs, m, s[i]); err != nil {
					return nil, err
				}
			}
		}
	}
	return &sectionAttrs, nil
}

func splitTextOnRegexMatches(regex, text string) (matches []string, sections []string, err error) {
	r := regexp.MustCompile(regex)
	matches = r.FindAllString(text, -1)
	sections = r.Split(text, -1)
	if len(sections) != len(matches)+1 {
		return nil, nil, errors.New("unexpected number of matches")
	}
	sections = sections[1:]
	return
}
