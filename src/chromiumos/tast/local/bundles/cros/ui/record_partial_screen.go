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
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type deviceModeType string

const (
	clamshellMode deviceModeType = "clamshell mode"
	tabletMode    deviceModeType = "tablet mode"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RecordPartialScreen,
		Desc: "Checks that partial screen video record works correctly",
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
				Val:  clamshellMode,
			},
			{
				Name: "tablet_mode",
				Val:  tabletMode,
			},
		},
	})
}

func RecordPartialScreen(ctx context.Context, s *testing.State) {
	deviceMode := s.Param().(deviceModeType)

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	var isTabletMode bool
	if deviceMode == tabletMode {
		isTabletMode = true
	} else {
		isTabletMode = false
	}
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, isTabletMode)
	if err != nil {
		s.Fatalf("Failed to ensure %s: %v", deviceMode, err)
	}
	defer cleanup(ctx)

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ac := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	// Starts partial screen recording via UI.
	statusArea := nodewith.ClassName("ash/StatusAreaWidgetDelegate")
	collapseButton := nodewith.ClassName("CollapseButton")
	screenCaptureButton := nodewith.ClassName("FeaturePodIconButton").Name("Screen capture")
	screenRecordToggleButton := nodewith.ClassName("CaptureModeToggleButton").Name("Screen record")
	recordPartialScreenToggleButton := nodewith.ClassName("CaptureModeToggleButton").Name("Record partial screen")
	dragStartPt := info.WorkArea.TopLeft()
	dragEndPt := info.WorkArea.CenterPoint()
	stopRecordButton := nodewith.ClassName("TrayBackgroundView").Name("Stop screen recording")
	recordTakenLabel := nodewith.ClassName("Label").Name("Screen recording taken")
	if err := uiauto.Combine(
		"record partial screen",
		ac.LeftClick(statusArea),
		ac.WaitUntilExists(collapseButton),
		ac.LeftClick(screenCaptureButton),
		ac.LeftClick(screenRecordToggleButton),
		ac.LeftClick(recordPartialScreenToggleButton),
		// Drags to select an area to record.
		mouse.Drag(tconn, dragStartPt, dragEndPt, time.Second),
		kb.AccelAction("Enter"),
		// Records partial screen for about 30 seconds.
		ac.Sleep(30*time.Second),
		ac.LeftClick(stopRecordButton),
		// Checks if the screen record is taken.
		ac.WaitUntilExists(recordTakenLabel),
	)(ctx); err != nil {
		s.Fatal("Failed to record partial screen: ", err)
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
