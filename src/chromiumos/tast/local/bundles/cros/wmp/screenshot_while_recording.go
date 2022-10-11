// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/wmp/wmputils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/wmp"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenshotWhileRecording,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that the screenshot can be performed while doing screen recording",
		Contacts: []string{
			"xdai@google.com",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// ScreenshotWhileRecording verifies that taking screenshot while doing screen recording works.
func ScreenshotWhileRecording(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// For verifying the screenshot and screen recording later, delete all screen capture files first.
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}
	if err := wmp.DeleteAllScreenCaptureFiles(downloadsPath, true /*deleteScreenRecording=*/, true /*deleteScreenshots=*/); err != nil {
		s.Fatal("Failed to delete all screenshots and screen recordings: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	if err := wmp.LaunchScreenCapture(ctx, tconn); err != nil {
		s.Fatal("Failed to launch 'Screen capture': ", err)
	}
	defer wmputils.EnsureCaptureModeActivated(tconn, false)(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	s.Log("Launch 'Screen capture' and start recording")
	if err := wmp.StartFullScreenRecording(tconn)(ctx); err != nil {
		s.Fatal("Failed to start screen recording: ", err)
	}
	s.Log("Start screenshot while recording is in progress")
	if err := wmp.LaunchScreenCapture(ctx, tconn); err != nil {
		s.Fatal("Failed to launch 'Screen capture again': ", err)
	}
	if err := wmp.CaptureScreenshot(tconn, wmp.FullScreenshot)(ctx); err != nil {
		s.Fatal("Failed to capture screenshot while recording is in progress: ", err)
	}
	s.Log("End screen recording now")
	if err := wmp.EndScreenRecording(tconn)(ctx); err != nil {
		s.Fatal("Failed to end screen recording: ", err)
	}

	defer func() {
		if err := wmp.DeleteAllScreenCaptureFiles(downloadsPath, true /*deleteScreenRecording=*/, true /*deleteScreenshots=*/); err != nil {
			s.Fatal("Failed to delete all screenshots and screen recordings: ", err)
		}
	}()

	s.Log("Check the existence and the size of the screenshot and recording")
	if err := wmp.CheckScreenshot(ctx, tconn, downloadsPath); err != nil {
		s.Fatal("Failed to verify the screenshot: ", err)
	}
	s.Log("Check the existence and the size of the screen recording")
	has, err := wmputils.HasScreenRecord(ctx, downloadsPath)
	if err != nil {
		s.Fatal("Failed to check whether screen record is present: ", err)
	}
	if !has {
		s.Fatal("No screen record is stored in Downloads folder")
	}
}
