// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updateserver

import (
	"testing"
)

// TestParseLSBRelease runs parseLSBRelease on a fake lsb-release and checks result.
func TestParseLSBRelease(t *testing.T) {
	if appID, relVersion, err := parseLSBRelease("testdata/lsb-release.txt"); err != nil {
		t.Error("Failed to parse lsb-release: ", err)
	} else {
		if exp := "{35EF2A87-CD2B-62EE-E83C-F6E0F71C7FEE}"; appID != exp {
			t.Errorf("parseLSBRelease returned appID %q; want %q", appID, exp)
		}
		if exp := "11803.0.2019_02_21_1037"; relVersion != exp {
			t.Errorf("parseLSBRelease returned relVersion %q; want %q", relVersion, exp)
		}
	}
}
