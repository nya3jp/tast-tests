// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package perappdensity provides functions to assist with perappdensity tast tests.
package perappdensity

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

const (
	setprop        = "/system/bin/setprop"
	densitySetting = "persist.sys.enable_application_zoom"
)

// DensityChange is a struct containing information to perform a density change.
type DensityChange struct {
	// The action that should be performed to the current density.
	Name string
	// The corresponding key sequence to perform the action.
	KeySequence string
	// The expected pixel count after performing the current action.
	BlackPixelCount float64
}

// PerformAndConfirmDensityChange changes the density of the activity,
// and confirms that the density was changed by validating the size of the square on the screen.
func PerformAndConfirmDensityChange(ctx context.Context, cr *chrome.Chrome, ew *input.KeyboardEventWriter, a *arc.ARC, name, keySequence string, blackPixelCount int) error {
	testing.ContextLogf(ctx, "%s density using key %q", name, keySequence)
	if err := ew.Accel(ctx, keySequence); err != nil {
		return errors.Wrapf(err, "could not change scale factor using %q", keySequence)
	}
	if err := CheckBlackPixels(ctx, cr, blackPixelCount); err != nil {
		return errors.Wrap(err, "could not check number of black pixels")
	}
	return nil
}

// CheckBlackPixels grabs a screenshots and checks that number of black pixels is equal to wantPixelCount.
func CheckBlackPixels(ctx context.Context, cr *chrome.Chrome, wantPixelCount int) error {
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

// RunTest installs the application, and runs activities to perform perappdensity test.
func RunTest(ctx context.Context, s *testing.State, apkName, packageName string, activityNames []string, f func(context.Context, *chrome.Chrome, *arc.ARC, *input.KeyboardEventWriter, string, float64) error) {
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
	if err := a.Install(ctx, arc.APKPath(apkName)); err != nil {
		s.Fatal("Failed to install the APK: ", err)
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
	for _, activityName := range activityNames {
		s.Run(ctx, activityName, func(ctx context.Context, s *testing.State) {
			act, err := arc.NewActivity(a, packageName, activityName)
			if err != nil {
				s.Fatal("Failed to create new activity: ", err)
			}
			defer act.Close()

			if err := act.Start(ctx, tconn); err != nil {
				s.Fatal("Failed to start the activity: ", err)
			}
			defer act.Stop(ctx)

			if err := act.SetWindowState(ctx, arc.WindowStateFullscreen); err != nil {
				s.Fatal("Failed to set window state to fullscreen: ", err)
			}
			if err := ash.WaitForARCAppWindowState(ctx, tconn, packageName, ash.WindowStateFullscreen); err != nil {
				s.Fatal("Failed to wait for the activity to be fullscreen: ", err)
			}
			if err := f(ctx, cr, a, ew, activityName, displayDensity); err != nil {
				/*path := filepath.Join(s.OutDir(), "screenshot.png")
				if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
					s.Log("Failed to capture screenshot: ", err)
				}*/
				s.Fatal("Failed to run the test: ", err)
			}
		})
	}
}
