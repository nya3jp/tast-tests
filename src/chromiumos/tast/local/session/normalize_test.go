// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import "testing"

func TestNormalizeEmail(t *testing.T) {
	// Empty out indicates an error is expected.
	for _, tc := range []struct{ in, out string }{
		{"user", "user@gmail.com"},
		{"Test.User", "testuser@gmail.com"},
		{"user@gmail.com", "user@gmail.com"},
		{"Test.User@Gmail.com", "testuser@gmail.com"},
		{"user@example.com", "user@example.com"},
		{"Test.User@Example.com", "test.user@example.com"},
		{"", ""},
		{"@gmail.com", ""},
		{"@example.com", ""},
		{"bad@user@example.com", ""},
	} {
		out, err := NormalizeEmail(tc.in)
		if tc.out != "" && err != nil {
			t.Errorf("NormalizeEmail(%q) failed: %v", tc.in, err)
		} else if tc.out != "" && out != tc.out {
			t.Errorf("NormalizeEmail(%q) = %q; want %q", tc.in, out, tc.out)
		} else if tc.out == "" && err == nil {
			t.Errorf("NormalizeEmail(%q) = %q; wanted error", tc.in, out)
		}
	}
}
