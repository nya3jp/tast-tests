// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package perappdensity provides functions to assist with perappdensity tast tests.
package perappdensity

import (
	"context"
	"image"
	"image/color"
	"math"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/screen"
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
	// Setting is the settings string for allowing density changes.
	Setting = "persist.sys.enable_application_zoom"
	// Apk is the name of the apk used in these tests.
	Apk = "ArcPerAppDensityTest.apk"
	// PackageName is the name of density application.
	PackageName = "org.chromium.arc.testapp.perappdensitytest"
	// ViewActivity is the name of view (main) activity.
	ViewActivity = ".ViewActivity"
)

// Change is a struct containing information to perform density changes.
type Change struct {
	// The action that should be performed to the current density.
	Name string
	// The corresponding key sequence to perform the action.
	KeySequence string
	// The expected pixel count after performing the current action.
	BlackPixelCount float64
}

// Execute executes the density change, specified by KeySequence
// and confirms that the density was changed by validating the size of the square on the screen.
func (dc *Change) Execute(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, ew *input.KeyboardEventWriter) error {
	testing.ContextLogf(ctx, "%s density using key %q", dc.Name, dc.KeySequence)
	if err := ew.Accel(ctx, dc.KeySequence); err != nil {
		return errors.Wrapf(err, "could not change scale factor using %q", dc.KeySequence)
	}

	if err := confirmPixelCount(ctx, cr, a, int(dc.BlackPixelCount), screenshot.GrabScreenshot, color.Black); err != nil {
		return errors.Wrap(err, "could not check number of black pixels")
	}
	return nil
}

func confirmPixelCount(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, wantPixelCount int, grabScreenshot func(context.Context, *chrome.Chrome) (image.Image, error), clr color.Color) error {
	// Need to wait for relayout to complete, before grabbing new screenshot.
	if err := screen.WaitForStableFrames(ctx, a, PackageName); err != nil {
		return errors.Wrap(err, "failed waiting for updated frames")
	}
	img, err := grabScreenshot(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to grab screenshot")
	}
	n := screenshot.CountPixels(img, clr)
	diff := math.Abs(float64(wantPixelCount-n) / float64(wantPixelCount))
	// Allow a small epsilon as wantPixelCount is computed as a float.
	if diff > 0.01 {
		return errors.Errorf("wrong number of black pixels, got: %d, want: %d", n, wantPixelCount)
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
	if err := arc.BootstrapCommand(ctx, Setprop, Setting, "true").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to set developer option")
	}
	testing.ContextLog(ctx, "Installing app")
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		return errors.Wrap(err, "failed to install the APK")
	}
	return nil
}

// StartActivityWithWindowState starts the view activity with the specified window state.
// It is the responsibility of the caller to close the activity.
func StartActivityWithWindowState(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, windowState arc.WindowState, activity string) (*arc.Activity, error) {
	act, err := arc.NewActivity(a, PackageName, activity)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new activity")
	}

	if err := act.Start(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to start the activity")
	}

	if err := ash.WaitForVisible(ctx, tconn, PackageName); err != nil {
		return nil, errors.Wrap(err, "failed to wait for visible app")
	}

	if err := act.SetWindowState(ctx, tconn, windowState); err != nil {
		return nil, errors.Wrap(err, "failed to set window state to normal")
	}

	ashWindowState, err := windowState.ToAshWindowState()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get ash window state")
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, PackageName, ashWindowState); err != nil {
		return nil, errors.Wrapf(err, "failed to wait for the activity to have required window state %q", windowState)
	}

	return act, nil
}

// VerifyPixelsWithUSFEnabled enables Uniform Scale Factor(USF), which applies a scale factor of 1.25. It then confirms that the
// 1.25 scaling has been correctly applied by checking the number of pixels drawn.
// nonScaledPixelCount is the size of the drawn square, before USF is applied. This value is used to compute the expected of the
// drawn square.
func VerifyPixelsWithUSFEnabled(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, a *arc.ARC, windowState arc.WindowState, nonScaledPixelCount int, clr color.Color) error {
	const (
		// Uniform scale factor applies 1.25 scaling.
		uniformScaleFactor        = 1.25
		uniformScaleFactorSetting = "persist.sys.ui.uniform_app_scaling"
		cleanupTime               = 10 * time.Second // time reserved for cleanup.
	)
	if err := arc.BootstrapCommand(ctx, Setprop, uniformScaleFactorSetting, "1").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to set developer option")
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		return errors.Wrap(err, "failed to set tablet mode to false")
	}
	defer cleanup(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	testing.ContextLog(ctx, "Running app, with uniform scaling enabled")
	act, err := StartActivityWithWindowState(ctx, tconn, a, windowState, ViewActivity)
	if err != nil {
		return errors.Wrap(err, "failed to start activity after enabling uniform scale factor")
	}
	defer act.Close()

	// Multiply each side of the drawn square by uniform scale factor.
	wantPixelCount := (int)((float64)(nonScaledPixelCount) * uniformScaleFactor * uniformScaleFactor)

	bounds, err := act.SurfaceBounds(ctx)
	if err != nil {
		return err
	}
	grabScreenshot := func(ctx context.Context, cr *chrome.Chrome) (image.Image, error) {
		return screenshot.GrabAndCropScreenshot(ctx, cr, bounds)
	}
	if err := confirmPixelCount(ctx, cr, a, wantPixelCount, grabScreenshot, clr); err != nil {
		return errors.Wrap(err, "failed to verify uniform scale factor state")
	}
	return nil
}

// RunTest takes a slice of activity names and a slice of density changes and executes them.
func RunTest(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, packageName string, testSteps []Change, activity string, expectedInitialPixelCount float64) error {
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

	if err := confirmPixelCount(ctx, cr, a, int(expectedInitialPixelCount), screenshot.GrabScreenshot, color.Black); err != nil {
		return errors.Wrap(err, "failed to check initial state: ")
	}

	// Ensure that density is restored to initial state.
	defer func() {
		initialState := Change{"reset", "ctrl+0", expectedInitialPixelCount}

		if err := initialState.Execute(ctx, cr, a, ew); err != nil {
			testing.ContextLog(ctx, "Failed to restore initial state: ", err)
		}
	}()

	for _, testStep := range testSteps {
		if err := testStep.Execute(ctx, cr, a, ew); err != nil {
			return errors.Wrapf(err, "failed performing %q on %q", testStep.Name, activity)
		}
	}

	return nil
}
