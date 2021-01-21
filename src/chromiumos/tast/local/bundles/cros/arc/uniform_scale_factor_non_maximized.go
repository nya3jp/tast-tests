// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"image/color"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/perappdensity"
	"chromiumos/tast/local/bundles/cros/arc/screen"
	"chromiumos/tast/local/bundles/cros/arc/screenshot"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     UniformScaleFactorNonMaximized,
		Desc:     "Checks that the uniform scale factor is applied to non-maximized Android applications",
		Contacts: []string{"sarakato@chromium.org", "arc-framework+tast@google.com"},
		// TODO(http://b/172089190): Test is disabled until it can be fixed
		// Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
		Pre:          arc.Booted(),
	})
}

// baselinePixelCount obtains the pixel count without uniform scaling applied.
// The returned value is used to confirm that uniform scaling is applied correctly.
func baselinePixelCount(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, a *arc.ARC) (int, error) {
	const cleanupTime = 10 * time.Second

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		return 0, errors.Wrap(err, "failed to set tablet mode to false")
	}
	defer cleanup(ctx)

	act, err := perappdensity.StartActivityWithWindowState(ctx, tconn, a, arc.WindowStateNormal, perappdensity.ViewActivity)
	if err != nil {
		return 0, errors.Wrap(err, "failed to start activity")
	}
	defer act.Stop(ctx, tconn)

	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	if err := screen.WaitForStableFrames(ctx, a, perappdensity.PackageName); err != nil {
		return 0, errors.Wrap(err, "failed waiting for updated frames")
	}

	bounds, err := act.SurfaceBounds(ctx)
	if err != nil {
		return 0, err
	}

	img, err := screenshot.GrabAndCropScreenshot(ctx, cr, bounds)
	if err != nil {
		return 0, err
	}

	return screenshot.CountPixels(img, color.Black), nil
}

func UniformScaleFactorNonMaximized(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC
	s.Log(ctx, "Running app, without uniform scaling")
	if err := perappdensity.SetUpApk(ctx, a, perappdensity.Apk); err != nil {
		s.Fatal("Failed to set up perappdensity.apk: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}

	origShelfBehavior, err := ash.GetShelfBehavior(ctx, tconn, dispInfo.ID)
	if err != nil {
		s.Fatal("Failed to get shelf behavior: ", err)
	}

	// Hide shelf.
	if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorAlwaysAutoHide); err != nil {
		s.Fatal("Failed to set shelf behavior to Always Auto Hide: ", err)
	}
	// Restore shelf state to original behavior.
	defer ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, origShelfBehavior)

	baselinePixelCount, err := baselinePixelCount(ctx, cr, tconn, a)
	if err != nil {
		s.Fatal("Failed to get baseline pixel count: ", err)
	}

	if err := perappdensity.VerifyPixelsWithUSFEnabled(ctx, cr, tconn, a, arc.WindowStateNormal, baselinePixelCount, color.Black); err != nil {
		s.Fatal("Failed to confirm state after enabling uniform scale factor: ", err)
	}
}
