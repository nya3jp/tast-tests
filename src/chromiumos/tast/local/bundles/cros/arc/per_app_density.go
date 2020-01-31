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
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerAppDensity,
		Desc:         "Checks that density can be changed with Android applications",
		Contacts:     []string{"sarakato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{perAppDensityApk},
		Timeout:      4 * time.Minute,
		Pre:          arc.Booted(),
	})
}

const (
	perAppDensityApk = "ArcPerAppDensityTest.apk"
)

// performAndConfirmDensityChange changes the density of the activity,
// and confirms that the density was changed by validating the size of the square on the screen.
func performAndConfirmDensityChange(ctx context.Context, cr *chrome.Chrome, ew *input.KeyboardEventWriter, a *arc.ARC, name string, keySequence string, densityDp int) error {
	testing.ContextLogf(ctx, "%s density using key %q", name, keySequence)
	if err := ew.Accel(ctx, keySequence); err != nil {
		return errors.Wrapf(err, "could not %q scale factor", keySequence)
	}
	if err := checkBlackPixels(ctx, cr, densityDp); err != nil {
		return errors.Wrap(err, "could not check number of black pixels")
	}
	return nil
}

// checkBlackPixels grabs a screenshots and checks that number of black pixels is equal to dp*dp.
// dp refers to density independent pixel.
func checkBlackPixels(ctx context.Context, cr *chrome.Chrome, dp int) error {
	wantPixelCount := dp * dp
	// Need to wait for relayout to complete, before grabbing new screenshot.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		img, err := screenshot.GrabScreenshot(ctx, cr)
		if err != nil {
			return testing.PollBreak(err)
		}
		if gotPixelCount := screenshot.CountPixels(img, color.Black); gotPixelCount != wantPixelCount {
			return errors.Errorf("wrong number of black pixels, got: %d, want: %d", gotPixelCount, wantPixelCount)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "timed out waiting for updated screen")
	}
	return nil
}

func PerAppDensity(ctx context.Context, s *testing.State) {
	const (
		setprop        = "/system/bin/setprop"
		mainActivity   = ".MainActivity"
		packageName    = "org.chromium.arc.testapp.perappdensitytest"
		densitySetting = "persist.sys.enable_application_zoom"
		// The factors below have been taken from TaskRecordArc.
		initialDp   = 150
		increasedDp = initialDp * 1.1
		decreasedDp = initialDp * 0.9
	)

	type densityChange struct {
		// The action that should be performed to the current density.
		name string
		// The corresponding key sequence to perform the action.
		keySequence string
		// The expected dp after performing the current action.
		densityDp int
	}
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
	if err := a.Install(ctx, s.DataPath(perAppDensityApk)); err != nil {
		s.Fatal("Failed to install app: ", err)
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
	if err := act.WaitForResumed(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for the activity to resume: ", err)
	}
	if err := act.SetWindowState(ctx, arc.WindowStateFullscreen); err != nil {
		s.Fatal("Failed to set window state to Maximized: ", err)
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, packageName, ash.WindowStateFullscreen); err != nil {
		s.Fatal("Failed to wait for the activity to be Maximized: ", err)
	}
	if err := checkBlackPixels(ctx, cr, initialDp); err != nil {
		s.Fatal("Could not check initial state: ", err)
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
			increasedDp,
		},
		{
			"reset",
			"ctrl+0",
			initialDp,
		},
		{
			"decrease",
			"ctrl+-",
			decreasedDp,
		},
	} {
		if err := performAndConfirmDensityChange(ctx, cr, ew, a, test.name, test.keySequence, test.densityDp); err != nil {
			s.Fatalf("Error with performing %s: %s", test.name, err)
		}
	}
}
