// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package secconf

import (
	"reflect"
	"testing"
)

func TestNewWpaConfig(t *testing.T) {
	// Test NewWpaConfig and WpaConfig.validate.
	testcases := []struct {
		ops        []WpaOption
		expected   *WpaConfig
		shouldFail bool
	}{
		{
			ops: []WpaOption{
				WpaPsk("chromeos"),
				WpaMode(WpaPure),
			},
			expected:   nil,
			shouldFail: true, // missing cipher
		},
		{
			ops: []WpaOption{
				WpaPsk("chromeos"),
				WpaMode(Wpa2Pure),
			},
			expected:   nil,
			shouldFail: true, // missing both cipher and cipher2
		},
		{
			ops: []WpaOption{
				WpaPsk("chromeos"),
				WpaMode(WpaPure),
				WpaCiphers(CipherTKIP, CipherCCMP),
				Wpa2Ciphers(CipherCCMP),
			},
			expected:   nil,
			shouldFail: true, // using pure WPA but cipher2 is set
		},
		{
			ops: []WpaOption{
				WpaMode(WpaPure),
				WpaCiphers(CipherTKIP),
			},
			expected:   nil,
			shouldFail: true, // missing PSK
		},
		{
			ops: []WpaOption{
				WpaPsk("01234" +
					"0123456789" +
					"0123456789" +
					"0123456789" +
					"0123456789" +
					"0123456789" +
					"0123456789"),
				WpaMode(WpaPure),
				WpaCiphers(CipherTKIP),
			},
			expected:   nil,
			shouldFail: true, // passphrase with length over 64
		},
		{
			ops: []WpaOption{
				WpaPsk("zzzz" +
					"0123456789" +
					"0123456789" +
					"0123456789" +
					"0123456789" +
					"0123456789" +
					"0123456789"),
				WpaMode(WpaPure),
				WpaCiphers(CipherTKIP),
			},
			expected:   nil,
			shouldFail: true, // passphrase with length 64 but contains non-hex digits
		},
		// Good cases.
		{
			ops: []WpaOption{
				WpaPsk("chromeos"),
				WpaMode(WpaMixed),
				WpaCiphers(CipherTKIP, CipherCCMP),
				Wpa2Ciphers(CipherCCMP),
				WpaFTMode(FTModeNone),
				WpaGmkRekeyPeriod(86400),
				WpaGtkRekeyPeriod(86400),
				WpaPtkRekeyPeriod(600),
				WpaUseStrictRekey(true),
			},
			expected: &WpaConfig{
				Psk:               "chromeos",
				WpaMode:           WpaMixed,
				WpaCiphers:        []Cipher{CipherTKIP, CipherCCMP},
				Wpa2Ciphers:       []Cipher{CipherCCMP},
				FTMode:            FTModeNone,
				WpaGmkRekeyPeriod: 86400,
				WpaGtkRekeyPeriod: 86400,
				WpaPtkRekeyPeriod: 600,
				UseStrictRekey:    true,
			},
			shouldFail: false,
		},
		{
			// 64 byte hex digits is ok.
			ops: []WpaOption{
				WpaPsk("0123" +
					"0123456789" +
					"0123456789" +
					"0123456789" +
					"0123456789" +
					"abcdefaaaa" +
					"ABCDEFAAAA"),
				WpaMode(WpaPure),
				WpaCiphers(CipherTKIP),
			},
			expected: &WpaConfig{
				Psk: "0123" +
					"0123456789" +
					"0123456789" +
					"0123456789" +
					"0123456789" +
					"abcdefaaaa" +
					"ABCDEFAAAA",
				WpaMode:    WpaPure,
				WpaCiphers: []Cipher{CipherTKIP},
				FTMode:     FTModeNone,
			},
			shouldFail: false,
		},
		{
			// Have cipher but don't have cipher2 is ok to WPA2.
			ops: []WpaOption{
				WpaPsk("chromeos"),
				WpaMode(Wpa2Pure),
				WpaCiphers(CipherTKIP),
			},
			expected: &WpaConfig{
				Psk:        "chromeos",
				WpaMode:    Wpa2Pure,
				WpaCiphers: []Cipher{CipherTKIP},
				FTMode:     FTModeNone,
			},
			shouldFail: false,
		},
	}

	for i, tc := range testcases {
		conf, err := NewWpaConfig(tc.ops...)
		if tc.shouldFail {
			if err == nil {
				t.Errorf("testcase %d should not pass config validation", i)
			}
			continue
		}
		if err != nil {
			t.Errorf("testcase %d NewWpaConfig failed with err=%s", i, err.Error())
		}
		if !reflect.DeepEqual(conf, tc.expected) {
			t.Errorf("testcase %d got %v but expect %v", i, conf, tc.expected)
		}
	}
}

func TestWpaConfigGetHostapdConfig(t *testing.T) {
	// Test WpaConfig.GetHostapdConfig.

	testcases := []struct {
		conf   *WpaConfig
		verify map[string]string // fields to verify
	}{
		{
			// All set.
			conf: &WpaConfig{
				Psk:               "chromeos",
				WpaMode:           WpaMixed,
				WpaCiphers:        []Cipher{CipherTKIP, CipherCCMP},
				Wpa2Ciphers:       []Cipher{CipherCCMP},
				FTMode:            FTModeNone,
				WpaGmkRekeyPeriod: 86400,
				WpaGtkRekeyPeriod: 86400,
				WpaPtkRekeyPeriod: 600,
				UseStrictRekey:    true,
			},
			verify: map[string]string{
				"wpa_passphrase":   "chromeos",
				"wpa":              "3",
				"wpa_pairwise":     "TKIP CCMP",
				"rsn_pairwise":     "CCMP",
				"wpa_key_mgmt":     "WPA-PSK",
				"wpa_gmk_rekey":    "86400",
				"wpa_group_rekey":  "86400",
				"wpa_ptk_rekey":    "600",
				"wpa_strict_rekey": "1",
			},
		},
		{
			// Use 64 byte PSK.
			conf: &WpaConfig{
				Psk: "0123" +
					"0123456789" +
					"0123456789" +
					"0123456789" +
					"0123456789" +
					"abcdefaaaa" +
					"ABCDEFAAAA",
				WpaMode:    WpaPure,
				WpaCiphers: []Cipher{CipherTKIP},
				FTMode:     FTModeNone,
			},
			verify: map[string]string{
				"wpa_psk": "0123" +
					"0123456789" +
					"0123456789" +
					"0123456789" +
					"0123456789" +
					"abcdefaaaa" +
					"ABCDEFAAAA",
				"wpa":          "1",
				"wpa_pairwise": "TKIP",
				"wpa_key_mgmt": "WPA-PSK",
			},
		},
		{
			// Mixed FT mode.
			conf: &WpaConfig{
				Psk:        "chromeos",
				WpaMode:    WpaPure,
				WpaCiphers: []Cipher{CipherTKIP},
				FTMode:     FTModeMixed,
			},
			verify: map[string]string{
				"wpa_passphrase": "chromeos",
				"wpa":            "1",
				"wpa_pairwise":   "TKIP",
				"wpa_key_mgmt":   "WPA-PSK FT-PSK",
			},
		},
	}

	for i, tc := range testcases {
		m, err := tc.conf.GetHostapdConfig()
		if err != nil {
			t.Errorf("testcase %d failed with err=%s", i, err.Error())
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
