// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updateserver

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Unittests,
		Desc:         "Verfies that appid and version can be parsed from lsb-release",
		Contacts:     []string{"xiaochu@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"dlc"},
		Data:         []string{lsbRelease},
	})
}

const lsbRelease = "lsb-release.txt"

// Unittests runs RunParseLsbRelease on a fake lsb-release and checks result.
func Unittests(ctx context.Context, s *testing.State) {
	if appid, targetVersion, err := RunParseLsbRelease(s.DataPath(lsbRelease)); err != nil {
		s.Fatal("Failed to parse lsb-release: ", err)
	} else {
		if appid != "{35EF2A87-CD2B-62EE-E83C-F6E0F71C7FEE}" {
			s.Fatal("appid is not expected: ", appid)
		}
		if targetVersion != "11803.0.2019_02_21_1037" {
			s.Fatal("targetVersion is not expected: ", targetVersion)
		}
	}
}
