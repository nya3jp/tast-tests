// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"reflect"
	"testing"
)

func TestParseConnSpec(t *testing.T) {
	for _, tc := range []struct {
		input        string
		expectedHost string
		expectedPort int
		expectErr    bool
	}{
		{"", "", 0, true},
		{"localhost", "localhost", 9999, false},
		{"localhost:1234", "localhost", 1234, false},
		{"rutabaga:localhost:1234", "", 0, true},
	} {
		actualHost, actualPort, err := parseConnSpec(tc.input)
		if err != nil && !tc.expectErr {
			t.Errorf("parseConnSpec(%q) returned unexpected error: %v", tc.input, err)
			return
		}
		if err == nil && tc.expectErr {
			t.Errorf("parseConnSpec(%q) unexpectedly succeeded", tc.input)
			return
		}
		if actualHost != tc.expectedHost {
			t.Errorf("parseConnSpec(%q) returned host %q; want %q", tc.input, actualHost, tc.expectedHost)
		}
		if actualPort != tc.expectedPort {
			t.Errorf("parseConnSpec(%q) returned port %d; want %d", tc.input, actualPort, tc.expectedPort)
		}
	}
}

func AssertEquals(t *testing.T, expected, actual interface{}) {
	switch expected.(type) {
	case []interface{}:
		eVal := reflect.ValueOf(expected)
		aVal := reflect.ValueOf(actual)
		for i := 0; i < eVal.Len(); i++ {
			AssertEquals(t, eVal.Index(i).Interface(), aVal.Index(i).Interface())
		}
	case string:
		if expected.(string) != actual.(string) {
			t.Errorf("String %q != %q", expected, actual)
		}
	default:
		t.Errorf("Unhandled type %T of %v", expected, expected)
	}
}

func TestParseStringList(t *testing.T) {
	var empty []interface{}
	res, err := ParseStringList("[]")
	if err != nil {
		t.Error(err)
	}
	AssertEquals(t, empty, res)
	res, err = ParseStringList("")
	if err == nil {
		t.Error("ParseStringList(\"\") unexpectedly succeeded")
	}
	res, err = ParseStringList(`['foo', 'bar\'', 'ba\\z']`)
	AssertEquals(t, []interface{}{"foo", "bar'", `ba\z`}, res)
	res, err = ParseStringList(`[['one', 'two'], ['three']]`)
	AssertEquals(t, []interface{}{[]interface{}{"one", "two"}, []interface{}{"three"}}, res)
}
