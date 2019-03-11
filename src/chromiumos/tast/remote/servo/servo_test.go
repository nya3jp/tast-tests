// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
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
			t.Errorf("input %v gave unexpected error: %v", tc.input, err)
			return
		}
		if err == nil && tc.expectErr {
			t.Errorf("input %v did not throw expected error: %v", tc.input, err)
			return
		}
		if actualHost != tc.expectedHost {
			t.Errorf("got %v; want %v", actualHost, tc.expectedHost)
		}
		if actualPort != tc.expectedPort {
			t.Errorf("got %v; want %v", actualPort, tc.expectedPort)
		}
	}
}
