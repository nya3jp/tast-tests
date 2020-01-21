// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package secconf

import (
	"reflect"
	"testing"
)

func TestNewWepConfig(t *testing.T) {
	// Test NewWepConfig and WepConfig.validate.
	testcases := []struct {
		ops        []WepOption
		expected   *WepConfig
		shouldFail bool
	}{
		{
			ops: []WepOption{
				WepKeys([]string{"abcde", "abcde", "abcde", "abcde", "abcde"}),
			},
			expected:   nil,
			shouldFail: true, // number of keys cannot be more than 4
		},
		{
			ops: []WepOption{
				WepKeys([]string{"abcde", "abcde"}),
				WepDefaultKey(2),
			},
			expected:   nil,
			shouldFail: true, // default key out of range
		},
		{
			ops: []WepOption{
				WepKeys([]string{"abcde"}),
				WepAuthAlgs(WepAuthAlgsEnum(0)),
			},
			expected:   nil,
			shouldFail: true, // no authentication algorithms is set
		},
		{
			ops: []WepOption{
				WepKeys([]string{"abcde"}),
				WepAuthAlgs(WepAuthAlgsEnum(5)),
			},
			expected:   nil,
			shouldFail: true, // invalid authentication algorithms is set
		},
		{
			ops: []WepOption{
				WepKeys([]string{"abcdef"}),
			},
			expected:   nil,
			shouldFail: true, // invalid key length
		},
		{
			ops: []WepOption{
				WepKeys([]string{"abcdefghij"}),
			},
			expected:   nil,
			shouldFail: true, // hex passphrase contains non-hex digits
		},
		// Good case.
		{
			ops: []WepOption{
				WepKeys([]string{"abcde", "abcde01234", "abcdefghijklm", "0123456789abcdefABCDEF0123"}),
				WepDefaultKey(2),
				WepAuthAlgs(WepAuthAlgsShared),
			},
			expected: &WepConfig{
				Keys:       []string{"abcde", "abcde01234", "abcdefghijklm", "0123456789abcdefABCDEF0123"},
				DefaultKey: 2,
				AuthAlgs:   WepAuthAlgsShared,
			},
			shouldFail: false,
		},
	}

	for i, tc := range testcases {
		conf, err := NewWepConfig(tc.ops...)
		if tc.shouldFail {
			if err == nil {
				t.Errorf("testcase %d should not pass config validation", i)
			}
			continue
		}
		if err != nil {
			t.Errorf("testcase %d NewWepConfig failed with err=%s", i, err.Error())
		}
		if !reflect.DeepEqual(conf, tc.expected) {
			t.Errorf("testcase %d got %v but expect %v", i, conf, tc.expected)
		}
	}
}

func TestWepConfigGetHostapdConfig(t *testing.T) {
	// Test WepConfig.GetHostapdConfig.

	testcases := []struct {
		conf   *WepConfig
		verify map[string]string // fields to verify
	}{
		{
			// All set.
			conf: &WepConfig{
				Keys:       []string{"abcde", "abcde01234", "abcdefghijklm", "0123456789abcdefABCDEF0123"},
				DefaultKey: 2,
				AuthAlgs:   WepAuthAlgsOpen,
			},
			verify: map[string]string{
				"wep_key0":        "\"abcde\"",
				"wep_key1":        "abcde01234",
				"wep_key2":        "\"abcdefghijklm\"",
				"wep_key3":        "0123456789abcdefABCDEF0123",
				"wep_default_key": "2",
				"auth_algs":       "1",
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
