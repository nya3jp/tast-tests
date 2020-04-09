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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	// Setprop is the path for setprop command.
	Setprop = "/system/bin/setprop"
	// DensitySetting is the settings string for allowing density changes.
	DensitySetting = "persist.sys.enable_application_zoom"
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
	if err := CountBlackPixels(ctx, cr, int(dc.BlackPixelCount)); err != nil {
		return errors.Wrap(err, "could not check number of black pixels")
	}
	return nil
}

// CountBlackPixels grabs a screenshots and checks that number of black pixels is equal to wantPixelCount.
func CountBlackPixels(ctx context.Context, cr *chrome.Chrome, wantPixelCount int) error {
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

// InitDisplay initializes the display and returns its physical density.
func InitDisplay(ctx context.Context, s *testing.State) (float64, error) {
	a := s.PreValue().(arc.PreData).ARC
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

// RunTest takes a slice of activity names, and slice of density changes and executes them.
func RunTest(ctx context.Context, s *testing.State, packageName string, testSteps []DensityChange, activities []string, expectedInitialPixelCount float64) error {
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "error creating keyboard")
	}
	defer ew.Close()

	// Start each activity, and execute the density changes for each activity.
	for _, activity := range activities {
		testing.ContextLogf(ctx, "Running %q", activity)

		act, err := arc.NewActivity(a, packageName, activity)
		if err != nil {
			return errors.Wrap(err, "failed to create new activity")
		}
		defer act.Close()

		if err := act.Start(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to start the activity")
		}
		defer act.Stop(ctx)

		if err := CountBlackPixels(ctx, cr, int(expectedInitialPixelCount)); err != nil {
			return errors.Wrap(err, "failed to check initial state: ")
		}

		for _, testStep := range testSteps {
			if err := testStep.ExecuteChange(ctx, cr, ew); err != nil {
				return errors.Wrapf(err, "failed performing %q on %q", testStep.Name, activity)
			}
		}
	}
	return nil
}
