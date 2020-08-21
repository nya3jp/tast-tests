// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
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
		Desc:         "Checks that the uniform scale factor is applied to Android applications",
		Contacts:     []string{"sarakato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
		Pre:          arc.Booted(),
	})
}

func UniformScaleFactor(ctx context.Context, s *testing.State) {
	const (
		cleanupTime              = 10 * time.Second // time reserved for cleanup.
		squareSidePx             = 100
		uniformScaleFactor       = 1.25
		perAppDensityPackageName = "org.chromium.arc.testapp.perappdensitytest"

		uniformScaleFactorSetting = "persist.sys.ui.uniform_app_scaling"
	)

	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	if err := arc.BootstrapCommand(ctx, perappdensity.Setprop, uniformScaleFactorSetting, "1").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to set developer option: ", err)
	}

	dd, err := perappdensity.MeasureDisplayDensity(ctx, a)
	if err != nil {
		s.Fatal("Error obtaining initial display density: ", err)
	}

	expectedPixelCount := (int)((dd * squareSidePx * uniformScaleFactor) * (dd * squareSidePx * uniformScaleFactor))

	if err := perappdensity.SetUpApk(ctx, a, perappdensity.DensityApk); err != nil {
		s.Fatal("Failed to set up perAppDensityApk: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	act, err := arc.NewActivity(a, perAppDensityPackageName, ".ViewActivity")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to set tablet mode to false: ", err)
	}
	defer func(ctx context.Context) {
		cleanup(ctx)
	}(ctx)
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}
	defer func(ctx context.Context, tconn *chrome.TestConn) {
		act.Stop(ctx, tconn)
	}(ctx, tconn)
	ctx, cancel = ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	if err := ash.WaitForVisible(ctx, tconn, perAppDensityPackageName); err != nil {
		s.Fatal("Failed to wait for visible app: ", err)
	}

	if err := act.SetWindowState(ctx, tconn, arc.WindowStateFullscreen); err != nil {
		s.Fatal("Failed to set window state to fullscreen: ", err)
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, perAppDensityPackageName, ash.WindowStateFullscreen); err != nil {
		s.Fatal("Failed to wait for the activity to be fullscreen: ", err)
	}

	if err := perappdensity.CountBlackPixels(ctx, cr, expectedPixelCount); err != nil {
		s.Fatal("Failed to confirm expected pixels: ", err)
	}
}
