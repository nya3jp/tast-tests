// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"image/color"
	"math"
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

const perAppDensityApk = "ArcPerAppDensityTest.apk"

// performAndConfirmDensityChange changes the density of the activity,
// and confirms that the density was changed by validating the size of the square on the screen.
func performAndConfirmDensityChange(ctx context.Context, cr *chrome.Chrome, ew *input.KeyboardEventWriter, a *arc.ARC, name string, keySequence string, blackPixelCount int) error {
	testing.ContextLogf(ctx, "%s density using key %q", name, keySequence)
	if err := ew.Accel(ctx, keySequence); err != nil {
		return errors.Wrapf(err, "could not change scale factor using %q", keySequence)
	}
	if err := checkBlackPixels(ctx, cr, blackPixelCount); err != nil {
		return errors.Wrap(err, "could not check number of black pixels")
	}
	return nil
}

// checkBlackPixels grabs a screenshots and checks that number of black pixels is equal to wantPixelCount.
func checkBlackPixels(ctx context.Context, cr *chrome.Chrome, wantPixelCount int) error {
	// Need to wait for relayout to complete, before grabbing new screenshot.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		img, err := screenshot.GrabScreenshot(ctx, cr)
		if err != nil {
			return testing.PollBreak(err)
		}
		gotPixelCount := screenshot.CountPixels(img, color.Black)
		diff := math.Abs(float64(wantPixelCount-gotPixelCount) / float64(wantPixelCount))
		if diff > 0.01 {
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
		packageName    = "org.chromium.arc.testapp.perappdensitytest"
		densitySetting = "persist.sys.enable_application_zoom"
		// The following scale factors have been taken from TaskRecordArc.
		increasedSF = 1.1
		decreasedSF = 0.9
		// Defined in XML files in vendor/google_arc/packages/developments/ArcPerAppDensityTest/res/layout.
		squareSidePx = 100
	)

	type densityChange struct {
		// The action that should be performed to the current density.
		name string
		// The corresponding key sequence to perform the action.
		keySequence string
		// The expected pixel count after performing the current action.
		blackPixelCount float64
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

	testing.ContextLog(ctx, "Installing app")
	if err := a.Install(ctx, s.DataPath(perAppDensityApk)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to set tablet mode to false: ", err)
	}
	defer cleanup(ctx)

	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error creating keyboard: ", err)
	}
	defer ew.Close()

	// To obtain the size of the expected black rectangle, it's necessary to obtain the dimensions of the rectangle
	// as drawn on the screen. After changing the density, we then need to multiply by the square of the new scale
	// factor (in order to account for changes to both width and height).
	disp, err := arc.NewDisplay(a, arc.DefaultDisplayID)
	if err != nil {
		s.Fatal("Failed to create new display: ", err)
	}
	displayDensity, err := disp.PhysicalDensity(ctx)
	if err != nil {
		s.Fatal("Error obtaining physical density: ", err)
	}

	expectedInitialPixelCount := (displayDensity * squareSidePx) * (displayDensity * squareSidePx)
	for _, testActivity := range []string{
		".ViewActivity",
		".SurfaceViewActivity",
	} {
		s.Run(ctx, testActivity, func(ctx context.Context, s *testing.State) {
			act, err := arc.NewActivity(a, packageName, testActivity)
			if err != nil {
				s.Fatal("Failed to create new activity: ", err)
			}
			defer act.Close()

			if err := act.Start(ctx); err != nil {
				s.Fatal("Failed to start the activity: ", err)
			}
			defer act.Stop(ctx)

			if err := act.WaitForResumed(ctx, 10*time.Second); err != nil {
				s.Fatal("Failed to wait for the activity to resume: ", err)
			}
			if err := act.SetWindowState(ctx, arc.WindowStateFullscreen); err != nil {
				s.Fatal("Failed to set window state to fullscreen: ", err)
			}
			if err := ash.WaitForARCAppWindowState(ctx, tconn, packageName, ash.WindowStateFullscreen); err != nil {
				s.Fatal("Failed to wait for the activity to be fullscreen: ", err)
			}
			if err := checkBlackPixels(ctx, cr, int(expectedInitialPixelCount)); err != nil {
				s.Fatal("Failed to check initial state: ", err)
			}

			for _, test := range []densityChange{
				{
					"increase",
					"ctrl+=",
					expectedInitialPixelCount * float64(increasedSF) * float64(increasedSF),
				},
				{
					"reset",
					"ctrl+0",
					expectedInitialPixelCount,
				},
				{
					"decrease",
					"ctrl+-",
					expectedInitialPixelCount * float64(decreasedSF) * float64(decreasedSF),
				},
			} {
				// Return density to initial state.
				defer func() {
					if err := performAndConfirmDensityChange(ctx, cr, ew, a, "reset", "ctrl+0", int(expectedInitialPixelCount)); err != nil {
						s.Fatalf("Error with performing %s: %s", test.name, err)
					}
				}()
				if err := performAndConfirmDensityChange(ctx, cr, ew, a, test.name, test.keySequence, int(test.blackPixelCount)); err != nil {
					s.Fatalf("Error with performing %s: %s", test.name, err)
				}
			}
		})
	}
}
