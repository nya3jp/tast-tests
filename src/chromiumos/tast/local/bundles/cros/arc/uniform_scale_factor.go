// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"image/color"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/perappdensity"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UniformScaleFactor,
		Desc:         "Checks that the uniform scale factor is applied to Android applications",
		Contacts:     []string{"sarakato@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
		Pre:          arc.Booted(),
	})
}

func UniformScaleFactor(ctx context.Context, s *testing.State) {
	const (
		squareSidePx   = 100
		viewID         = perappdensity.PackageName + ":id/" + "view"
		secondActivity = ".SecondActivity"
	)

	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	if err := arc.BootstrapCommand(ctx, perappdensity.Setprop, perappdensity.UniformScaleFactorSetting, "1").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to set developer option: ", err)
	}

	if err := arc.BootstrapCommand(ctx, perappdensity.Setprop, "persist.sys.enablechromecaption", "0").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to set developer option: ", err)
	}

	dd, err := perappdensity.MeasureDisplayDensity(ctx, a)
	if err != nil {
		s.Fatal("Error obtaining initial display density: ", err)
	}

	if err := perappdensity.SetUpApk(ctx, a, perappdensity.Apk); err != nil {
		s.Fatal("Failed to set up perAppDensityApk: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to initialize UI Automator: ", err)
	}
	defer d.Close(ctx)

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to set tablet mode to true: ", err)
	}
	defer cleanup(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	viewAct, err := perappdensity.StartActivityWithWindowState(ctx, tconn, a, arc.WindowStateFullscreen, perappdensity.ViewActivity)
	if err != nil {
		s.Fatal("Failed to start activity after enabling uniform scale factor: ", err)
	}
	defer viewAct.Close()

	wantPixelCount := (int)((dd * squareSidePx) * (dd * squareSidePx))
	if err := perappdensity.VerifyPixelCount(ctx, cr, a, color.Black, wantPixelCount, viewAct); err != nil {
		s.Fatal("Failed to confirm uniform scale factor state on ViewActivity: ", err)
	}

	if err := d.Object(ui.ID(viewID)).Click(ctx); err != nil {
		s.Fatalf("Failed to click %s view: %v", viewID, err)
	}

	secondAct, err := arc.NewActivity(a, perappdensity.PackageName, secondActivity)
	if err != nil {
		s.Fatal("Failed to get secondActivity: ", err)
	}
	defer secondAct.Close()

	if err := secondAct.SetWindowState(ctx, tconn, arc.WindowStateFullscreen); err != nil {
		s.Fatal("Failed to set window state to normal: ", err)
	}

	if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for idle, ignoring: ", err)
	}
	if err := perappdensity.VerifyPixelCount(ctx, cr, a, color.RGBA{255, 0, 0, 255}, wantPixelCount, viewAct); err != nil {
		s.Fatal("Failed to confirm uniform scale factor state after switching activities: ", err)
	}

}
