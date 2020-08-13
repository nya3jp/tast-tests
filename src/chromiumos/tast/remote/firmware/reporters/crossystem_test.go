// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package reporters

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseCrossystemOutput(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
		res   map[CrossystemParam]string
		err   bool
	}{
		{
			name: "Sanity",
			lines: []string{"kernkey_vfy             = sig                            # [RO/str] Type of verification done on kernel keyblock",
				"mainfw_type             = normal                         # [RO/str] Active main firmware type"},
			res: map[CrossystemParam]string{CrossystemParamKernkeyVfy: "sig", CrossystemParamMainfwType: "normal"},
			err: false,
		},
		{
			name: "Known and unknown keys",
			lines: []string{"kernkey_vfy             = sig                            # [RO/str] Type of verification done on kernel keyblock",
				"new_key                 = abc                            # [RO/str] Some new key"},
			res: map[CrossystemParam]string{CrossystemParamKernkeyVfy: "sig"},
			err: false,
		},
		{
			name: "Duplicate keys",
			lines: []string{"kernkey_vfy             = sig                            # [RO/str] Type of verification done on kernel keyblock",
				"mainfw_type             = normal                         # [RO/str] Active main firmware type",
				"new_key                 = abc                            # [RO/str] Some new key",
				"new_key                 = def                            # [RO/str] Some new key"},

			res: nil,
			err: true,
		},
	} {
		res, err := parseCrossystemOutput(tc.lines)
		if tc.err && err == nil {
			t.Errorf("%v: wanted err != nil", tc.name)
		}
		if !tc.err && err != nil {
			t.Errorf("%v: wanted err == nil, got %v", tc.name, err)
		}
		if diff := cmp.Diff(tc.res, res); diff != "" {
			t.Errorf("%v result different from expected: %v", tc.name, diff)
		}
	}
}
