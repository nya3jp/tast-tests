// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"reflect"
	"testing"
)

// deepEquals is like reflect.DeepEqual, except that it works for []interface{}
func deepEquals(t *testing.T, expected, actual interface{}) bool {
	switch expected.(type) {
	case []interface{}:
		eVal := reflect.ValueOf(expected)
		aVal := reflect.ValueOf(actual)
		same := true
		for i := 0; i < eVal.Len(); i++ {
			same = same && deepEquals(t, eVal.Index(i).Interface(), aVal.Index(i).Interface())
		}
		return same
	case string:
		return expected.(string) == actual.(string)
	default:
		t.Errorf("Unhandled type %T of %v", expected, expected)
		return false
	}
}

func TestParseStringList(t *testing.T) {
	type testCase struct {
		pslParam  string
		expectErr bool
		expected  interface{}
	}
	for _, tc := range []testCase{
		{"[]", false, []interface{}{}},
		{"", true, []interface{}{}},
		{`['foo', 'bar\'', 'ba\\z']`, false, []interface{}{"foo", "bar'", `ba\z`}},
		{`[['one', 'two'], ['three']]`, false, []interface{}{[]interface{}{"one", "two"}, []interface{}{"three"}}},
	} {
		res, err := ParseStringList(tc.pslParam)
		if err == nil && tc.expectErr {
			t.Errorf("ParseStringList(%q) unexpectedly succeeded", tc.pslParam)
		} else if err != nil && !tc.expectErr {
			t.Errorf("ParseStringList(%q) failed %s", tc.pslParam, err)
		} else if !deepEquals(t, tc.expected, res) {
			t.Errorf("Expected %q but got %q", tc.expected, res)
		}
	}
}
