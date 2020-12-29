// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/videocuj"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TC05T2NetflixVideoCUJ,
		Desc:         "Measures the smoothess of switch between full screen Netflix video and another browser window",
		Contacts:     []string{"xiyuan@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      5 * time.Minute,
		Fixture:      "loggedInToCUJUserKeepState",
		Vars: []string{
			"ui.netflix_username",
			"ui.netflix_password",
			"perf_level",
		},
	})
}

func TC05T2NetflixVideoCUJ(ctx context.Context, s *testing.State) {
	const tabletMode = true
	cr := s.FixtValue().(cuj.FixtureData).Chrome
	videocuj.Run(ctx, s, cr, nil, videocuj.NetflixWeb, tabletMode)
}
