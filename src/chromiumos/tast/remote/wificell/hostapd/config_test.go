// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hostapd

import (
	"reflect"
	"strings"
	"testing"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/common/wifi/security/base"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/errors"
)

func TestNewConfig(t *testing.T) {
	// Test NewConfig and Config.validate.
	testcases := []struct {
		ops        []Option
		expected   *Config
		shouldFail bool
	}{
		// Check mode validation.
		{
			ops: []Option{
				Channel(1),
			},
			expected:   nil,
			shouldFail: true, // due to missing Mode.
		},
		// Check channel validation.
		{
			ops: []Option{
				Mode(Mode80211g),
			},
			expected:   nil,
			shouldFail: true, // due to missing Channel.
		},
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(1),
			},
			expected:   nil,
			shouldFail: true, // due to Channel not supported by Mode.
		},
		{
			ops: []Option{
				Mode(Mode80211g),
				Channel(36),
			},
			expected:   nil,
			shouldFail: true, // due to Channel not supported by Mode.
		},
		// Check HTCap validation.
		{
			ops: []Option{
				Mode(Mode80211g),
				Channel(1),
				HTCaps(HTCapHT40),
			},
			expected:   nil,
			shouldFail: true, // 80211g should not have HTCaps.
		},
		{
			ops: []Option{
				Mode(Mode80211nPure),
				Channel(1),
			},
			expected:   nil,
			shouldFail: true, // due to missing HTCaps on 802.11n mode.
		},
		{
			ops: []Option{
				Mode(Mode80211nPure),
				Channel(1),
				HTCaps(HTCapHT40Minus),
			},
			expected:   nil,
			shouldFail: true, // HT40- cannot be enabled on ch1.
		},
		{
			ops: []Option{
				Mode(Mode80211nMixed),
				Channel(161),
				HTCaps(HTCapHT40Plus),
			},
			expected:   nil,
			shouldFail: true, // HT40+ cannot be enabled on ch161.
		},
		// Check VHTCap validation.
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(36),
				VHTCaps(VHTCapSGI80),
			},
			expected:   nil,
			shouldFail: true, // should not set VHTCaps on mode other than 802.11ac
		},
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(36),
				VHTCenterChannel(42),
			},
			expected:   nil,
			shouldFail: true, // should not set VHTCenterChannel on mode other than 802.11ac
		},
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(36),
				VHTChWidth(VHTChWidth80),
			},
			expected:   nil,
			shouldFail: true, // should not set VHTChWidth on mode other than 802.11ac
		},
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(36),
				DTIMPeriod(256),
			},
			expected:   nil,
			shouldFail: true, // should not set DTIMPeriod to a value out of the range (1..255)
		},
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(36),
				DTIMPeriod(-2),
			},
			expected:   nil,
			shouldFail: true, // should not set DTIMPeriod to a value out of the range (1..255)
		},
		// Check beacon interval validation.
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(36),
				BeaconInterval(10),
			},
			expected:   nil,
			shouldFail: true, // BeaconInterval should be in 15...65535.
		},
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(36),
				BeaconInterval(66000),
			},
			expected:   nil,
			shouldFail: true, // BeaconInterval should be in 15...65535.
		},
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(36),
				DTIMPeriod(5),
				BSSID("00:11:22:33:44:55:00"),
			},
			expected:   nil,
			shouldFail: true, // should not set the BSSID if its length is over 17.
		},
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(36),
				DTIMPeriod(5),
				BSSID("00:11:22:33:44"),
			},
			expected:   nil,
			shouldFail: true, // should not set the BSSID if its length is under 17.
		},
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(36),
				MobilityDomain("efgh"),
			},
			expected:   nil,
			shouldFail: true, // mobility domain should be a hex string.
		},
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(36),
				MobilityDomain("012345"),
			},
			expected:   nil,
			shouldFail: true, // mobility domain should be 2-octet identifier as a hex string.
		},
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(36),
				R1KeyHolder("1122334455gg"),
			},
			expected:   nil,
			shouldFail: true, // r1 key holder should be a hex string.
		},
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(36),
				R1KeyHolder("11223344556677"),
			},
			expected:   nil,
			shouldFail: true, // r1 key holder identifier should be 6-octet identifier as a hex string.
		},
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(36),
				R0KHs("02:01:02:03:04:05 r0kh-1.example.com 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f extra fields"),
			},
			expected:   nil,
			shouldFail: true, // Key holders should have exactly three fields.
		},
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(36),
				R0KHs("02:01:02:03:04:05 r0kh-1.example.com"),
			},
			expected:   nil,
			shouldFail: true, // Key holders should have exactly three fields.
		},
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(36),
				R0KHs("02:01:02:03:0405 r0kh-1.example.com 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"),
			},
			expected:   nil,
			shouldFail: true, // The first field should be a MAC address.
		},
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(36),
				R1KHs("02:01:02:03:04:05 r0kh-1.example.com 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"),
			},
			expected:   nil,
			shouldFail: true, // The second field of r1kh should be a MAC address.
		},
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(36),
				R0KHs("02:01:02:03:04:05 r0kh-1.example.com zz0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"),
			},
			expected:   nil,
			shouldFail: true, // The third field should be a hex string.
		},
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(36),
				R0KHs("02:01:02:03:04:05 r0kh-1.example.com 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e"),
			},
			expected:   nil,
			shouldFail: true, // The third field should be 256 bits.
		},
		// Good cases.
		{
			ops: []Option{
				SSID("ssid"),
				Mode(Mode80211a),
				Channel(36),
				DTIMPeriod(5),
				BSSID("00:11:22:33:44:55"),
			},
			expected: &Config{
				SSID:           "ssid",
				Mode:           Mode80211a,
				Channel:        36,
				HTCaps:         0,
				SecurityConfig: &base.Config{},
				DTIMPeriod:     5,
				BSSID:          "00:11:22:33:44:55",
			},
			shouldFail: false,
		},
		{
			ops: []Option{
				SSID("ssid"),
				BSSID("00:11:22:33:44:55"),
				Mode(Mode80211g),
				Channel(1),
				DTIMPeriod(254),
			},
			expected: &Config{
				SSID:           "ssid",
				BSSID:          "00:11:22:33:44:55",
				Mode:           Mode80211g,
				Channel:        1,
				HTCaps:         0,
				SecurityConfig: &base.Config{},
				DTIMPeriod:     254,
			},
			shouldFail: false,
		},
		{
			ops: []Option{
				SSID("ssid"),
				BSSID("00:11:22:33:44:55"),
				Mode(Mode80211nMixed),
				Channel(1),
				HTCaps(HTCapHT20),
				DTIMPeriod(1),
			},
			expected: &Config{
				SSID:           "ssid",
				BSSID:          "00:11:22:33:44:55",
				Mode:           Mode80211nMixed,
				Channel:        1,
				HTCaps:         HTCapHT20,
				SecurityConfig: &base.Config{},
				DTIMPeriod:     1,
			},
			shouldFail: false,
		},
		{
			ops: []Option{
				SSID("ssid"),
				BSSID("00:11:22:33:44:55"),
				Mode(Mode80211nPure),
				Channel(36),
				HTCaps(HTCapHT40),
				HTCaps(HTCapSGI20),
				DTIMPeriod(100),
			},
			expected: &Config{
				SSID:           "ssid",
				BSSID:          "00:11:22:33:44:55",
				Mode:           Mode80211nPure,
				Channel:        36,
				HTCaps:         HTCapHT40 | HTCapSGI20,
				SecurityConfig: &base.Config{},
				DTIMPeriod:     100,
			},
			shouldFail: false,
		},
		{
			ops: []Option{
				SSID("ssid"),
				BSSID("00:11:22:33:44:55"),
				Mode(Mode80211acPure),
				Channel(157),
				HTCaps(HTCapHT40Plus),
				VHTCaps(VHTCapSGI80),
				VHTCenterChannel(155),
				VHTChWidth(VHTChWidth80),
				DTIMPeriod(50),
			},
			expected: &Config{
				SSID:             "ssid",
				BSSID:            "00:11:22:33:44:55",
				Mode:             Mode80211acPure,
				Channel:          157,
				HTCaps:           HTCapHT40Plus,
				VHTCaps:          []VHTCap{VHTCapSGI80},
				VHTCenterChannel: 155,
				VHTChWidth:       VHTChWidth80,
				SecurityConfig:   &base.Config{},
				DTIMPeriod:       50,
			},
			shouldFail: false,
		},
		{
			ops: []Option{
				SSID("ssid"),
				BSSID("00:11:22:33:44:55"),
				Mode(Mode80211a),
				Channel(36),
				Hidden(),
				DTIMPeriod(200),
			},
			expected: &Config{
				SSID:           "ssid",
				BSSID:          "00:11:22:33:44:55",
				Mode:           Mode80211a,
				Channel:        36,
				HTCaps:         0,
				Hidden:         true,
				SecurityConfig: &base.Config{},
				DTIMPeriod:     200,
			},
			shouldFail: false,
		},
		{
			ops: []Option{
				SSID("ssid"),
				BSSID("00:11:22:33:44:55"),
				Mode(Mode80211a),
				Channel(36),
				OBSSInterval(5),
			},
			expected: &Config{
				SSID:           "ssid",
				BSSID:          "00:11:22:33:44:55",
				Mode:           Mode80211a,
				Channel:        36,
				HTCaps:         0,
				SecurityConfig: &base.Config{},
				OBSSInterval:   5,
			},
			shouldFail: false,
		},
		{
			ops: []Option{
				SSID("ssid"),
				BSSID("00:11:22:33:44:55"),
				Mode(Mode80211a),
				Channel(36),
				Bridge("br0"),
				MobilityDomain("0123"),
				NASIdentifier("r0kh-1.example.com"),
				R1KeyHolder("112233445566"),
				R0KHs("02:01:02:03:04:05 r0kh-1.example.com 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"),
				R1KHs("02:01:02:03:04:05 02:01:02:03:04:05 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"),
			},
			expected: &Config{
				SSID:           "ssid",
				BSSID:          "00:11:22:33:44:55",
				Mode:           Mode80211a,
				Channel:        36,
				SecurityConfig: &base.Config{},
				Bridge:         "br0",
				MobilityDomain: "0123",
				NASIdentifier:  "r0kh-1.example.com",
				R1KeyHolder:    "112233445566",
				R0KHs:          []string{"02:01:02:03:04:05 r0kh-1.example.com 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"},
				R1KHs:          []string{"02:01:02:03:04:05 02:01:02:03:04:05 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"},
			},
			shouldFail: false,
		},
		{
			ops: []Option{
				SSID("ssid"),
				BSSID("00:11:22:33:44:55"),
				Mode(Mode80211a),
				Channel(36),
				BasicRates(6),
				SupportedRates(6, 11),
			},
			expected: &Config{
				SSID:           "ssid",
				BSSID:          "00:11:22:33:44:55",
				Mode:           Mode80211a,
				Channel:        36,
				HTCaps:         0,
				SecurityConfig: &base.Config{},
				BasicRates:     []float32{6.0},
				SupportedRates: []float32{6.0, 11.0},
			},
			shouldFail: false,
		},
	}

	for i, tc := range testcases {
		conf, err := NewConfig(tc.ops...)
		if tc.shouldFail {
			if err == nil {
				t.Errorf("testcase %d should not pass config validation", i)
			}
			continue
		}
		if err != nil {
			t.Errorf("testcase %d NewConfig failed with err=%s", i, err.Error())
		}
		if !reflect.DeepEqual(conf, tc.expected) {
			t.Errorf("testcase %d got %v but expect %v", i, conf, tc.expected)
		}
	}
}

func parseConfigString(s string) (map[string]string, []map[string]string, error) {
	var ret map[string]string
	var additionalBSSs []map[string]string
	bss := make(map[string]string)
	multipleBSSs := false
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			continue
		}
		tokens := strings.SplitN(line, "=", 2)
		if len(tokens) != 2 {
			return nil, nil, errors.Errorf("invalid config line: %q", line)
		}
		if tokens[0] == "bss" {
			if multipleBSSs {
				additionalBSSs = append(additionalBSSs, bss)
			} else {
				ret = bss
				multipleBSSs = true
			}
			bss = make(map[string]string)
		}
		bss[strings.TrimSpace(tokens[0])] = strings.TrimSpace(tokens[1])
	}
	if multipleBSSs {
		additionalBSSs = append(additionalBSSs, bss)
	} else {
		ret = bss
	}
	return ret, additionalBSSs, nil
}

func TestConfigFormat(t *testing.T) {
	// Test Config.Format.

	// Fixed input for Config.Format call.
	const iface = "tiface0"
	const ctrl = "t.ctrl"

	testcases := []struct {
		conf   *Config
		verify map[string]string // fields to verify
		bsss   []map[string]string
	}{
		// Check basic fields.
		{
			conf: &Config{
				SSID:           "ssid000",
				Mode:           Mode80211b,
				Channel:        1,
				SecurityConfig: &base.Config{},
				DTIMPeriod:     200,
				BSSID:          "00:11:22:33:44:55",
			},
			verify: map[string]string{
				"ssid2":          `P"ssid000"`,
				"hw_mode":        "b",
				"channel":        "1",
				"interface":      iface,
				"ctrl_interface": ctrl,
				"dtim_period":    "200",
				"bssid":          "00:11:22:33:44:55",
			},
		},
		// Check 802.11n pure.
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211nPure,
				Channel:        3,
				SecurityConfig: &base.Config{},
				DTIMPeriod:     5,
			},
			verify: map[string]string{
				"hw_mode":     "g",
				"channel":     "3",
				"ieee80211n":  "1",
				"require_ht":  "1",
				"dtim_period": "5",
			},
		},
		// Check ht_capab.
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211nPure,
				Channel:        40,
				HTCaps:         HTCapHT20,
				SecurityConfig: &base.Config{},
			},
			verify: map[string]string{
				"hw_mode":    "a",
				"channel":    "40",
				"ieee80211n": "1",
				"require_ht": "1",
				"ht_capab":   "", // empty for HTCapHT20 only case.
			},
		},
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211nMixed,
				Channel:        36,
				HTCaps:         HTCapHT40 | HTCapSGI20 | HTCapSGI40,
				SecurityConfig: &base.Config{},
			},
			verify: map[string]string{
				"hw_mode":    "a",
				"channel":    "36",
				"ieee80211n": "1",
				"require_ht": "", // not set
				"ht_capab":   "[HT40+][SHORT-GI-20][SHORT-GI-40]",
			},
		},
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211nMixed,
				Channel:        5,
				HTCaps:         HTCapHT40 | HTCapSGI40,
				SecurityConfig: &base.Config{},
			},
			verify: map[string]string{
				"hw_mode":    "g",
				"channel":    "5",
				"ieee80211n": "1",
				"ht_capab":   "[HT40-][SHORT-GI-40]",
			},
		},
		// Check vht_capab.
		{
			conf: &Config{
				SSID:             "ssid",
				Mode:             Mode80211acPure,
				Channel:          157,
				HTCaps:           HTCapHT40Plus,
				VHTCaps:          []VHTCap{VHTCapSGI80},
				VHTCenterChannel: 155,
				VHTChWidth:       VHTChWidth80,
				SecurityConfig:   &base.Config{},
			},
			verify: map[string]string{
				"hw_mode":                      "a",
				"channel":                      "157",
				"ieee80211n":                   "1",
				"ht_capab":                     "[HT40+]",
				"ieee80211ac":                  "1",
				"vht_oper_chwidth":             "1",
				"vht_oper_centr_freq_seg0_idx": "155",
				"vht_capab":                    "[SHORT-GI-80]",
				"require_vht":                  "1",
			},
		},
		{
			conf: &Config{
				SSID:             "ssid",
				Mode:             Mode80211acMixed,
				Channel:          36,
				HTCaps:           HTCapHT40Plus,
				VHTCenterChannel: 42,
				VHTChWidth:       VHTChWidth80,
				SecurityConfig:   &base.Config{},
			},
			verify: map[string]string{
				"hw_mode":                      "a",
				"channel":                      "36",
				"ieee80211n":                   "1",
				"ht_capab":                     "[HT40+]",
				"ieee80211ac":                  "1",
				"vht_oper_chwidth":             "1",
				"vht_oper_centr_freq_seg0_idx": "42",
				"vht_capab":                    "",
			},
		},
		// Check hidden.
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211b,
				Channel:        1,
				Hidden:         true,
				SecurityConfig: &base.Config{},
			},
			verify: map[string]string{
				"ignore_broadcast_ssid": "1",
			},
		},
		// Check non-ASCII SSIDs.
		{
			conf: &Config{
				SSID:           "\a\b\f\n\r\t\v'\"\x1b", // Escaped characters.
				Mode:           Mode80211b,
				Channel:        1,
				SecurityConfig: &base.Config{},
			},
			verify: map[string]string{
				"ssid2": `P"\x07\x08\x0c\n\r\t\x0b'\"\e"`,
			},
		},
		{
			conf: &Config{
				SSID:           "\xe9\x89\xbb", // UTF-8
				Mode:           Mode80211b,
				Channel:        1,
				SecurityConfig: &base.Config{},
			},
			verify: map[string]string{
				"ssid2": `P"\xe9\x89\xbb"`,
			},
		},
		{
			conf: &Config{
				SSID:           "\xf2\xe3\x00\xd4\xc5\xb6", // Random binary
				Mode:           Mode80211b,
				Channel:        1,
				SecurityConfig: &base.Config{},
			},
			verify: map[string]string{
				"ssid2": `P"\xf2\xe3\x00\xd4\xc5\xb6"`,
			},
		},
		// Check PMF.
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211b,
				Channel:        1,
				Hidden:         true,
				SecurityConfig: &base.Config{},
				PMF:            PMFRequired,
			},
			verify: map[string]string{
				"ieee80211w": "2",
			},
		},
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211b,
				Channel:        1,
				Hidden:         true,
				SecurityConfig: &base.Config{},
				PMF:            PMFOptional,
			},
			verify: map[string]string{
				"ieee80211w": "1",
			},
		},
		// Check spectrum management.
		{
			conf: &Config{
				SSID:               "ssid",
				Mode:               Mode80211b,
				Channel:            1,
				SpectrumManagement: true,
				SecurityConfig:     &base.Config{},
			},
			verify: map[string]string{
				"country_code":           "US",
				"ieee80211d":             "1",
				"local_pwr_constraint":   "0",
				"spectrum_mgmt_required": "1",
			},
		},
		// Check beacon interval.
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211b,
				Channel:        1,
				SecurityConfig: &base.Config{},
				BeaconInterval: 200,
			},
			verify: map[string]string{
				"beacon_int": "200",
			},
		},
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211b,
				Channel:        1,
				SecurityConfig: &base.Config{},
				BeaconInterval: 0,
			},
			verify: map[string]string{
				"beacon_int": "", // Let hostapd have its default when not specified.
			},
		},
		// Check OBSS scan interval.
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211b,
				Channel:        1,
				SecurityConfig: &base.Config{},
				OBSSInterval:   5,
			},
			verify: map[string]string{
				"obss_interval": "5",
			},
		},
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211b,
				Channel:        1,
				SecurityConfig: &base.Config{},
				OBSSInterval:   0,
			},
			verify: map[string]string{
				"obss_interval": "", // Let hostapd have its default when not specified.
			},
		},
		// Check AdditionalBSSs
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211b,
				Channel:        1,
				SecurityConfig: &base.Config{},
				AdditionalBSSs: []AdditionalBSS{
					AdditionalBSS{"iface1", "ssid1", "00:11:22:33:44:55"},
					AdditionalBSS{"iface2", "ssid2", "55:44:33:22:11:00"},
				},
			},
			bsss: []map[string]string{
				{
					"bss":   "iface1",
					"ssid":  "ssid1",
					"bssid": "00:11:22:33:44:55",
				},
				{
					"bss":   "iface2",
					"ssid":  "ssid2",
					"bssid": "55:44:33:22:11:00",
				},
			},
		},
		// Check basic/supported rates.
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211b,
				Channel:        1,
				SecurityConfig: &base.Config{},
				BasicRates:     []float32{6.0},
				SupportedRates: []float32{6.0, 11.0, 24.0},
			},
			verify: map[string]string{
				"basic_rates":     "60",
				"supported_rates": "60 110 240",
			},
		},
	}

	for i, tc := range testcases {
		cs, err := tc.conf.Format(iface, ctrl)
		if err != nil {
			t.Errorf("testcase %d failed with err=%s", i, err.Error())
			continue
		}
		m, bsss, err := parseConfigString(cs)
		if err != nil {
			t.Errorf("testcase %d failed when parsing config string, err=%s", i, err.Error())
			continue
		}
		// Verify the requested fields.
		for k, v := range tc.verify {
			if v != m[k] {
				t.Errorf("testcase %d has %q=%q, expect %q", i, k, m[k], v)
			}
		}
		if !reflect.DeepEqual(bsss, tc.bsss) {
			t.Errorf("testcase %d has AdditionalBSSs=%v, expected %v", i, bsss, tc.bsss)
		}
	}
}

func TestFreqOptions(t *testing.T) {
	testcases := []struct {
		conf       *Config
		shouldFail bool
		expect     []iw.SetFreqOption
	}{
		{
			conf: &Config{
				SSID:    "ssid000",
				Mode:    Mode80211b,
				Channel: 1,
			},
			expect: []iw.SetFreqOption{iw.SetFreqChWidth(iw.ChWidthNOHT)},
		},
		{
			conf: &Config{
				SSID:    "ssid",
				Mode:    Mode80211nPure,
				Channel: 3,
			},
			expect: []iw.SetFreqOption{iw.SetFreqChWidth(iw.ChWidthHT20)},
		},
		{
			conf: &Config{
				SSID:    "ssid",
				Mode:    Mode80211nPure,
				Channel: 1,
				HTCaps:  HTCapHT40,
			},
			expect: []iw.SetFreqOption{iw.SetFreqChWidth(iw.ChWidthHT40Plus)},
		},
		{
			conf: &Config{
				SSID:    "ssid",
				Mode:    Mode80211nMixed,
				Channel: 5,
				HTCaps:  HTCapHT40 | HTCapSGI40,
			},
			expect: []iw.SetFreqOption{iw.SetFreqChWidth(iw.ChWidthHT40Minus)},
		},
		{
			conf: &Config{
				SSID:       "ssid",
				Mode:       Mode80211acMixed,
				Channel:    157,
				HTCaps:     HTCapHT40Plus,
				VHTChWidth: VHTChWidth20Or40,
			},
			expect: []iw.SetFreqOption{iw.SetFreqChWidth(iw.ChWidthHT40Plus)},
		},
		{
			conf: &Config{
				SSID:       "ssid",
				Mode:       Mode80211acMixed,
				Channel:    157,
				HTCaps:     HTCapHT40Plus,
				VHTChWidth: VHTChWidth80,
			},
			expect: []iw.SetFreqOption{iw.SetFreqChWidth(iw.ChWidth80)},
		},
		{
			conf: &Config{
				SSID:       "ssid",
				Mode:       Mode80211acMixed,
				Channel:    108,
				HTCaps:     HTCapHT40Plus,
				VHTChWidth: VHTChWidth160,
			},
			expect: []iw.SetFreqOption{iw.SetFreqChWidth(iw.ChWidth160)},
		},
		{
			// 80+80 not yet supported.
			conf: &Config{
				SSID:       "ssid",
				Mode:       Mode80211acMixed,
				Channel:    157,
				HTCaps:     HTCapHT40Plus,
				VHTChWidth: VHTChWidth80Plus80,
			},
			shouldFail: true,
		},
	}

	equal := func(a, b []iw.SetFreqOption) bool {
		if len(a) != len(b) {
			return false
		}
		for i, op := range a {
			if !op.Equal(b[i]) {
				return false
			}
		}
		return true
	}

	for i, tc := range testcases {
		ops, err := tc.conf.PcapFreqOptions()
		if tc.shouldFail {
			if err == nil {
				t.Errorf("testcase #%d should fail", i)
			}
			continue
		}
		if err != nil {
			t.Errorf("testcase #%d failed with err=%v", i, err)
			continue
		}
		if !equal(ops, tc.expect) {
			t.Errorf("testcase #%d failed, got %v, want %v", i, ops, tc.expect)
		}
	}
}

func TestPerfDesc(t *testing.T) {
	wpaConf, err := wpa.NewConfigFactory(
		"chromeos",
		wpa.Mode(wpa.ModePureWPA2),
		wpa.Ciphers(wpa.CipherTKIP),
	).Gen()
	if err != nil {
		t.Fatal("failed to prepare test wpa config")
	}

	testcases := []struct {
		conf   *Config
		expect string
	}{
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211b,
				Channel:        1,
				SecurityConfig: &base.Config{},
			},
			expect: "ch001_mode11b_none",
		},
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211nPure,
				Channel:        3,
				SecurityConfig: &base.Config{},
			},
			expect: "ch003_modeHT20_none",
		},
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211nPure,
				Channel:        1,
				HTCaps:         HTCapHT40,
				SecurityConfig: &base.Config{},
			},
			expect: "ch001_modeHT40p_none",
		},
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211nMixed,
				Channel:        5,
				HTCaps:         HTCapHT40 | HTCapSGI40,
				SecurityConfig: &base.Config{},
			},
			expect: "ch005_modeHT40m_none",
		},
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211acMixed,
				Channel:        157,
				HTCaps:         HTCapHT40Plus,
				VHTChWidth:     VHTChWidth20Or40,
				SecurityConfig: &base.Config{},
			},
			expect: "ch157_modeVHT40_none",
		},
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211acMixed,
				Channel:        157,
				HTCaps:         HTCapHT40Plus,
				VHTChWidth:     VHTChWidth80,
				SecurityConfig: &base.Config{},
			},
			expect: "ch157_modeVHT80_none",
		},
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211acMixed,
				Channel:        108,
				HTCaps:         HTCapHT40Plus,
				VHTChWidth:     VHTChWidth160,
				SecurityConfig: &base.Config{},
			},
			expect: "ch108_modeVHT160_none",
		},
		{
			conf: &Config{
				SSID:           "ssid",
				Mode:           Mode80211nPure,
				Channel:        3,
				SecurityConfig: wpaConf,
			},
			expect: "ch003_modeHT20_psk",
		},
	}

	for i, tc := range testcases {
		desc := tc.conf.PerfDesc()
		if desc != tc.expect {
			t.Errorf("testcase #%d failed, got %s, want %s", i, desc, tc.expect)
		}
	}
}

func TestDefaultBSSID(t *testing.T) {
	// Ensure that we have a default non-empty BSSID that can pass validation.
	conf, err := NewConfig(SSID("ssid"), Mode(Mode80211g), Channel(1))
	if err != nil {
		t.Errorf("NewConfig failed with err=%s", err.Error())
	}
	if conf.BSSID == "" {
		t.Errorf("Expect a non-empty default BSSID")
	}
}
