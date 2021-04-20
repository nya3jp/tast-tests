// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
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
		{"[]", false, []interface{}(nil)},
		{"", true, nil},
		{`['foo', 'bar\'', 'ba\\z']`, false, []interface{}{"foo", "bar'", `ba\z`}},
		{`[['one', 'two'], ['three']]`, false, []interface{}{[]interface{}{"one", "two"}, []interface{}{"three"}}},
	} {
		res, err := ParseStringList(tc.pslParam)
		if tc.expectErr {
			if err == nil {
				t.Errorf("ParseStringList(%q) unexpectedly succeeded", tc.pslParam)
			}
		} else if err != nil {
			t.Errorf("ParseStringList(%q) failed %s", tc.pslParam, err)
		} else if !cmp.Equal(tc.expected, res) {
			t.Errorf("ParseStringList(%q) %s", tc.pslParam, cmp.Diff(tc.expected, res))
		}
	}
}
