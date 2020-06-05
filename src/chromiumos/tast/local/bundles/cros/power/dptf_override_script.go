// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"

	"chromiumos/tast/local/bundles/cros/power/dptf"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DptfOverrideScript,
		Desc: "Check that dptf loads correct thermal profile from override script",
		Contacts: []string{
			"puthik@chromium.org",                // test author
			"chromeos-platform-power@google.com", // CrOS platform power developers
		},
		Attr: []string{"group:mainline"},
		// Only Atlas use override script, board developed later uses unibuild.
		HardwareDeps: hwdep.D(hwdep.Platform("atlas")),
	})
}

func DptfOverrideScript(ctx context.Context, s *testing.State) {
	expectedProfile, err := dptf.GetProfileFromOverrideScript(ctx)
	if err != nil {
		s.Fatal("GetProfileFromOverrideScript failed: ", err)
	}
	actualProfile, err := dptf.GetProfileFromPgrep(ctx)
	if err != nil {
		s.Fatal("GetProfileFromPgrep failed: ", err)
	}

	if expectedProfile != actualProfile {
		s.Errorf("Unexpected DPTF profile: got %q; want %q", actualProfile, expectedProfile)
	}
}
