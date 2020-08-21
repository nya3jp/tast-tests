// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/perappdensity"
	"chromiumos/tast/local/bundles/cros/arc/screen"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UniformScaleFactorNonMaximized,
		Desc:         "Checks that the uniform scale factor is applied to non maximized Android applications",
		Contacts:     []string{"sarakato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
		Pre:          arc.Booted(),
	})
}

// baselinePixelCount obtains the pixel count without uniform scaling applied.
// The returned value is used to confirm that uniform scaling is applied correctly.
func baselinePixelCount(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, a *arc.ARC) (int, error) {
	const cleanupTime = 10 * time.Second
	act, err := perappdensity.StartDensityActivityWithWindowState(ctx, tconn, a, arc.WindowStateNormal)
	if err != nil {
		return 0, errors.Wrap(err, "failed to start activity")
	}
	defer act.Stop(ctx, tconn)

	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()
	if err := screen.WaitForStableFrames(ctx, a, perappdensity.DensityPackageName); err != nil {
		return 0, errors.Wrap(err, "failed waiting for updated frames")
	}

	pixelCount, err := perappdensity.CountBlackPixels(ctx, cr)
	if err != nil {
		return 0, errors.Wrap(err, "failed to count black pixels")
	}

	return pixelCount, nil
}

func UniformScaleFactorNonMaximized(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	if err := perappdensity.SetUpApk(ctx, a, perappdensity.DensityApk); err != nil {
		s.Fatal("Failed to set up perAppDensityApk: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	testing.ContextLog(ctx, "Running app, without uniform scaling")

	baselinePixelCount, err := baselinePixelCount(ctx, cr, tconn, a)
	if err != nil {
		s.Fatal("Failed to get baseline pixel count: ", err)
	}

	if err := perappdensity.VerifyPixelsWithUSFEnabled(ctx, cr, tconn, a, arc.WindowStateNormal, baselinePixelCount); err != nil {
		s.Fatal("Failed to confirm state after enabling uniform scale factor: ", err)
	}
}
