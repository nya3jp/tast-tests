// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/cuj"
	et "chromiumos/tast/local/bundles/cros/ui/everydaymultitaskingcuj"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TC03T3EverydayMultiTaskingCUJSpotifyBT,
		Desc:         "Measures the performance of everyday multi-tasking CUJ",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org", "jane.yang@cienet.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"android_p", "chrome", "tablet_mode"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay(), hwdep.TouchScreen()),
		Timeout:      15 * time.Minute,
		Vars: []string{
			"mute",
			"ui.bt_devicename",
			"perf_level",
		},
		Fixture: "loggedInToCUJUserKeepState",
		Data:    []string{"cca_ui.js"},
	})
}

func TC03T3EverydayMultiTaskingCUJSpotifyBT(ctx context.Context, s *testing.State) {
	const (
		tabletMode    = true
		openBluetooth = true
	)
	cr := s.FixtValue().(cuj.FixtureData).Chrome
	a := s.FixtValue().(cuj.FixtureData).ARC
	et.Run(ctx, s, cr, a, et.Spotify, tabletMode, openBluetooth)
}
