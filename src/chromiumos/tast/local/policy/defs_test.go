// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"encoding/json"
	"testing"
)

func TestBoolCompare(t *testing.T) {
	tcs := []struct {
		m      json.RawMessage
		v      bool
		result bool
	}{{json.RawMessage("True"), true, true},
		{json.RawMessage("False"), false, true},
		{json.RawMessage("True"), false, false},
		{json.RawMessage(""), false, false}}

	for _, tc := range tcs {
		bp := AllowDinosaurEasterEgg{Val: tc.v}
		cmp, err := bp.Compare(tc.m)
		if err != nil {
			t.Errorf("error comparing %v and %v: %s", tc.v, tc.m, err)
		}
		if cmp != tc.result {
			t.Errorf("unexpected comparison: got %s, expected %s", cmp, tc.result)
		}
	}
}

// TBD more tests here
