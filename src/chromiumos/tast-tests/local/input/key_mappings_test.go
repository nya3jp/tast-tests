// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"reflect"
	"testing"
)

func TestParseAccel(t *testing.T) {
	for _, tc := range []struct {
		accel string      // input string
		keys  []EventCode // expected keys or nil for error
	}{
		{"Ctrl", []EventCode{KEY_LEFTCTRL}},
		{"A", []EventCode{KEY_A}},
		{"Ctrl+T", []EventCode{KEY_LEFTCTRL, KEY_T}},
		{"Ctrl+Shift+T", []EventCode{KEY_LEFTCTRL, KEY_LEFTSHIFT, KEY_T}},
		{"Shift+Space+Enter+Backspace+Tab", []EventCode{KEY_LEFTSHIFT, KEY_SPACE, KEY_ENTER, KEY_BACKSPACE, KEY_TAB}},
		{"alt+b", []EventCode{KEY_LEFTALT, KEY_B}},
		{"Ctrl+Bogus", nil},
		{"", nil},
	} {
		keys, err := parseAccel(tc.accel)
		if err != nil {
			if tc.keys != nil {
				t.Errorf("parseAccel(%q) returned error: %v", tc.accel, err)
			}
			continue
		}

		if tc.keys == nil {
			t.Errorf("parseAccel(%q) returned %v rather than expected error", tc.accel, keys)
		} else if !reflect.DeepEqual(keys, tc.keys) {
			t.Errorf("parseAccel(%q) = %v; want %v", tc.accel, keys, tc.keys)
		}
	}
}
