// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

const (
	pattern = "Screenshot*.png"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CaptureFullScreenshot,
		Desc:         "Capture full screenshot",
		Contacts:     []string{"sun.tsai@cienet.com", "cienet-development@googlegroups.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Timeout:      2 * time.Minute,
	})
}

func CaptureFullScreenshot(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer tconn.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(cleanupCtx, cr, s.OutDir(), s.HasError)

	// For verifying the full screenshot later, delete all screenshot files first.
	if err := deleteAllScreenshots(); err != nil {
		s.Fatal("Failed to delete all screenshots: ", err)
	}

	ui := uiauto.New(tconn)
	statusArea := nodewith.HasClass("ash/StatusAreaWidgetDelegate")
	collapseBtn := nodewith.HasClass("CollapseButton")
	screenCaptureBtn := nodewith.Name("Screen capture").HasClass("FeaturePodIconButton")

	testing.ContextLog(ctx, "Open the uber tray")
	if err := uiauto.Combine("open the uber tray",
		ui.LeftClick(statusArea),
		ui.WaitUntilExists(collapseBtn),
		ui.LeftClick(screenCaptureBtn),
	)(ctx); err != nil {
		s.Fatal("Failed to open the uber tray: ", err)
	}

	toggleBtn := nodewith.Role(role.ToggleButton)
	btn := nodewith.Role(role.Button)

	// To make sure "Screen capture" is launched correctly, verify the existence of these buttons.
	testing.ContextLog(ctx, "Verify the ui of toolbar")
	for _, btn := range []*nodewith.Finder{
		toggleBtn.Name("Screenshot"),
		toggleBtn.Name("Screen record"),
		toggleBtn.Name("Take full screen screenshot"),
		toggleBtn.Name("Take partial screenshot"),
		toggleBtn.Name("Take window screenshot"),
		btn.Name("CaptureModeButton"),
	} {
		if err := ui.WaitUntilExists(btn)(ctx); err != nil {
			s.Fatal("Failed to verify the ui of toolbar: ", err)
		}
	}

	verifyText := nodewith.Name("Click anywhere to capture full screen")
	window := nodewith.Role(role.Window).First()

	testing.ContextLog(ctx, "Take full screen screenshot")
	if err := uiauto.Combine("take full screenshot",
		ui.LeftClick(toggleBtn.Name("Screenshot")),
		ui.LeftClick(toggleBtn.Name("Take full screen screenshot")),
		ui.WaitUntilExists(verifyText),
		ui.LeftClick(window), // Click on the center of root window to take the screenshot.
	)(ctx); err != nil {
		s.Fatal("Failed to take full screenshot: ", err)
	}
	defer func(ctx context.Context) {
		if err := deleteAllScreenshots(); err != nil {
			testing.ContextLog(ctx, "Failed to delete the screenshot")
		}
	}(cleanupCtx)

	screenshotTakenText := nodewith.Role(role.StaticText).Name("Screenshot taken")
	re := regexp.MustCompile(`^(Screenshot).*(\.png)$`)
	screenshotFile := nodewith.Role(role.StaticText).NameRegex(re)
	closeFilesAppBtn := nodewith.Role(role.Button).Name("Close")

	testing.ContextLog(ctx, "Open Files app to make sure the screenshot has been saved")
	if err := uiauto.Combine("open files app",
		ui.LeftClick(screenshotTakenText),  // Click the node will open Files app.
		ui.WaitUntilExists(screenshotFile), // Wait for the screenshot file saved.
	)(ctx); err != nil {
		s.Fatal("Failed to take full screenshot: ", err)
	}
	defer func(ctx context.Context) {
		if err := ui.LeftClick(closeFilesAppBtn)(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close Files app")
		}
	}(cleanupCtx)

	testing.ContextLog(ctx, "Check the size of the screenshot")
	// Check the size of the screenshot to verify the full screen size image has been captured.
	if err := checkScreenshotSize(ctx, tconn); err != nil {
		s.Fatal("Failed to check the size of the screenshot: ", err)
	}
}

func deleteAllScreenshots() error {
	files, err := filepath.Glob(filepath.Join(filesapp.DownloadPath, pattern))
	if err != nil {
		return errors.Wrapf(err, "the pattern %q is malformed", pattern)
	}

	for _, f := range files {
		if err := os.Remove(f); err != nil {
			return errors.Wrap(err, "failed to delete the screenshot")
		}
	}

	return nil
}

func checkScreenshotSize(ctx context.Context, tconn *chrome.TestConn) error {
	infos, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}

	displayMode, err := infos.GetSelectedMode()
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}
	fullScreenSize := coords.NewSize(displayMode.WidthInNativePixels, displayMode.HeightInNativePixels)

	files, err := filepath.Glob(filepath.Join(filesapp.DownloadPath, pattern))
	if err != nil {
		return errors.Wrapf(err, "the pattern %q is malformed", pattern)
	}

	if len(files) == 0 {
		return errors.Errorf("failed to find the screenshot with pattern %q", pattern)
	} else if len(files) > 1 {
		return errors.Errorf("1 screeshot expected, but got %d screenshots", len(files))
	}

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

	if image.Width != fullScreenSize.Width || image.Height != fullScreenSize.Height {
		return errors.New("the size of the screenshot doesn't match the size of full screen")
	}

	return nil
}
