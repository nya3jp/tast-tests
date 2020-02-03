// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hostapd

import (
	"strings"
	"testing"

	"chromiumos/tast/errors"
)

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
