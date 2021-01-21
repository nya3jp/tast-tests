// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"image/color"
	"time"

	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/perappdensity"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UniformScaleFactorActivity,
		Desc:         "Checks that the uniform scale factor is persisted across activities in Android applications",
		Contacts:     []string{"sarakato@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
		Pre:          arc.Booted(),
	})
}

func UniformScaleFactorActivity(ctx context.Context, s *testing.State) {
	// Figure out how to click button
	const squareSidePx = 100

	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

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

	wantPixelCount := (int)((dd * squareSidePx) * (dd * squareSidePx))
	if err := perappdensity.VerifyPixelsWithUSFEnabled(ctx, cr, tconn, a, arc.WindowStateFullscreen, wantPixelCount, color.Black); err != nil {
		s.Fatal("Failed to confirm state after enabling uniform scale factor: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to initialize UI Automator: ", err)
	}
	defer d.Close(ctx)

	// Hide shelf.
	if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorAlwaysAutoHide); err != nil {
		s.Fatal("Failed to set shelf behavior to Always Auto Hide: ", err)
	}
	idPrefix := perappdensity.PackageName + ":id/"
	sendID := idPrefix + "button"
	if err := d.Object(ui.ID(sendID)).Click(ctx); err != nil {
		s.Fatalf("Failed to click %s button: %v", sendID, err)
	}

	var col color.RGBA
	col.R = 255
	col.G = 0
	col.B = 0
	col.A = 255
	if err := perappdensity.VerifyPixelsWithUSFEnabled(ctx, cr, tconn, a, arc.WindowStateFullscreen, wantPixelCount, col); err != nil {
		s.Fatal("Failed to confirm state after enabling uniform scale factor: ", err)
	}
}
