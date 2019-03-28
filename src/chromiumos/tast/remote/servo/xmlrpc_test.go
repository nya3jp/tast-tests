// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"testing"
)

func TestXMLBooleanToBool(t *testing.T) {
	for _, tc := range []struct {
		input     string
		expected  bool
		expectErr bool
	}{
		{"1", true, false},
		{"0", false, false},
		{"rutabaga", false, true},
	} {
		actual, err := xmlBooleanToBool(tc.input)
		if tc.expectErr {
			if err != nil {
				t.Errorf("xmlBooleanToBool(%q) unexpectedly succeeded", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("xmlBooleanToBool(%q) failed: %v", tc.input, err)
			}
			if actual != tc.expected {
				t.Errorf("xmlBooleanToBool(%q) = %q; want %q", tc.input, actual, tc.expected)
			}
		}
	}
}

func TestBoolToXMLBoolean(t *testing.T) {
	for _, tc := range []struct {
		input    bool
		expected string
	}{
		{true, "1"},
		{false, "0"},
	} {
		actual := boolToXMLBoolean(tc.input)
		if actual != tc.expected {
			t.Errorf("boolToXMLBoolean(%q) = %q; want %q", tc.input, actual, tc.expected)
		}
	}
}
func TestNewValue(t *testing.T) {
	expectedStr := "rutabaga"
	v, err := newValue(expectedStr)
	if err != nil {
		t.Errorf("newValue(%q) failed: %v", expectedStr, err)
		return
	}
	if v.String != expectedStr {
		t.Errorf("got %q; want %q", v.String, expectedStr)
	}

	expectedBool := true
	expectedBoolStr := "1"
	v, err = newValue(expectedBool)
	if err != nil {
		t.Errorf("input %v gave unexpected error: %v", expectedStr, err)
		return
	}
	if v.Boolean != expectedBoolStr {
		t.Errorf("got %q; want %q", v.Boolean, expectedBoolStr)
	}

	expectedUnsupported := 1234
	v, err = newValue(expectedUnsupported)
	if err == nil {
		t.Errorf("input %v did not throw expected error", expectedUnsupported)
	}
}

func TestNewParams(t *testing.T) {
	actual, err := newParams([]interface{}{"rutabaga", true})

	if err != nil {
		t.Errorf("got unexpected error: %v", err)
		return
	}
	if len(actual) != 2 {
		t.Errorf("got len %d; want %d", len(actual), 3)
	}
	if actual[0].Value.String != "rutabaga" {
		t.Errorf("for first return value got %q; want %q", actual[0].Value.String, "rutabaga")
	}
	if actual[1].Value.Boolean != "1" {
		t.Errorf("for second return value got %q; want %q", actual[1].Value.Boolean, "1")
	}
}
