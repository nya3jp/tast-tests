// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/storage"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MyFiles,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks whether the MyFiles directory is properly shared from Chrome OS to ARC",
		Contacts: []string{
			"youkichihosoi@chromium.org", "arc-storage@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Data:         []string{"capybara.jpg"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 6 * time.Minute,
	})
}

func MyFiles(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome
	d := s.FixtValue().(*arc.PreData).UIDevice

	if err := arc.WaitForARCMyFilesVolumeMount(ctx, a); err != nil {
		s.Fatal("Failed to wait for MyFiles to be mounted in ARC: ", err)
	}

	cryptohomeUserPath, err := cryptohome.UserPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatalf("Failed to get the cryptohome user path for %s: %v", cr.NormalizedUser(), err)
	}
	myFilesPath := cryptohomeUserPath + "/MyFiles"

	const testFile = "capybara.jpg"

	if err := storage.TestVolumeSharing(ctx, a, cr, d, myFilesPath, "My files", arc.MyFilesUUID, testFile, s.DataPath(testFile)); err != nil {
		s.Fatal("Failed to verify MyFiles volume sharing: ", err)
	}
}
