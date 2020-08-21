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

// defaultPixelCount obtains the pixel count without uniform scaling applied.
// The returned value is used as the baseline to confirm that unifrom scaling is applied correctly.
func defaultPixelCount(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, a *arc.ARC, act *arc.Activity, outDir string) (int, error) {
	act, err := perappdensity.StartDensityActivityWithWindowState(ctx, cr, tconn, a, arc.WindowStateNormal)
	if err != nil {
		return 0, errors.Wrap(err, "failed to start activity")
	}
	defer func(ctx context.Context, tconn *chrome.TestConn) {
		act.Stop(ctx, tconn)
	}(ctx, tconn)
	cleanupTime := 10 * time.Second
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	var initialPixelCount, pixelCount, prevPixelCount int
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		pixelCount, err = perappdensity.CountBlackPixels(ctx, cr)
		if err != nil {
			return errors.Wrap(err, "failed to count black pixels")
		}

		// Return once pixel count stops changing.
		if pixelCount != initialPixelCount && prevPixelCount != initialPixelCount {
			return nil
		}

		prevPixelCount = pixelCount
		return errors.Errorf("pixel count %d", pixelCount)
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return 0, errors.Wrap(err, "timed out waiting for initial pixel count")
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
	act, err := arc.NewActivity(a, perappdensity.DensityPackageName, ".ViewActivity")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	defaultPixelCount, err := defaultPixelCount(ctx, cr, tconn, a, act, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get default pixel count: ", err)
	}

	if err := perappdensity.VerifyPixelsWithUSFEnabled(ctx, a, tconn, cr, arc.WindowStateNormal, defaultPixelCount); err != nil {
		s.Fatal("Failed to confirm state after enabling uniform scale factor: ", err)
	}
}
