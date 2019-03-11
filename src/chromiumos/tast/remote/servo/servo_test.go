// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
			t.Errorf("parseConnSpec(%q) returned unexpected error: %v", tc.input, err)
			return
		}
		if err == nil && tc.expectErr {
			t.Errorf("parseConnSpec(%q) unexpectedly succeeded", tc.input)
			return
		}
		if actualHost != tc.expectedHost {
			t.Errorf("parseConnSpec(%q) returned host %q; want %q", actualHost, tc.expectedHost)
		}
		if actualPort != tc.expectedPort {
			t.Errorf("parseConnSpec(%q) returned port %d; want %d", actualPort, tc.expectedPort)
		}
	}
}
