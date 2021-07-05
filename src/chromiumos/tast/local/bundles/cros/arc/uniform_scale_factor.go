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
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UniformScaleFactor,
		Desc:         "Checks that the uniform scale factor is applied to Android applications",
		Contacts:     []string{"sarakato@chromium.org", "arc-framework+tast@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
		Fixture:      "arcBooted",
	})
}

func UniformScaleFactor(ctx context.Context, s *testing.State) {
	const (
		squareSidePx   = 100
		viewID         = perappdensity.PackageName + ":id/" + "view"
		secondActivity = ".SecondActivity"
	)

	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC
	d := s.FixtValue().(*arc.PreData).UIDevice

	if err := perappdensity.ToggleUniformScaleFactor(ctx, a, perappdensity.Enabled); err != nil {
		s.Fatal("Failed to set developer option: ", err)
	}
	defer perappdensity.ToggleUniformScaleFactor(ctx, a, perappdensity.Disabled)

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

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}

	dispMode, err := ash.PrimaryDisplayMode(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display mode: ", err)
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

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to set tablet mode to true: ", err)
	}
	defer cleanup(cleanupCtx)

	viewAct, err := perappdensity.StartActivityWithWindowState(ctx, tconn, a, arc.WindowStateFullscreen, perappdensity.ViewActivity)
	if err != nil {
		s.Fatal("Failed to start activity after enabling uniform scale factor: ", err)
	}
	defer viewAct.Close()

	squarePixelCount := (int)((dd * squareSidePx) * (dd * squareSidePx))
	if err := perappdensity.ConfirmPixelCountInActivitySurface(ctx, cr, a, color.Black, squarePixelCount, viewAct); err != nil {
		s.Fatal("Failed to confirm uniform scale factor state on ViewActivity: ", err)
	}

	view := d.Object(ui.PackageName(perappdensity.PackageName),
		ui.ClassName("android.view.View"),
		ui.ID(viewID))

	viewBounds, err := view.GetBounds(ctx)
	if err != nil {
		s.Fatal("Failed to get view bounds: ", err)
	}

	// A point inside of the view, which we will click.
	point := viewBounds.CenterPoint()
	point.X = int((float64(point.X) / dispMode.DeviceScaleFactor * perappdensity.UniformScaleFactor))
	point.Y = int((float64(point.Y) / dispMode.DeviceScaleFactor * perappdensity.UniformScaleFactor))

	// Invoke a mouse click (rather than using click from UI automator), as to avoid the caption bar
	// being shown unnecessarily. This occurs as the events from UI automator aren't forwarded to Chrome.
	if err := mouse.Click(ctx, tconn, point, mouse.LeftButton); err != nil {
		s.Fatal("Failed to click on the view: ", err)
	}

	secondAct, err := arc.NewActivity(a, perappdensity.PackageName, secondActivity)
	if err != nil {
		s.Fatal("Failed to get secondActivity: ", err)
	}
	defer secondAct.Close()

	if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for idle: ", err)
	}

	if err := perappdensity.ConfirmPixelCountInActivitySurface(ctx, cr, a, color.RGBA{255, 0, 0, 255}, squarePixelCount, secondAct); err != nil {
		s.Fatal("Failed to confirm uniform scale factor state after switching activities: ", err)
	}

}
