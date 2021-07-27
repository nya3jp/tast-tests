// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/wmp"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RecordFullScreen,
		Desc: "Checks that full screen video record works correctly",
		Contacts: []string{
			"yichenz@chromium.org",
			"chromeos-wmp@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{
			{
				Name: "clamshell_mode",
				Val:  false,
			},
			{
				Name: "tablet_mode",
				Val:  true,
			},
		},
	})
}

func RecordFullScreen(ctx context.Context, s *testing.State) {
	tabletMode := s.Param().(bool)

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(ctx)

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ac := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}

	// Starts full screen recording via UI.
	statusArea := nodewith.ClassName("ash/StatusAreaWidgetDelegate")
	collapseButton := nodewith.ClassName("CollapseButton")
	screenCaptureButton := nodewith.ClassName("FeaturePodIconButton").Name("Screen capture")
	screenRecordToggleButton := nodewith.ClassName("CaptureModeToggleButton").Name("Screen record")
	recordFullscreenToggleButton := nodewith.ClassName("CaptureModeToggleButton").Name("Record full screen")
	stopRecordButton := nodewith.ClassName("TrayBackgroundView").Name("Stop screen recording")
	recordTakenLabel := nodewith.ClassName("Label").Name("Screen recording taken")
	if err := uiauto.Combine(
		"record full screen",
		ac.LeftClick(statusArea),
		ac.WaitUntilExists(collapseButton),
		ac.LeftClick(screenCaptureButton),
		ac.LeftClick(screenRecordToggleButton),
		ac.LeftClick(recordFullscreenToggleButton),
		kb.AccelAction("Enter"),
		// Records full screen for about 30 seconds.
		ac.Sleep(30*time.Second),
		ac.LeftClick(stopRecordButton),
		// Checks if the screen record is taken.
		ac.WaitUntilExists(recordTakenLabel),
	)(ctx); err != nil {
		s.Fatal("Failed to record full screen: ", err)
	}

	// Checks there is a screen record video file stored in Downloads folder.
	has, err := wmp.HasScreenRecord(ctx)
	if err != nil {
		s.Fatal("Failed to check whether screen record is present: ", err)
	}
	if !has {
		s.Fatal("No screen record is stored in Downloads folder")
	}
}
