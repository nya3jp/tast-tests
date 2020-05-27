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
		/*
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
		*/
		// Good cases.
		{
			ops: []Option{
				SSID("ssid"),
				Mode(Mode80211a),
				Channel(36),
				//DTIMPeriod(5),
			},
			expected: &Config{
				Ssid:           "ssid",
				Mode:           Mode80211a,
				Channel:        36,
				HTCaps:         0,
				SecurityConfig: &base.Config{},
				//DTIMPeriod:     5,
			},
			shouldFail: false,
		},
		{
			ops: []Option{
				SSID("ssid"),
				Mode(Mode80211g),
				Channel(1),
				//DTIMPeriod(254),
			},
			expected: &Config{
				Ssid:           "ssid",
				Mode:           Mode80211g,
				Channel:        1,
				HTCaps:         0,
				SecurityConfig: &base.Config{},
				//DTIMPeriod:     254,
			},
			shouldFail: false,
		},
		{
			ops: []Option{
				SSID("ssid"),
				Mode(Mode80211nMixed),
				Channel(1),
				HTCaps(HTCapHT20),
				//DTIMPeriod(1),
			},
			expected: &Config{
				Ssid:           "ssid",
				Mode:           Mode80211nMixed,
				Channel:        1,
				HTCaps:         HTCapHT20,
				SecurityConfig: &base.Config{},
				//DTIMPeriod:     1,
			},
			shouldFail: false,
		},
		{
			ops: []Option{
				SSID("ssid"),
				Mode(Mode80211nPure),
				Channel(36),
				HTCaps(HTCapHT40),
				HTCaps(HTCapSGI20),
				//DTIMPeriod(100),
			},
			expected: &Config{
				Ssid:           "ssid",
				Mode:           Mode80211nPure,
				Channel:        36,
				HTCaps:         HTCapHT40 | HTCapSGI20,
				SecurityConfig: &base.Config{},
				//DTIMPeriod:     100,
			},
			shouldFail: false,
		},
		{
			ops: []Option{
				SSID("ssid"),
				Mode(Mode80211acPure),
				Channel(157),
				HTCaps(HTCapHT40Plus),
				VHTCaps(VHTCapSGI80),
				VHTCenterChannel(155),
				VHTChWidth(VHTChWidth80),
				//DTIMPeriod(50),
			},
			expected: &Config{
				Ssid:             "ssid",
				Mode:             Mode80211acPure,
				Channel:          157,
				HTCaps:           HTCapHT40Plus,
				VHTCaps:          []VHTCap{VHTCapSGI80},
				VHTCenterChannel: 155,
				VHTChWidth:       VHTChWidth80,
				SecurityConfig:   &base.Config{},
				//DTIMPeriod:       50,
			},
			shouldFail: false,
		},
		{
			ops: []Option{
				SSID("ssid"),
				Mode(Mode80211a),
				Channel(36),
				Hidden(),
				//DTIMPeriod(200),
			},
			expected: &Config{
				Ssid:           "ssid",
				Mode:           Mode80211a,
				Channel:        36,
				HTCaps:         0,
				Hidden:         true,
				SecurityConfig: &base.Config{},
				//DTIMPeriod:     200,
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

func parseConfigString(s string) (map[string]string, error) {
	ret := make(map[string]string)
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			continue
		}
		tokens := strings.SplitN(line, "=", 2)
		if len(tokens) != 2 {
			return nil, errors.Errorf("invalid config line: %q", line)
		}
		ret[strings.TrimSpace(tokens[0])] = strings.TrimSpace(tokens[1])
	}
	return ret, nil
}

func TestConfigFormat(t *testing.T) {
	// Test Config.Format.

	// Fixed input for Config.Format call.
	const iface = "tiface0"
	const ctrl = "t.ctrl"

	testcases := []struct {
		conf   *Config
		verify map[string]string // fields to verify
	}{
		// Check basic fields.
		{
			conf: &Config{
				Ssid:           "ssid000",
				Mode:           Mode80211b,
				Channel:        1,
				SecurityConfig: &base.Config{},
			},
			verify: map[string]string{
				"ssid":           "ssid000",
				"hw_mode":        "b",
				"channel":        "1",
				"interface":      iface,
				"ctrl_interface": ctrl,
			},
		},
		// Check 802.11n pure.
		{
			conf: &Config{
				Ssid:           "ssid",
				Mode:           Mode80211nPure,
				Channel:        3,
				SecurityConfig: &base.Config{},
			},
			verify: map[string]string{
				"hw_mode":    "g",
				"channel":    "3",
				"ieee80211n": "1",
				"require_ht": "1",
			},
		},
		// Check ht_capab.
		{
			conf: &Config{
				Ssid:           "ssid",
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
				Ssid:           "ssid",
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
				Ssid:           "ssid",
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
				Ssid:             "ssid",
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
				Ssid:             "ssid",
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
				Ssid:           "ssid",
				Mode:           Mode80211b,
				Channel:        1,
				Hidden:         true,
				SecurityConfig: &base.Config{},
			},
			verify: map[string]string{
				"ignore_broadcast_ssid": "1",
			},
		},
	}

	for i, tc := range testcases {
		cs, err := tc.conf.Format(iface, ctrl)
		if err != nil {
			t.Errorf("testcase %d failed with err=%s", i, err.Error())
			continue
		}
		m, err := parseConfigString(cs)
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
				Ssid:    "ssid000",
				Mode:    Mode80211b,
				Channel: 1,
			},
			expect: []iw.SetFreqOption{iw.SetFreqChWidth(iw.ChWidthNOHT)},
		},
		{
			conf: &Config{
				Ssid:    "ssid",
				Mode:    Mode80211nPure,
				Channel: 3,
			},
			expect: []iw.SetFreqOption{iw.SetFreqChWidth(iw.ChWidthHT20)},
		},
		{
			conf: &Config{
				Ssid:    "ssid",
				Mode:    Mode80211nPure,
				Channel: 1,
				HTCaps:  HTCapHT40,
			},
			expect: []iw.SetFreqOption{iw.SetFreqChWidth(iw.ChWidthHT40Plus)},
		},
		{
			conf: &Config{
				Ssid:    "ssid",
				Mode:    Mode80211nMixed,
				Channel: 5,
				HTCaps:  HTCapHT40 | HTCapSGI40,
			},
			expect: []iw.SetFreqOption{iw.SetFreqChWidth(iw.ChWidthHT40Minus)},
		},
		{
			conf: &Config{
				Ssid:       "ssid",
				Mode:       Mode80211acMixed,
				Channel:    157,
				HTCaps:     HTCapHT40Plus,
				VHTChWidth: VHTChWidth20Or40,
			},
			expect: []iw.SetFreqOption{iw.SetFreqChWidth(iw.ChWidthHT40Plus)},
		},
		{
			conf: &Config{
				Ssid:       "ssid",
				Mode:       Mode80211acMixed,
				Channel:    157,
				HTCaps:     HTCapHT40Plus,
				VHTChWidth: VHTChWidth80,
			},
			expect: []iw.SetFreqOption{iw.SetFreqChWidth(iw.ChWidth80)},
		},
		{
			conf: &Config{
				Ssid:       "ssid",
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
				Ssid:       "ssid",
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
