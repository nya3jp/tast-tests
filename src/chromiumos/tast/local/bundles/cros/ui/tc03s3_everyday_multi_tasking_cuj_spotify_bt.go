// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	et "chromiumos/tast/local/bundles/cros/ui/everydaymultitaskingcuj"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TC03S3EverydayMultiTaskingCUJSpotifyBT,
		Desc:         "Measures the performance of everyday multi-tasking CUJ",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org", "jane.yang@cienet.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"android_p", "chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
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

func TC03S3EverydayMultiTaskingCUJSpotifyBT(ctx context.Context, s *testing.State) {
	const (
		tabletMode    = false
		openBluetooth = true
	)
	et.Run(ctx, s, et.Spotify, tabletMode, openBluetooth)
}
