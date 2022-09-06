// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"image"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wmp/wmputils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/wmp"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FullScreenshot,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Take a full screenshot by 'Screen capture' button in the quick settings and verify the existence of the screenshot",
		Contacts: []string{
			"sun.tsai@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// FullScreenshot takes a full screenshot and verify the screenshot's existence.
func FullScreenshot(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// For verifying the full screenshot later, delete all screenshot files first.
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}
	if err := deleteAllScreenshots(downloadsPath); err != nil {
		s.Fatal("Failed to delete all screenshots: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	if err := wmp.LaunchScreenCapture(ctx, tconn); err != nil {
		s.Fatal("Failed to launch 'Screen capture': ", err)
	}
	defer wmputils.EnsureCaptureModeActivated(tconn, false)(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	testing.ContextLog(ctx, "Launch 'Screen capture' and capture screenshot")
	if err := wmp.CaptureScreenshot(tconn, wmp.FullScreen)(ctx); err != nil {
		s.Fatal("Failed to capture screenshot: ", err)
	}
	defer func(ctx context.Context) {
		if err := deleteAllScreenshots(downloadsPath); err != nil {
			testing.ContextLog(ctx, "Failed to delete the screenshot")
		}
	}(cleanupCtx)

	testing.ContextLog(ctx, "Check the existence and the size of the screenshot")
	if err := checkScreenshot(ctx, tconn, downloadsPath); err != nil {
		s.Fatal("Failed to verify the screenshot: ", err)
	}
}

const (
	screenshotPattern = "Screenshot*.png"
)

func deleteAllScreenshots(downloadsPath string) error {
	files, err := filepath.Glob(filepath.Join(downloadsPath, screenshotPattern))
	if err != nil {
		return errors.Wrapf(err, "the pattern %q is malformed", screenshotPattern)
	}

	for _, f := range files {
		if err := os.Remove(f); err != nil {
			return errors.Wrap(err, "failed to delete the screenshot")
		}
	}

	return nil
}

// checkScreenshot checks the screenshot's existence.
// And then verifies its size is the same as the size of the full screen by decoding the screenshot.
func checkScreenshot(ctx context.Context, tconn *chrome.TestConn, downloadsPath string) error {
	displayInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the primary display info")
	}

	displayMode, err := displayInfo.GetSelectedMode()
	if err != nil {
		return errors.Wrap(err, "failed to obtain the display mode")
	}

	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the display orientation")
	}

	expectedFullScreenshotSize := coords.NewSize(displayMode.WidthInNativePixels, displayMode.HeightInNativePixels)
	// The screen ui orientation might be different from the DUT's default orientation setting.
	// If the orientation angle is 90 or 270 degrees,
	// swap the width and height value to match the current screen ui orientation.
	if orientation.Angle == 90 || orientation.Angle == 270 {
		expectedFullScreenshotSize.Width, expectedFullScreenshotSize.Height = expectedFullScreenshotSize.Height, expectedFullScreenshotSize.Width
	}

	files, err := filepath.Glob(filepath.Join(downloadsPath, screenshotPattern))
	if err != nil {
		return errors.Wrapf(err, "the pattern %q is malformed", screenshotPattern)
	}

	if len(files) == 0 {
		return errors.New("screenshot not found")
	} else if len(files) > 1 {
		return errors.Errorf("unexpected screeshot count, want 1, got %d", len(files))
	}

	// Expecting only one screenshot exist.
	imgFile := files[0]

	reader, err := os.Open(imgFile)
	if err != nil {
		return errors.Wrap(err, "failed to open the screenshot")
	}
	defer reader.Close()

	image, _, err := image.DecodeConfig(reader)
	if err != nil {
		return errors.Wrap(err, "failed to decode the screenshot")
	}

	if image.Width != expectedFullScreenshotSize.Width || image.Height != expectedFullScreenshotSize.Height {
		return errors.Errorf("screenshot size mismatched: want %s, got (%d x %d)", expectedFullScreenshotSize, image.Width, image.Height)
	}

	return nil
}
