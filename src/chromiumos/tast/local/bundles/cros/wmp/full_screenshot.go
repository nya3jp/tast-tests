// Copyright 2021 The ChromiumOS Authors
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
		SearchFlags: []*testing.StringPair{
			{
				Key:   "feature_id",
				Value: "screenplay-936ea36a-b93f-4127-9260-9975e69365fa",
			},
		},
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
	if err := wmp.CaptureScreenshot(tconn, wmp.FullScreenshot)(ctx); err != nil {
		s.Fatal("Failed to capture screenshot: ", err)
	}
	defer func(ctx context.Context) {
		if err := deleteAllScreenshots(downloadsPath); err != nil {
			testing.ContextLog(ctx, "Failed to delete the screenshot")
		}
	}(cleanupCtx)

	testing.ContextLog(ctx, "Check the existence and the size of the screenshot")
	if err := wmp.CheckScreenshot(ctx, tconn, downloadsPath); err != nil {
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
