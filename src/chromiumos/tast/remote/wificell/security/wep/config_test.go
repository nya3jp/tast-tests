// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wep

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
				[]string{"abcde", "abcde", "abcde", "abcde", "abcde"},
			),
			expected:   nil,
			shouldFail: true, // number of keys cannot be more than 4
		}, {
			factory: NewFactory(
				[]string{"abcde", "abcde"},
				DefaultKey(2),
			),
			expected:   nil,
			shouldFail: true, // default key out of range
		}, {
			factory: NewFactory(
				[]string{"abcde"},
				AuthAlgs(AuthAlgsEnum(0)),
			),
			expected:   nil,
			shouldFail: true, // no authentication algorithms is set
		}, {
			factory: NewFactory(
				[]string{"abcde"},
				AuthAlgs(AuthAlgsEnum(5)),
			),
			expected:   nil,
			shouldFail: true, // invalid authentication algorithms is set
		}, {
			factory: NewFactory(
				[]string{"abcdef"},
			),
			expected:   nil,
			shouldFail: true, // invalid key length
		}, {
			factory: NewFactory(
				[]string{"abcdefghij"},
			),
			expected:   nil,
			shouldFail: true, // hex passphrase contains non-hex digits
		}, { // Good case.
			factory: NewFactory(
				[]string{"abcde", "abcde01234", "abcdefghijklm", "0123456789abcdefABCDEF0123"},
				DefaultKey(2),
				AuthAlgs(AuthAlgsShared),
			),
			expected: &Config{
				Keys:       []string{"abcde", "abcde01234", "abcdefghijklm", "0123456789abcdefABCDEF0123"},
				DefaultKey: 2,
				AuthAlgs:   AuthAlgsShared,
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
				Keys:       []string{"abcde", "abcde01234", "abcdefghijklm", "0123456789abcdefABCDEF0123"},
				DefaultKey: 2,
				AuthAlgs:   AuthAlgsOpen,
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
	})
}
