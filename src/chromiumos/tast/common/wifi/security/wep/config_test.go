// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wep

import (
	"reflect"
	"testing"

	"chromiumos/tast/common/wifi/security"
)

func TestGen(t *testing.T) {
	// Test ConfigFactory.Gen and Config.validate.
	for i, tc := range []struct {
		factory    security.ConfigFactory
		expected   security.Config
		shouldFail bool
	}{
		{
			factory: NewConfigFactory(
				[]string{"abcde", "abcde", "abcde", "abcde", "abcde"},
			),
			expected:   nil,
			shouldFail: true, // number of keys cannot be more than 4
		}, {
			factory: NewConfigFactory(
				[]string{"abcde", "abcde"},
				DefaultKey(2),
			),
			expected:   nil,
			shouldFail: true, // default key out of range
		}, {
			factory: NewConfigFactory(
				[]string{"abcde"},
				AuthAlgs(AuthAlgo(0)),
			),
			expected:   nil,
			shouldFail: true, // no authentication algorithms is set
		}, {
			factory: NewConfigFactory(
				[]string{"abcde"},
				AuthAlgs(AuthAlgo(5)),
			),
			expected:   nil,
			shouldFail: true, // invalid authentication algorithms is set
		}, {
			factory: NewConfigFactory(
				[]string{"abcdef"},
			),
			expected:   nil,
			shouldFail: true, // invalid key length
		}, {
			factory: NewConfigFactory(
				[]string{"abcdefghij"},
			),
			expected:   nil,
			shouldFail: true, // hex passphrase contains non-hex digits
		}, { // Good case.
			factory: NewConfigFactory(
				[]string{"abcde", "abcde01234", "abcdefghijklm", "0123456789abcdefABCDEF0123"},
				DefaultKey(2),
				AuthAlgs(AuthAlgoShared),
			),
			expected: &Config{
				keys:       []string{"abcde", "abcde01234", "abcdefghijklm", "0123456789abcdefABCDEF0123"},
				defaultKey: 2,
				authAlgs:   AuthAlgoShared,
			},
			shouldFail: false,
		},
	} {
		conf, err := tc.factory.Gen()
		if tc.shouldFail {
			if err == nil {
				t.Errorf("testcase %d should not pass Config validation", i)
			}
			continue
		}
		if err != nil {
			t.Errorf("testcase %d Gen failed: %s", i, err)
		}
		if !reflect.DeepEqual(conf, tc.expected) {
			t.Errorf("testcase %d got %v, want %v", i, conf, tc.expected)
		}
	}
}

func TestGet(t *testing.T) {
	// Test Config.HostapdConfig and Config.ShillServiceProperties.
	for i, tc := range []struct {
		conf          security.Config
		verifyHostapd map[string]string      // hostapd config fields to verify
		verifyShill   map[string]interface{} // shill config fields to verify
	}{
		{
			// All set.
			conf: &Config{
				keys:       []string{"abcde", "abcde01234", "abcdefghijklm", "0123456789abcdefABCDEF0123"},
				defaultKey: 2,
				authAlgs:   AuthAlgoOpen,
			},
			verifyHostapd: map[string]string{
				"wep_key0":        "\"abcde\"",
				"wep_key1":        "abcde01234",
				"wep_key2":        "\"abcdefghijklm\"",
				"wep_key3":        "0123456789abcdefABCDEF0123",
				"wep_default_key": "2",
				"auth_algs":       "1",
			},
			verifyShill: map[string]interface{}{
				"Passphrase": "2:abcdefghijklm",
			},
		},
	} {
		// Verify the requested hostapd fields.
		h, err := tc.conf.HostapdConfig()
		if err != nil {
			t.Errorf("testcase %d HostapdConfig failed: %s", i, err)
			continue
		}
		if !reflect.DeepEqual(h, tc.verifyHostapd) {
			t.Errorf("testcase %d HostapdConfig got %v, want %v", i, h, tc.verifyHostapd)
		}

		// Verify the requested shill fields.
		s, err := tc.conf.ShillServiceProperties()
		if err != nil {
			t.Errorf("testcase %d ShillServiceProperties failed: %s", i, err)
			continue
		}
		if !reflect.DeepEqual(s, tc.verifyShill) {
			t.Errorf("testcase %d ShillServiceProperties got %v, want %v", i, s, tc.verifyShill)
		}
	}
}
