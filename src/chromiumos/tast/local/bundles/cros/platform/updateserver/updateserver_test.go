// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updateserver

import (
	"testing"
)

// TestParseLsbRelease runs parseLsbRelease on a fake lsb-release and checks result.
func TestParseLsbRelease(t *testing.T) {
	if appid, targetVersion, err := parseLSBRelease("testdata/lsb-release.txt"); err != nil {
		t.Fatal("Failed to parse lsb-release: ", err)
	} else {
		if appid != "{35EF2A87-CD2B-62EE-E83C-F6E0F71C7FEE}" {
			t.Fatal("appid is not expected: ", appid)
		}
		if targetVersion != "11803.0.2019_02_21_1037" {
			t.Fatal("targetVersion is not expected: ", targetVersion)
		}
	}
}
