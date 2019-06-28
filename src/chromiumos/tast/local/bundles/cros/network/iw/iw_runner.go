// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package iw contains utility functions to wrap around the iw program.
package iw

import (
	"context"
	"fmt"
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

// ScanDump will return a list of BSSData from a scan dump.
func ScanDump(ctx context.Context, iface string) ([]*BSSData, error) {
	args := []string{"dev", iface, "scan", "dump"}
	out, err := iwRun(ctx, args...)
	if err != nil {
		err = errors.Wrapf(err, "scan dump failed")
		return nil, err
	}
	return parseScanResults(out)
}

// GetFragmentationThreshold yields the phy's fragmentation threshold in number of bytes.
func GetFragmentationThreshold(ctx context.Context, phy string) (int, error) {
	args := []string{"phy", phy, "info"}
	out, err := iwRun(ctx, args...)
	if err != nil {
		err = errors.Wrap(err, "failed to get phy info")
		return 0, err
	}
	r := regexp.MustCompile(`^\s+Fragmentation threshold:\s+([0-9]+)$`)
	for _, line := range strings.Split(out, "\n") {
		m := r.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		thresh, err := strconv.Atoi(m[1])
		if err != nil {
			errors.New("could not parse threshold")
			return 0, err
		}
		return thresh, nil
	}
	return 0, errors.New("could not find threshold")
}

// GetLinkValue will get the specified link value from the iw link output.
func GetLinkValue(ctx context.Context, iface string, iwLinkKey string) (string, error) {
	args := []string{"dev", iface, "link"}
	out, err := iwRun(ctx, args...)
	if err != nil {
		err = errors.Wrapf(err, "failed to get link information from interface %s", iface)
		return "", err
	}
	out = getAllLinkKeys(out)[iwLinkKey]
	if out == "" {
		err = errors.Errorf("could not extract link value from link information with link key %s", iwLinkKey)
		return "", err
	}
	return out, nil
}

//GetOperatingMode will get the interface's operating mode.
func GetOperatingMode(ctx context.Context, iface string) (string, error) {
	args := []string{"dev", iface, "info"}
	out, err := iwRun(ctx, args...)
	if err != nil {
		err = errors.Wrapf(err, "failed to get interface information")
		return "", nil
	}
	r := regexp.MustCompile(`^\s*type (.*)$`)
	supportedDevModes := []string{"AP", "monitor", "managed"}
	for _, line := range strings.Split(out, "\n") {
		m := r.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		opMode := m[1]
		for _, v := range supportedDevModes {
			if v == opMode {
				return opMode, nil
			}
		}
		err = errors.Wrap(err, fmt.Sprintf("Unsupported operating mode %s found for"+
			" interface: %s.", opMode, iface))
	}
	if err == nil {
		err = errors.New("could not find operating mode")
	}
	return "", err
}

// GetRadioConfig will get the radio configuration from the interface's information.
func GetRadioConfig(ctx context.Context, iface string) (ChannelConfig, error) {
	args := []string{"dev", iface, "info"}
	out, err := iwRun(ctx, args...)
	if err != nil {
		err = errors.Wrapf(err, "failed to get interface information")
		return ChannelConfig{}, err
	}
	r := regexp.MustCompile(
		`^\s*channel ([0-9]+) \(([0-9]+) MHz\), width: ([2,4,8]0) MHz, center1: ([0-9]+) MHz`)
	for _, line := range strings.Split(out, "\n") {
		m := r.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		var number, freq, width, center1Freq int
		if val, err := strconv.Atoi(m[1]); err != nil {
			number = val
			err = errors.New("could not parse number")
			return ChannelConfig{}, err
		}
		if val, err := strconv.Atoi(m[2]); err != nil {
			freq = val
			err = errors.New("could not parse freq")
			return ChannelConfig{}, err
		}
		if val, err := strconv.Atoi(m[3]); err != nil {
			width = val
			err = errors.New("could not parse width")
			return ChannelConfig{}, err
		}
		if val, err := strconv.Atoi(m[4]); err != nil {
			center1Freq = val
			err = errors.New("could not parse center1Freq")
			return ChannelConfig{}, err
		}
		return ChannelConfig{
			Number:      number,
			Freq:        freq,
			Width:       width,
			Center1Freq: center1Freq,
		}, nil
	}
	return ChannelConfig{}, errors.New("could not find radio config")
}

// GetRegulatoryDomain will get the regulatory domain.
func GetRegulatoryDomain(ctx context.Context) (string, error) {
	args := []string{"reg", "get"}
	out, err := iwRun(ctx, args...)
	if err != nil {
		err = errors.Wrap(err, "failed to get regulatory domain")
		return "", err
	}
	r := regexp.MustCompile(`^country (..):`)
	for _, line := range strings.Split(out, "\n") {
		if r.MatchString(line) {
			m := r.FindStringSubmatch(line)
			return m[1], nil
		}
	}
	return "", errors.New("could not find regulatory domain")
}

// SetTxPower will set the wireless interface's transmit power.
func SetTxPower(ctx context.Context, iface string, power string) error {
	args := []string{"dev", iface, "set", "txpower", power}
	_, err := iwRun(ctx, args...)
	if err != nil {
		err = errors.Wrap(err, "failed to set txpower")
	}
	return err
}

// SetFreq will set the wireless interface's LO center freq.
func SetFreq(ctx context.Context, iface string, freq int) error {
	args := []string{"dev", iface, "set", "freq", strconv.Itoa(freq)}
	_, err := iwRun(ctx, args...)
	if err != nil {
		err = errors.Wrap(err, "failed to set freq")
	}
	return err
}

// SetRegulatoryDomain will set set the regulatory domain.
func SetRegulatoryDomain(ctx context.Context, domainString string) error {
	args := []string{"reg", "set", domainString}
	_, err := iwRun(ctx, args...)
	if err != nil {
		err = errors.Wrap(err, "failed to set regulatory domain")
	}
	return err
}

// SetAntennaBitmap will set the antenna bitmap.
func SetAntennaBitmap(ctx context.Context, phy string, txBitmap int, rxBitmap int) error {
	args := []string{"phy", phy, "set", "antenna", strconv.Itoa(txBitmap), strconv.Itoa(rxBitmap)}
	_, err := iwRun(ctx, args...)
	if err != nil {
		err = errors.Wrap(err, "failed to set Antenna bitmap")
	}
	return err
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

// iwRun runs an iw command and returns a string representation of the output.
func iwRun(ctx context.Context, arg ...string) (string, error) {
	res, err := testexec.CommandContext(ctx, "iw", arg...).Output()
	out := string(res)
	if err != nil {
		str := strings.Join(arg, " ")
		err = errors.Wrap(err, str)
	}
	return out, err
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
