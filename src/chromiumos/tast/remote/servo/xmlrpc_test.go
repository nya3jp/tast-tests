// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"testing"
)

func TestxmlBooleanToBoolean(t *testing.T) {
	for _, tc := range []struct {
		input     string
		expected  bool
		expectErr bool
	}{
		{"1", true, false},
		{"0", false, false},
		{"rutabaga", false, true},
	} {
		actual, err := xmlBooleanToBoolean(tc.input)
		if err != nil && !tc.expectErr {
			t.Errorf("input %v gave unexpected error: %v", tc.input, err)
			return
		}
		if actual != tc.expected {
			t.Errorf("got %v; want %v", actual, tc.expected)
		}
	}
}

func TestbooleanToXMLBoolean(t *testing.T) {
	for _, tc := range []struct {
		input    bool
		expected string
	}{
		{true, "1"},
		{false, "0"},
	} {
		actual := booleanToXMLBoolean(tc.input)
		if actual != tc.expected {
			t.Errorf("got %v; want %v", actual, tc.expected)
		}
	}
}
func TestmakeValue(t *testing.T) {
	expectedStr := "rutabaga"
	v, err := makeValue(expectedStr)
	if err != nil {
		t.Errorf("input %v gave unexpected error: %v", expectedStr, err)
		return
	}
	if v.String != expectedStr {
		t.Errorf("got %v; want %v", v.String, expectedStr)
	}

	expectedBool := true
	expectedBoolStr := "1"
	v, err = makeValue(expectedBool)
	if err != nil {
		t.Errorf("input %v gave unexpected error: %v", expectedStr, err)
		return
	}
	if v.Boolean != expectedBoolStr {
		t.Errorf("got %v; want %v", v.Boolean, expectedBoolStr)
	}

	expectedUnsupported := 1234
	v, err = makeValue(expectedUnsupported)
	if err == nil {
		t.Errorf("input %v did not throw expected error", expectedUnsupported)
	}
}

func TestmakeParams(t *testing.T) {
	actual, err := makeParams([]interface{}{"rutabaga", true})

	if err != nil {
		t.Errorf("got unexpected error: %v", err)
		return
	}
	if len(actual) != 2 {
		t.Errorf("got len %d; want %d", len(actual), 3)
	}
	if actual[0].Value.String != "rutabaga" {
		t.Errorf("for first return value got %s; want %s", actual[0].Value.String, "rutabaga")
	}
	if actual[1].Value.Boolean != "1" {
		t.Errorf("for second return value got %s; want %s", actual[1].Value.Boolean, "1")
	}
}
