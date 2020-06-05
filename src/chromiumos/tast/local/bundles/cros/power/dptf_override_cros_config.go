// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"

	"chromiumos/tast/local/bundles/cros/power/dptf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DptfOverrideCrosConfig,
		Desc: "Check that dptf loads correct thermal profile from cros_config",
		Contacts: []string{
			"puthik@chromium.org",                // test author
			"chromeos-platform-power@google.com", // CrOS platform power developers
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"dptf", "cros_config"},
	})
}

func DptfOverrideCrosConfig(ctx context.Context, s *testing.State) {
	expectedProfile, err := dptf.GetProfileFromCrosConfig(ctx)
	if err != nil {
		s.Fatal("GetProfileFromCrosConfig failed: ", err)
	}
	actualProfile, err := dptf.GetProfileFromPgrep(ctx)
	if err != nil {
		s.Fatal("GetProfileFromPgrep failed: ", err)
	}

	if expectedProfile != actualProfile {
		s.Errorf("Unexpected DPTF profile: got %q; want %q", actualProfile, expectedProfile)
	}
}
