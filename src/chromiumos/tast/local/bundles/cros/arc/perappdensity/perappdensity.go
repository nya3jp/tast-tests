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

	"chromiumos/tast/ctxutil"
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
	// Setprop is the path for setprop command.
	Setprop = "/system/bin/setprop"
	// DensitySetting is the settings string for allowing density changes.
	DensitySetting = "persist.sys.enable_application_zoom"
	// DensityApk is the name of the apk used in these tests.
	DensityApk = "ArcPerAppDensityTest.apk"
	// DensityPackageName is the name of density activity.
	DensityPackageName = "org.chromium.arc.testapp.perappdensitytest"
)

// DensityChange is a struct containing information to perform density changes.
type DensityChange struct {
	// The action that should be performed to the current density.
	Name string
	// The corresponding key sequence to perform the action.
	KeySequence string
	// The expected pixel count after performing the current action.
	BlackPixelCount float64
}

// ExecuteChange executes the density change, specified by KeySequence
// and confirms that the density was changed by validating the size of the square on the screen.
func (dc *DensityChange) ExecuteChange(ctx context.Context, cr *chrome.Chrome, ew *input.KeyboardEventWriter) error {
	testing.ContextLogf(ctx, "%s density using key %q", dc.Name, dc.KeySequence)
	if err := ew.Accel(ctx, dc.KeySequence); err != nil {
		return errors.Wrapf(err, "could not change scale factor using %q", dc.KeySequence)
	}
	if err := ConfirmBlackPixelCount(ctx, cr, int(dc.BlackPixelCount)); err != nil {
		return errors.Wrap(err, "could not check number of black pixels")
	}
	return nil
}

// CountBlackPixels grabs a screenshot and counts the number of black pixels.
func CountBlackPixels(ctx context.Context, cr *chrome.Chrome) (int, error) {
	img, err := screenshot.GrabScreenshot(ctx, cr)
	if err != nil {
		return 0, err
	}
	return screenshot.CountPixels(img, color.Black), nil
}

// ConfirmBlackPixelCount grabs a screenshot and checks that number of black pixels is equal to wantPixelCount.
func ConfirmBlackPixelCount(ctx context.Context, cr *chrome.Chrome, wantPixelCount int) error {
	// Need to wait for relayout to complete, before grabbing new screenshot.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		gotPixelCount, err := CountBlackPixels(ctx, cr)
		if err != nil {
			testing.PollBreak(err)
		}
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

// MeasureDisplayDensity initializes the display and returns its physical density.
func MeasureDisplayDensity(ctx context.Context, a *arc.ARC) (float64, error) {
	// To obtain the size of the expected black rectangle, it's necessary to obtain the dimensions of the rectangle
	// as drawn on the screen. After changing the density, we then need to multiply by the square of the new scale
	// factor (in order to account for changes to both width and height).
	disp, err := arc.NewDisplay(a, arc.DefaultDisplayID)
	if err != nil {
		return -1, errors.Wrap(err, "failed to create new display")
	}

	displayDensity, err := disp.PhysicalDensity(ctx)
	if err != nil {
		return -1, errors.Wrap(err, "error obtaining physical density")
	}
	return displayDensity, nil
}

// SetUpApk enables developer option for density changes, and installs the specified apk.
func SetUpApk(ctx context.Context, a *arc.ARC, apk string) error {
	if err := arc.BootstrapCommand(ctx, Setprop, DensitySetting, "true").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to set developer option")
	}
	testing.ContextLog(ctx, "Installing app")
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		return errors.Wrap(err, "failed to install the APK")
	}
	return nil
}

// StartDensityActivityWithWindowState starts the density activity with the specified window state.
// It is the callers responsibility to close the activity.
func StartDensityActivityWithWindowState(ctx context.Context, a *arc.ARC, tconn *chrome.TestConn, cr *chrome.Chrome, windowState arc.WindowState) (*arc.Activity, error) {
	cleanupTime := 10 * time.Second // time reserved for cleanup.

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set tablet mode to false")
	}
	defer func(ctx context.Context) {
		cleanup(ctx)
	}(ctx)
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	act, err := arc.NewActivity(a, DensityPackageName, ".ViewActivity")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new activity")
	}
	//defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to start the activity")
	}
	//	defer act.Stop(ctx, tconn)

	if err := ash.WaitForVisible(ctx, tconn, DensityPackageName); err != nil {
		return nil, errors.Wrap(err, "failed to wait for visible app")
	}

	if err := act.SetWindowState(ctx, tconn, windowState); err != nil {
		return nil, errors.Wrap(err, "failed to set window state to normal")
	}

	ashWindowState, err := windowState.ToAshWindowState()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get ash window state")
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, DensityPackageName, ashWindowState); err != nil {
		return nil, errors.Wrapf(err, "failed to wait for the activity to have required window state %q", windowState.String())
	}

	return act, nil
}

// EnableUSFAndConfirmPixelState enables USF, and then confirms the state afterwards.
func EnableUSFAndConfirmPixelState(ctx context.Context, a *arc.ARC, tconn *chrome.TestConn, cr *chrome.Chrome, windowState arc.WindowState, defaultPixelCount int) error {
	const (
		uniformScaleFactor        = 1.25
		uniformScaleFactorSetting = "persist.sys.ui.uniform_app_scaling"
	)
	if err := arc.BootstrapCommand(ctx, Setprop, uniformScaleFactorSetting, "1").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to set developer option")
	}

	expectedPixelCount := (int)((float64)(defaultPixelCount) * uniformScaleFactor * uniformScaleFactor)

	testing.ContextLog(ctx, "Running app, with uniform scaling enabled")
	act, err := StartDensityActivityWithWindowState(ctx, a, tconn, cr, windowState)
	if err != nil {
		return errors.Wrap(err, "failed to start activity after enabling uniform scale factor")
	}
	defer act.Close()

	if err := ConfirmBlackPixelCount(ctx, cr, expectedPixelCount); err != nil {
		return errors.Wrap(err, "failed to verify uniform scale factor state")
	}
	return nil
}

// RunTest takes a slice of activity names and a slice of density changes and executes them.
func RunTest(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, packageName string, testSteps []DensityChange, activity string, expectedInitialPixelCount float64) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "error creating keyboard")
	}
	defer ew.Close()

	act, err := arc.NewActivity(a, packageName, activity)
	if err != nil {
		return errors.Wrap(err, "failed to create new activity")
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start the activity")
	}
	defer act.Stop(ctx, tconn)

	if err := ash.WaitForVisible(ctx, tconn, packageName); err != nil {
		return errors.Wrap(err, "failed to wait for visible app")
	}

	if err := act.SetWindowState(ctx, tconn, arc.WindowStateFullscreen); err != nil {
		return errors.Wrap(err, "failed to set window state to fullscreen")
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, packageName, ash.WindowStateFullscreen); err != nil {
		return errors.Wrap(err, "failed to wait for the activity to be fullscreen")
	}

	if err := ConfirmBlackPixelCount(ctx, cr, int(expectedInitialPixelCount)); err != nil {
		return errors.Wrap(err, "failed to check initial state: ")
	}

	for _, testStep := range testSteps {
		if err := testStep.ExecuteChange(ctx, cr, ew); err != nil {
			return errors.Wrapf(err, "failed performing %q on %q", testStep.Name, activity)
		}
	}

	// Ensure that density is restored to initial state.
	defer func() error {
		initialState := DensityChange{"reset", "ctrl+0", expectedInitialPixelCount}

		if err := initialState.ExecuteChange(ctx, cr, ew); err != nil {
			return errors.Wrap(err, "error with restoring initial state: ")
		}
		return nil
	}()
	return nil
}
