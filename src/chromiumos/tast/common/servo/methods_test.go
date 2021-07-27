// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

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
