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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RecordFullScreen,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that full screen video record works correctly",
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

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	ac := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}

	// Starts full screen recording via UI.
	screenRecordToggleButton := nodewith.ClassName("CaptureModeToggleButton").Name("Screen record")
	recordFullscreenToggleButton := nodewith.ClassName("CaptureModeToggleButton").Name("Record full screen")
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
		"record full screen",
		ac.LeftClick(screenRecordToggleButton),
		ac.LeftClick(recordFullscreenToggleButton),
		kb.AccelAction("Enter"),
		// Records full screen for about 30 seconds.
		uiauto.Sleep(30*time.Second),
		ac.LeftClick(stopRecordButton),
		// Checks if the screen record is taken.
		ac.WaitUntilExists(recordTakenLabel),
	)(ctx); err != nil {
		s.Fatal("Failed to record full screen: ", err)
	}

	// Checks there is a screen record video file stored in Downloads folder.
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
