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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/wmp"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FullScreenshot,
		Desc: "Take a full screenshot by 'Screen capture' button in the quick settings and verify the existence of the screenshot",
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
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(cleanupCtx, cr, s.OutDir(), s.HasError)

	// For verifying the full screenshot later, delete all screenshot files first.
	if err := deleteAllScreenshots(); err != nil {
		s.Fatal("Failed to delete all screenshots: ", err)
	}

	testing.ContextLog(ctx, "Launch 'Screen capture' and capture screenshot")
	if err := wmp.CaptureScreenshot(ctx, tconn, wmp.FullScreen); err != nil {
		s.Fatal("Failed to capture screenshot: ", err)
	}
	defer func(ctx context.Context) {
		if err := deleteAllScreenshots(); err != nil {
			testing.ContextLog(ctx, "Failed to delete the screenshot")
		}
	}(cleanupCtx)

	testing.ContextLog(ctx, "Check the existence and the size of the screenshot")
	if err := checkScreenshot(ctx, tconn); err != nil {
		s.Fatal("Failed to check the existence and the size of the screenshot: ", err)
	}
}

const (
	screenshotPattern = "Screenshot*.png"
)

func deleteAllScreenshots() error {
	files, err := filepath.Glob(filepath.Join(filesapp.DownloadPath, screenshotPattern))
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
func checkScreenshot(ctx context.Context, tconn *chrome.TestConn) error {
	infos, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}

	displayMode, err := infos.GetSelectedMode()
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}
	fullScreenSize := coords.NewSize(displayMode.WidthInNativePixels, displayMode.HeightInNativePixels)

	files, err := filepath.Glob(filepath.Join(filesapp.DownloadPath, screenshotPattern))
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

	if image.Width != fullScreenSize.Width || image.Height != fullScreenSize.Height {
		return errors.New("the size of the screenshot doesn't match the size of full screen")
	}

	return nil
}
