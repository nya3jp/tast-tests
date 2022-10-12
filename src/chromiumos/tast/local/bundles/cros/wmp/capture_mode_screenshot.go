// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wmp/wmputils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/wmp"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CaptureModeScreenshot,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Take a fullscreen, partial or window screen shot by pressing the 'Screen capture' button in the quick settings and verify the existence of the screenshot",
		Contacts: []string{
			"michelefan@chromium.com",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{
			{
				Name: "fullscreen",
				Val:  wmp.FullScreen,
			},
			{
				Name: "partialscreen",
				Val:  wmp.PartialScreen,
			},
			{
				Name: "window",
				Val:  wmp.Window,
			},
		},
	})
}

func CaptureModeScreenshot(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// For verifying the screenshot later, delete all screenshot files first.
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}
	if err := deleteAllScreenshots(downloadsPath); err != nil {
		s.Fatal("Failed to delete all screenshots: ", err)
	}

	// Reserve 15 seconds for the cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	if err := wmp.LaunchScreenCapture(ctx, tconn); err != nil {
		s.Fatal("Failed to launch 'Screen capture': ", err)
	}
	defer wmputils.EnsureCaptureModeActivated(tconn, false)(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	testing.ContextLog(ctx, "Launch 'Screen capture' and capture screenshot")
	source := s.Param().(wmp.CaptureModeSource)

	if err := wmp.CaptureScreenshot(ctx, tconn, source)(ctx); err != nil {
		s.Fatal("Failed to capture screenshot: ", err)
	}
	defer func(ctx context.Context) {
		if err := deleteAllScreenshots(downloadsPath); err != nil {
			testing.ContextLog(ctx, "Failed to delete the screenshot")
		}
	}(cleanupCtx)

	testing.ContextLog(ctx, "Check the existence and the size of the screenshot")
	if err := wmp.CheckScreenshot(ctx, tconn, downloadsPath, source); err != nil {
		s.Fatal("Failed to verify the screenshot: ", err)
	}
}

func deleteAllScreenshots(downloadsPath string) error {
	const screenshotPattern = "Screenshot*.png"
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
