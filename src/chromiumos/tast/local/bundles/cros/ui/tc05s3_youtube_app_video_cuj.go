// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/videocuj"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TC05S3YoutubeAppVideoCUJ,
		Desc:         "Measures the smoothess of switch between full screen YouTube video and another browser window",
		Contacts:     []string{"xiyuan@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      6 * time.Minute,
		Fixture:      "loggedInToCUJUserKeepState",
		Vars: []string{
			"perf_level",
		},
	})
}

func TC05S3YoutubeAppVideoCUJ(ctx context.Context, s *testing.State) {
	const (
		pkgName    = "com.google.android.youtube"
		tabletMode = false
	)
	cr := s.FixtValue().(cuj.FixtureData).Chrome
	a := s.FixtValue().(cuj.FixtureData).ARC

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to setup ARC and Play Store: ", err)
	}
	defer d.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	s.Log("Installing app from play store")
	if err := playstore.InstallApp(ctx, a, d, pkgName, -1); err != nil {
		s.Fatalf("Failed to install %s: %v", pkgName, err)
	}

	s.Log("Closing play store")
	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to close Play Store: ", err)
	}

	videocuj.Run(ctx, s, cr, a, videocuj.YoutubeApp, tabletMode)
}
