// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/wmp/wmputils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/cryptohome"
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
		Func:         RecordPartialScreen,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that partial screen video record works correctly",
		Contacts: []string{
			"afakhry@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		SearchFlags: []*testing.StringPair{
			{
				Key:   "feature_id",
				Value: "screenplay-936ea36a-b93f-4127-9260-9975e69365fa",
			},
		},
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

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, isTabletMode)
	if err != nil {
		s.Fatalf("Failed to ensure %s: %v", deviceMode, err)
	}
	defer cleanup(cleanupCtx)

	ac := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	// Start partial screen recording via UI.
	screenRecordToggleButton := nodewith.ClassName("CaptureModeToggleButton").Name("Screen record")
	recordPartialScreenToggleButton := nodewith.ClassName("CaptureModeToggleButton").Name("Record partial screen")
	dragStartPt := info.WorkArea.CenterPoint().Sub(coords.Point{X: 100, Y: 100})
	dragEndPt := info.WorkArea.CenterPoint().Add(coords.Point{X: 100, Y: 100})
	// The click point must be outside of drag area (i.e. outside of dragStartPt - dragEndPt rectangle).
	dragClearPt := info.WorkArea.BottomCenter()
	stopRecordButton := nodewith.ClassName("TrayBackgroundView").Name("Stop screen recording")
	recordTakenLabel := nodewith.ClassName("Label").Name("Screen recording taken")

	// Enter screen capture mode.
	if err := wmputils.EnsureCaptureModeActivated(tconn, true)(ctx); err != nil {
		s.Fatal("Failed to enable recording: ", err)
	}
	// Ensure case exit screen capture mode.
	defer wmputils.EnsureCaptureModeActivated(tconn, false)(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	if err := uiauto.Combine(
		"record partial screen",
		ac.LeftClick(screenRecordToggleButton),
		ac.LeftClick(recordPartialScreenToggleButton),
		// Clear the drag area.
		mouse.Click(tconn, dragClearPt, mouse.LeftButton),
		// Drag to select an area to record.
		mouse.Drag(tconn, dragStartPt, dragEndPt, time.Second),
		kb.AccelAction("enter"),
		// Record partial screen for about 30 seconds.
		uiauto.Sleep(30*time.Second),
		ac.LeftClick(stopRecordButton),
		// Check if the screen record is taken.
		ac.WaitUntilExists(recordTakenLabel),
	)(ctx); err != nil {
		s.Fatal("Failed to record partial screen: ", err)
	}

	// Check there is a screen record video file stored in Downloads folder.
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}
	has, err := wmputils.HasScreenRecord(ctx, downloadsPath)
	if err != nil {
		s.Fatal("Failed to check whether screen record is present: ", err)
	}
	if !has {
		s.Fatal("No screen record is stored in Downloads folder")
	}
}
