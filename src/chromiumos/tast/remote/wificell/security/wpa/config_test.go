// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpa

import (
	"reflect"
	"testing"

	"chromiumos/tast/remote/wificell/security"
)

type testGenStruct struct {
	factory    security.Factory
	expected   security.Config
	shouldFail bool
}

func runTestGen(t *testing.T, testcases []testGenStruct) {
	for i, tc := range testcases {
		conf, err := tc.factory.Gen()
		if tc.shouldFail {
			if err == nil {
				t.Errorf("testcase %d should not pass config validation", i)
			}
			continue
		}
		if err != nil {
			t.Errorf("testcase %d Gen failed with err=%s", i, err.Error())
		}
		if !reflect.DeepEqual(conf, tc.expected) {
			t.Errorf("testcase %d got %v but expect %v", i, conf, tc.expected)
		}
	}
}

type testGetStruct struct {
	conf          security.Config
	verifyHostapd map[string]string      // hostapd config fields to verify
	verifyShill   map[string]interface{} // shill config fields to verify
}

func runTestGet(t *testing.T, testcases []testGetStruct) {
	for i, tc := range testcases {
		// Verify the requested hostapd fields.
		h, err := tc.conf.HostapdConfig()
		if err != nil {
			t.Errorf("testcase %d failed with err=%s", i, err.Error())
			continue
		}
		for k, v := range tc.verifyHostapd {
			if v != h[k] {
				t.Errorf("testcase %d has %q=%q, expect %q", i, k, h[k], v)
			}
		}

		// Verify the requested shill fields.
		s := tc.conf.ShillServiceProperties()
		for k, v := range tc.verifyShill {
			if !reflect.DeepEqual(v, s[k]) {
				t.Errorf("testcase %d has %q=%q, expect %q", i, k, s[k], v)
			}
		}
	}
}

func TestGen(t *testing.T) {
	// Test Factory.Gen and Config.validate.
	runTestGen(t, []testGenStruct{
		{
			factory: NewFactory(
				"chromeos",
				Mode(ModePure),
			),
			expected:   nil,
			shouldFail: true, // missing cipher
		}, {
			factory: NewFactory(
				"chromeos",
				Mode(ModePure2),
			),
			expected:   nil,
			shouldFail: true, // missing both cipher and cipher2
		}, {
			factory: NewFactory(
				"chromeos",
				Mode(ModePure),
				Ciphers(CipherTKIP, CipherCCMP),
				Ciphers2(CipherCCMP),
			),
			expected:   nil,
			shouldFail: true, // using pure WPA but cipher2 is set
		}, {
			factory: NewFactory(
				"01234"+
					"0123456789"+
					"0123456789"+
					"0123456789"+
					"0123456789"+
					"0123456789"+
					"0123456789",
				Mode(ModePure),
				Ciphers(CipherTKIP),
			),
			expected:   nil,
			shouldFail: true, // passphrase with length over 64
		}, {
			factory: NewFactory(
				"zzzz"+
					"0123456789"+
					"0123456789"+
					"0123456789"+
					"0123456789"+
					"0123456789"+
					"0123456789",
				Mode(ModePure),
				Ciphers(CipherTKIP),
			),
			expected:   nil,
			shouldFail: true, // passphrase with length 64 but contains non-hex digits
		}, { // Good cases.
			factory: NewFactory(
				"chromeos",
				Mode(ModeMixed),
				Ciphers(CipherTKIP, CipherCCMP),
				Ciphers2(CipherCCMP),
				FTMode(FTModeNone),
				GmkRekeyPeriod(86400),
				GtkRekeyPeriod(86400),
				PtkRekeyPeriod(600),
				UseStrictRekey(true),
			),
			expected: &Config{
				Psk:            "chromeos",
				Mode:           ModeMixed,
				Ciphers:        []Cipher{CipherTKIP, CipherCCMP},
				Ciphers2:       []Cipher{CipherCCMP},
				FTMode:         FTModeNone,
				GmkRekeyPeriod: 86400,
				GtkRekeyPeriod: 86400,
				PtkRekeyPeriod: 600,
				UseStrictRekey: true,
			},
			shouldFail: false,
		}, {
			// 64 byte hex digits is ok.
			factory: NewFactory(
				"0123"+
					"0123456789"+
					"0123456789"+
					"0123456789"+
					"0123456789"+
					"abcdefaaaa"+
					"ABCDEFAAAA",
				Mode(ModePure),
				Ciphers(CipherTKIP),
			),
			expected: &Config{
				Psk: "0123" +
					"0123456789" +
					"0123456789" +
					"0123456789" +
					"0123456789" +
					"abcdefaaaa" +
					"ABCDEFAAAA",
				Mode:    ModePure,
				Ciphers: []Cipher{CipherTKIP},
				FTMode:  FTModeNone,
			},
			shouldFail: false,
		}, {
			// Have cipher but don't have cipher2 is ok to WPA2.
			factory: NewFactory(
				"chromeos",
				Mode(ModePure2),
				Ciphers(CipherTKIP),
			),
			expected: &Config{
				Psk:     "chromeos",
				Mode:    ModePure2,
				Ciphers: []Cipher{CipherTKIP},
				FTMode:  FTModeNone,
			},
			shouldFail: false,
		},
	})
}

func TestGet(t *testing.T) {
	// Test Config.HostapdConfig and Config.ShillServiceProperties.
	runTestGet(t, []testGetStruct{
		{
			// All set.
			conf: &Config{
				Psk:            "chromeos",
				Mode:           ModeMixed,
				Ciphers:        []Cipher{CipherTKIP, CipherCCMP},
				Ciphers2:       []Cipher{CipherCCMP},
				FTMode:         FTModeNone,
				GmkRekeyPeriod: 86400,
				GtkRekeyPeriod: 86400,
				PtkRekeyPeriod: 600,
				UseStrictRekey: true,
			},
			verifyHostapd: map[string]string{
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
			verifyShill: map[string]interface{}{
				"Passphrase": "chromeos",
			},
		}, {
			// Use 64 byte PSK.
			conf: &Config{
				Psk: "0123" +
					"0123456789" +
					"0123456789" +
					"0123456789" +
					"0123456789" +
					"abcdefaaaa" +
					"ABCDEFAAAA",
				Mode:    ModePure,
				Ciphers: []Cipher{CipherTKIP},
				FTMode:  FTModeNone,
			},
			verifyHostapd: map[string]string{
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
			verifyShill: map[string]interface{}{
				"Passphrase": "0123" +
					"0123456789" +
					"0123456789" +
					"0123456789" +
					"0123456789" +
					"abcdefaaaa" +
					"ABCDEFAAAA",
			},
		}, {
			// Mixed FT mode.
			conf: &Config{
				Psk:     "chromeos",
				Mode:    ModePure,
				Ciphers: []Cipher{CipherTKIP},
				FTMode:  FTModeMixed,
			},
			verifyHostapd: map[string]string{
				"wpa_passphrase": "chromeos",
				"wpa":            "1",
				"wpa_pairwise":   "TKIP",
				"wpa_key_mgmt":   "WPA-PSK FT-PSK",
			},
			verifyShill: map[string]interface{}{
				"Passphrase":     "chromeos",
				"WiFi.FTEnabled": true,
			},
		},
	})
}
