// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	//"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/companionlib"
	"chromiumos/tast/local/bundles/cros/arc/perappdensity"
	"chromiumos/tast/local/bundles/cros/arc/screen"
	"chromiumos/tast/local/bundles/cros/arc/screenshot"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
	//sc "chromiumos/tast/local/screenshot"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UniformScaleFactorNonMaximized,
		Desc:         "Checks that the uniform scale factor is applied to non-maximized Android applications",
		Contacts:     []string{"sarakato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"white_wallpaper.jpg"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
		Pre:          arc.Booted(),
	})
}

// baselinePixelCount obtains the pixel count without uniform scaling applied.
// The returned value is used to confirm that uniform scaling is applied correctly.
func baselinePixelCount(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, a *arc.ARC, s *testing.State) (int, error) {
	const cleanupTime = 10 * time.Second

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		return 0, errors.Wrap(err, "failed to set tablet mode to false")
	}
	defer cleanup(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()
	act, err := perappdensity.StartViewActivityWithWindowState(ctx, tconn, a, arc.WindowStateNormal)
	if err != nil {
		return 0, errors.Wrap(err, "failed to start activity")
	}
	defer act.Stop(ctx, tconn)

	if err := screen.WaitForStableFrames(ctx, a, perappdensity.PackageName); err != nil {
		return 0, errors.Wrap(err, "failed waiting for updated frames")
	}

	winInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, perappdensity.PackageName)
	if err != nil {
		return 0, err
	}
	img, err := screenshot.CropScreenshot(ctx, cr, winInfo.BoundsInRoot)
	if err != nil {
		testing.PollBreak(err)
	}
	pixelCount, err := perappdensity.CountBlackPixels(img)
	if err != nil {
		return 0, errors.Wrap(err, "failed to count black pixels")
	}

	return pixelCount, nil
}

func UniformScaleFactorNonMaximized(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC
	if err := perappdensity.SetUpApk(ctx, a, perappdensity.Apk); err != nil {
		s.Fatal("Failed to set up perAppDensityApk: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Change the wallpaper to pure white to ensure that we aren't getting any noise
	// from the wallpaper when counting the number of black pixels.
	if err := companionlib.SetWhiteWallpaper(ctx, tconn, s); err != nil {
		s.Fatal("Failed to set white wallpaper: ", err)
	}

	s.Log(ctx, "Running app, without uniform scaling")

	baselinePixelCount, err := baselinePixelCount(ctx, cr, tconn, a, s)
	if err != nil {
		s.Fatal("Failed to get baseline pixel count: ", err)
	}

	if err := perappdensity.VerifyPixelsWithUSFEnabled(ctx, cr, tconn, a, arc.WindowStateNormal, baselinePixelCount); err != nil {
		s.Fatal("Failed to confirm state after enabling uniform scale factor: ", err)
	}
}
