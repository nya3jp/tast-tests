// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hostapd

import (
	"reflect"
	"strings"
	"testing"

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
			shouldFail: true,
		},
		// Check channel validation.
		{
			ops: []Option{
				Mode(Mode80211g),
			},
			expected:   nil,
			shouldFail: true,
		},
		{
			ops: []Option{
				Mode(Mode80211a),
				Channel(1),
			},
			expected:   nil,
			shouldFail: true,
		},
		{
			ops: []Option{
				Mode(Mode80211g),
				Channel(36),
			},
			expected:   nil,
			shouldFail: true,
		},
		// Check HTCap validation.
		{
			ops: []Option{
				Mode(Mode80211g),
				Channel(1),
				HTCaps(HTCapHT40),
			},
			expected:   nil,
			shouldFail: true,
		},
		{
			ops: []Option{
				Mode(Mode80211nPure),
				Channel(1),
			},
			expected:   nil,
			shouldFail: true,
		},
		{
			ops: []Option{
				Mode(Mode80211nPure),
				Channel(1),
				HTCaps(HTCapHT40Minus),
			},
			expected:   nil,
			shouldFail: true,
		},
		{
			ops: []Option{
				Mode(Mode80211nMixed),
				Channel(161),
				HTCaps(HTCapHT40Plus),
			},
			expected:   nil,
			shouldFail: true,
		},
		// Good cases.
		{
			ops: []Option{
				SSID("ssid"),
				Mode(Mode80211a),
				Channel(36),
			},
			expected: &Config{
				Ssid:    "ssid",
				Mode:    Mode80211a,
				Channel: 36,
				HTCaps:  0,
			},
			shouldFail: false,
		},
		{
			ops: []Option{
				SSID("ssid"),
				Mode(Mode80211g),
				Channel(1),
			},
			expected: &Config{
				Ssid:    "ssid",
				Mode:    Mode80211g,
				Channel: 1,
				HTCaps:  0,
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
			},
			expected: &Config{
				Ssid:    "ssid",
				Mode:    Mode80211nPure,
				Channel: 36,
				HTCaps:  HTCapHT40 | HTCapSGI20,
			},
			shouldFail: false,
		},
	}

	for i, tc := range testcases {
		conf, err := NewConfig(tc.ops...)
		if tc.shouldFail {
			if err == nil {
				t.Errorf("testcase %d succeeded but should fail", i)
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
				Ssid:    "ssid000",
				Mode:    Mode80211b,
				Channel: 1,
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
				Ssid:    "ssid",
				Mode:    Mode80211nPure,
				Channel: 3,
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
				Ssid:    "ssid",
				Mode:    Mode80211nMixed,
				Channel: 36,
				HTCaps:  HTCapHT40 | HTCapSGI20 | HTCapSGI40,
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
				Ssid:    "ssid",
				Mode:    Mode80211nMixed,
				Channel: 5,
				HTCaps:  HTCapHT40 | HTCapSGI40,
			},
			verify: map[string]string{
				"hw_mode":    "g",
				"channel":    "5",
				"ieee80211n": "1",
				"ht_capab":   "[HT40-][SHORT-GI-40]",
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
