// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import "testing"

func TestNormalizeEmail(t *testing.T) {
	// Empty out indicates an error is expected.
	for _, tc := range []struct {
		in         string
		removeDots bool
		out        string
	}{
		{"user", true, "user@gmail.com"},
		{"Test.User", true, "testuser@gmail.com"},
		{"Test.User", false, "test.user@gmail.com"},
		{"user@gmail.com", true, "user@gmail.com"},
		{"user@gmail.com", false, "user@gmail.com"},
		{"Test.User@Gmail.com", true, "testuser@gmail.com"},
		{"Test.User@Gmail.com", false, "test.user@gmail.com"},
		{"user@example.com", true, "user@example.com"},
		{"Test.User@Example.com", true, "test.user@example.com"},
		{"@gmail.com", true, "@gmail.com"},
		{"@example.com", true, "@example.com"},
		{"bad@user@example.com", true, "bad@user@example.com"},
	} {
		out, err := NormalizeEmail(tc.in, tc.removeDots)
		if tc.out != "" && err != nil {
			t.Errorf("NormalizeEmail(%q) failed: %v", tc.in, err)
		} else if tc.out != "" && out != tc.out {
			t.Errorf("NormalizeEmail(%q) = %q; want %q", tc.in, out, tc.out)
		} else if tc.out == "" && err == nil {
			t.Errorf("NormalizeEmail(%q) = %q; wanted error", tc.in, out)
		}
	}
}
