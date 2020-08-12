// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

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
		Func:         UniformScaleFactor,
		Desc:         "Checks that density can be changed with Android applications",
		Contacts:     []string{"sarakato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
		Pre:          arc.Booted(),
	})
}

const perAppDensityPackageName = "org.chromium.arc.testapp.perappdensitytest"

func startActivityAndCountPixels(ctx context.Context, a *arc.ARC, tconn *chrome.TestConn, cr *chrome.Chrome, expectedPixelCount float64) error {
	act, err := arc.NewActivity(a, perAppDensityPackageName, ".ViewActivity")
	if err != nil {
		return errors.Wrap(err, "failed to create new activity")
	}
	defer act.Close()

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		return errors.Wrap(err, "failed to set tablet mode to false")
	}
	defer cleanup(ctx)

	if err := act.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start the activity")
	}
	defer act.Stop(ctx, tconn)

	if err := ash.WaitForVisible(ctx, tconn, perAppDensityPackageName); err != nil {
		return errors.Wrap(err, "failed to wait for visible app")
	}

	if err := act.SetWindowState(ctx, tconn, arc.WindowStateFullscreen); err != nil {
		return errors.Wrap(err, "failed to set window state to fullscreen")
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, perAppDensityPackageName, ash.WindowStateFullscreen); err != nil {
		return errors.Wrap(err, "failed to wait for the activity to be fullscreen")
	}

	if err := perappdensity.CountBlackPixels(ctx, cr, int(expectedPixelCount)); err != nil {
		return errors.Wrap(err, "failed to confirm state")
	}

	return nil
}

func UniformScaleFactor(ctx context.Context, s *testing.State) {
	const (
		uniformSF = 1.25
		// Defined in XML files in vendor/google_arc/packages/developments/ArcPerAppDensityTest/res/layout.
		squareSidePx              = 100
		uniformScaleFactorSetting = "persist.sys.ui.uniform_app_scaling"
	)

	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	if err := arc.BootstrapCommand(ctx, perappdensity.Setprop, uniformScaleFactorSetting, "true").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to set developer option: ", err)
	}

	dd, err := perappdensity.MeasureDisplayDensity(ctx, a)
	if err != nil {
		s.Fatal("Error obtaining initial display density: ", err)
	}

	if err := perappdensity.SetUpApk(ctx, a, perappdensity.DensityApk); err != nil {
		s.Fatal("Failed to setup perAppDensityApk: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	expectedInitialPixelCount := (dd * squareSidePx) * (dd * squareSidePx)
	expectedUniformSFPixelCount := expectedInitialPixelCount * float64(uniformSF) * float64(uniformSF)

	if err := startActivityAndCountPixels(ctx, a, tconn, cr, expectedUniformSFPixelCount); err != nil {
		s.Fatal("Failed to check: ", err)
	}
}
