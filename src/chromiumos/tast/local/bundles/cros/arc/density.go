// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"image/color"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/screenshot"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Density,
		Desc:         "Checks that density can be changed with Android applications",
		Contacts:     []string{"sarakato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{densityApk},
		Timeout:      4 * time.Minute,
		Pre:          arc.Booted(),
	})
}

const (
	densityApk       = "ArcDensityTest.apk"
	defaultDensityDp = 150
)

type densityChange struct {
	action      string
	keySequence string
	densityDp   int
}

// performAndConfirmDensityChange changes the density of the activity, and confirms that the density was changed by checking
// the scale factor.
func performAndConfirmDensityChange(ctx context.Context, cr *chrome.Chrome, ew *input.KeyboardEventWriter, a *arc.ARC, test densityChange) error {
	testing.ContextLog(ctx, test.action+" density using key "+test.keySequence)
	if err := ew.Accel(ctx, test.keySequence); err != nil {
		return errors.Wrapf(err, "could not %s scale factor", test.keySequence)
	}

	// Wait for relayout to complete, before grabbing new screen shot.
	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		return err
	}
	if err := checkRectBounds(ctx, cr, test.densityDp); err != nil {
		return err
	}
	return nil
}

// checkRectBounds confirms that drawn rectangle on screen is the expected size.
func checkRectBounds(ctx context.Context, cr *chrome.Chrome, rectBound int) error {
	const colorMaxDiff = 128
	shouldBeBlack := func(x, y int) bool {
		return x < int(rectBound) && y < int(rectBound)
	}

	img, err := screenshot.GrabScreenshot(ctx, cr)
	if err != nil {
		return err
	}

	rect := img.Bounds()
	for i := 0; i < rect.Max.Y-rect.Min.Y; i++ {
		for j := 0; j < rect.Max.X-rect.Min.X; j++ {
			x := rect.Min.X + j
			y := rect.Min.Y + i
			curr := img.At(x, y)
			if shouldBeBlack(x, y) && !colorcmp.ColorsMatch(curr, color.Black, colorMaxDiff) {
				return errors.Errorf("expected black (= %s) at (%d, %d). But received %s", colorcmp.ColorStr(color.Black), x, y, colorcmp.ColorStr(curr))
			} else if !shouldBeBlack(x, y) && !colorcmp.ColorsMatch(curr, color.White, colorMaxDiff) {
				return errors.Errorf("expected white (= %s) at (%d, %d). But received %s", colorcmp.ColorStr(color.White), x, y, colorcmp.ColorStr(curr))
			}
		}
	}
	return nil
}

func Density(ctx context.Context, s *testing.State) {
	const (
		setprop        = "/system/bin/setprop"
		mainActivity   = ".MainActivity"
		packageName    = "org.chromium.arc.testapp.densitytest"
		densitySetting = "persist.sys.enable_application_zoom"
	)
	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	if err := arc.BootstrapCommand(ctx, setprop, densitySetting, "true").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to set developer option: ", err)
	}
	defer arc.BootstrapCommand(ctx, setprop, densitySetting, "false").Run(testexec.DumpLogOnError)

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)

	testing.ContextLog(ctx, "Installing app")
	if err := a.Install(ctx, s.DataPath(densityApk)); err != nil {
		s.Fatal("Failed to install app: ", densityApk)
	}

	act, err := arc.NewActivity(a, packageName, mainActivity)
	if err != nil {
		s.Fatal("Failed to create new activity")
	}
	defer act.Close()

	testing.ContextLog(ctx, "Starting activity")
	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed to start the  activity: ", err)
	}

	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to set tablet mode enabled to false: ", err)
	}
	if err := act.SetWindowState(ctx, arc.WindowStateFullscreen); err != nil {
		s.Fatal("Failed to set window state to Maximized: ", err)
	}
	if err := act.WaitForResumed(ctx, 4*time.Second); err != nil {
		s.Fatal("Failed to wait for the activity to resume: ", err)
	}
	if err := checkRectBounds(ctx, cr, defaultDensityDp); err != nil {
		s.Fatal("Failed to check size of rect drawn on screen: ", err)
	}

	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error creating keyboard: ", err)
	}
	defer ew.Close()

	for _, test := range []densityChange{
		{
			"increase",
			"ctrl+=",
			165,
		},
		{
			"reset",
			"ctrl+0",
			defaultDensityDp,
		},
		{
			"decrease",
			"ctrl+-",
			135,
		},
	} {
		if err := performAndConfirmDensityChange(ctx, cr, ew, a, test); err != nil {
			s.Fatal("Error with performing action: ", err)
		}
	}
}
