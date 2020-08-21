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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UniformScaleFactorNonMaximized,
		Desc:         "Checks that the uniform scale factor is applied to Android applications",
		Contacts:     []string{"sarakato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
		Pre:          arc.Booted(),
	})
}

// startScaleFactorActivity starts the actvity in normal mode.
// It is the responsibility of the caller to stop the activity.
func startScaleFactorActivity(ctx context.Context, a *arc.ARC, tconn *chrome.TestConn, cr *chrome.Chrome, act *arc.Activity) error {
	perAppDensityPackageName := "org.chromium.arc.testapp.perappdensitytest"
	cleanupTime := 10 * time.Second // time reserved for cleanup.

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		return errors.Wrap(err, "failed to set tablet mode to false")
	}
	defer func(ctx context.Context) {
		cleanup(ctx)
	}(ctx)
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	if err := act.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start the activity")
	}

	if err := ash.WaitForVisible(ctx, tconn, perAppDensityPackageName); err != nil {
		return errors.Wrap(err, "failed to wait for visible app")
	}

	if err := act.SetWindowState(ctx, tconn, arc.WindowStateNormal); err != nil {
		return errors.Wrap(err, "failed to set window state to normal")
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, perAppDensityPackageName, ash.WindowStateNormal); err != nil {
		return errors.Wrap(err, "failed to wait for the activity to have window state normal")
	}

	return nil
}

func UniformScaleFactorNonMaximized(ctx context.Context, s *testing.State) {
	const (
		uniformScaleFactor        = 1.25
		perAppDensityPackageName  = "org.chromium.arc.testapp.perappdensitytest"
		uniformScaleFactorSetting = "persist.sys.ui.uniform_app_scaling"
	)

	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	if err := perappdensity.SetUpApk(ctx, a, perappdensity.DensityApk); err != nil {
		s.Fatal("Failed to set up perAppDensityApk: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	testing.ContextLog(ctx, "Running perappdensity apk, without uniform scaling")
	// Count pixels, then start and stop the app.
	act, err := arc.NewActivity(a, perAppDensityPackageName, ".ViewActivity")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := startScaleFactorActivity(ctx, a, tconn, cr, act); err != nil {
		s.Fatal("Failed start activity: ", err)
	}

	var defaultPixelCount int
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		pixelCount, err := perappdensity.CountBlackPixels(ctx, cr)
		if err != nil {
			return errors.Wrap(err, "failed to count black pixels")
		}
		// Return once pixel count stops changing.
		if pixelCount == defaultPixelCount {
			return nil
		}

		// Return error on first run, to ensure updated state is being captured.
		if defaultPixelCount == 0 {
			defaultPixelCount = pixelCount
			return errors.New("initial pixel count")
		}

		defaultPixelCount = pixelCount
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Timed out waiting for initialPixelCount")
	}
	act.Stop(ctx, tconn)

	if err := arc.BootstrapCommand(ctx, perappdensity.Setprop, uniformScaleFactorSetting, "1").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to set developer option: ", err)
	}
	expectedPixelCount := (int)((float64)(defaultPixelCount) * uniformScaleFactor * uniformScaleFactor)

	testing.ContextLog(ctx, "Running app, with uniform scaling enabled")
	if err := startScaleFactorActivity(ctx, a, tconn, cr, act); err != nil {
		s.Fatal("Failed to start activity after enabling uniform scale factor: ", err)
	}

	if err := perappdensity.ConfirmBlackPixelCount(ctx, cr, expectedPixelCount); err != nil {
		s.Fatal("Failed to verify uniform scale factor state: ", err)
	}
	act.Stop(ctx, tconn)
}
