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

// BSSData contains contents pertaining to a BSS response.
type BSSData struct {
	BSS       string
	Frequency int
	SSID      string
	Security  string
	HT        string
	Signal    float64
}

// TimedScanData contains the BSS responses from an `iw scan` and its execution time.
type TimedScanData struct {
	Time    time.Duration
	BSSList []*BSSData
}

// ChannelConfig contains the configuration data for a radio config.
type ChannelConfig struct {
	Number, Freq, Width, Center1Freq int
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
	out, err := testexec.CommandContext(ctx, "iw", args...).Output(testexec.DumpLogOnError)
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
	out := getAllLinkKeys(string(res))[iwLinkKey]
	if out == "" {
		return "", errors.Errorf("could not extract link value from link information with link key %s: %v", iwLinkKey, getAllLinkKeys(string(res)))
	}
	return out, nil
}

//GetOperatingMode gets the interface's operating mode.
func GetOperatingMode(ctx context.Context, iface string) (string, error) {
	out, err := testexec.CommandContext(ctx, "iw", "dev", iface,
		"info").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to get interface information")
	}
	r := regexp.MustCompile(`(m)^\s*type (.*)$`)
	supportedDevModes := []string{"AP", "monitor", "managed"}
	m := r.FindStringSubmatch(string(out))
	if m == nil {
		return "", errors.New("could not find operating mode")
	}
	opMode := m[1]
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
	r := regexp.MustCompile(
		`(?m)^\s*channel (\d+) \((\d+) MHz\), width: (\d{2}) MHz, center1: (\d+) MHz`)
	m := r.FindStringSubmatch(string(out))
	if m == nil {
		return nil, errors.New("could not find radio config")
	}
	number, err := strconv.Atoi(m[1])
	if err != nil {
		return nil, errors.New("could not parse number")
	}
	freq, err := strconv.Atoi(m[2])
	if err != nil {
		return nil, errors.New("could not parse freq")
	}
	width, err := strconv.Atoi(m[3])
	if err != nil {
		return nil, errors.New("could not parse width")
	}
	center1Freq, err := strconv.Atoi(m[4])
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

// parseScanResults parses the output of `scan` and `scan dump` commands into
// a slice of BSSData structs.
func parseScanResults(output string) ([]*BSSData, error) {
	re := regexp.MustCompile(`BSS ([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}`)
	matches := re.FindAllString(output, -1)
	splits := re.Split(output, -1)
	if len(splits) != len(matches)+1 {
		return nil, errors.New("unexpected number of matches")
	}
	var bssList []*BSSData
	for i, m := range matches {
		data, err := newBSSData(m, splits[i+1])
		if err != nil {
			return nil, err
		}
		bssList = append(bssList, data)
	}
	return bssList, nil
}
