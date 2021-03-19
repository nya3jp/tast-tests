// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"image"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIPTZ,
		Desc:         "Opens CCA and verifies the PTZ functionality",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome"},
		Fixture:      "ccaLaunchedWithPTZScene",
	})
}

// findPattern finds the region where the pattern resides.
func findPattern(ctx context.Context, app *cca.App) (*image.Rectangle, error) {
	frame, err := app.PreviewFrame(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get preview frame")
	}
	defer frame.Release(ctx)

	// Find the coordinates of top-left corner.
	minPt, err := frame.Find(ctx, &cca.FirstBlack)
	if err != nil {
		return nil, err
	}

	// Find the coordinates of bottom-right corner.
	maxPt, err := frame.Find(ctx, &cca.LastBlack)
	if err != nil {
		return nil, err
	}

	return &image.Rectangle{*minPt, *maxPt}, nil
}

// calShift checks the size and calculates the x-y shift between |r| and |r2|.
func calShift(r, r2 *image.Rectangle) (*image.Point, error) {
	abs := func(n int) int {
		if n < 0 {
			return -n
		}
		return n
	}

	// The pattern should only shift without resizing.
	sz := r.Size()
	sz2 := r2.Size()

	// Do all comparisons with 1px precision tolerance introduced by fake
	// file VCD bilinear resizing implementation.
	const precision = 1
	if abs(sz.X-sz2.X) > precision {
		return nil, errors.Errorf("inconsistent width, got %v; want %v", sz2.X, sz.X)
	}
	if abs(sz.Y-sz2.Y) > precision {
		return nil, errors.Errorf("inconsistent height, got %v; want %v", sz2.Y, sz.Y)
	}
	return &image.Point{r2.Min.X - r.Min.X, r2.Min.Y - r.Min.Y}, nil
}

type ptzControl struct {
	// ui is the UI toggled for moving preview in one of PTZ direction.
	ui *cca.UIComponent
	// testFunc tests pattern before and after ptz control applied moving in the target direction.
	testFunc func(r, r2 *image.Rectangle) (bool, error)
}

var (
	panLeft = ptzControl{&cca.PanLeftButton, func(r, r2 *image.Rectangle) (bool, error) {
		shift, err := calShift(r, r2)
		if err != nil {
			return false, err
		}
		return shift.X < 0 && shift.Y == 0, nil
	}}
	panRight = ptzControl{&cca.PanRightButton, func(r, r2 *image.Rectangle) (bool, error) {
		shift, err := calShift(r, r2)
		if err != nil {
			return false, err
		}
		return shift.X > 0 && shift.Y == 0, nil
	}}
	tiltDown = ptzControl{&cca.TiltDownButton, func(r, r2 *image.Rectangle) (bool, error) {
		shift, err := calShift(r, r2)
		if err != nil {
			return false, err
		}
		return shift.X == 0 && shift.Y < 0, nil
	}}
	tiltUp = ptzControl{&cca.TiltUpButton, func(r, r2 *image.Rectangle) (bool, error) {
		shift, err := calShift(r, r2)
		if err != nil {
			return false, err
		}
		return shift.X == 0 && shift.Y > 0, nil
	}}
	zoomIn = ptzControl{&cca.ZoomInButton, func(r, r2 *image.Rectangle) (bool, error) {
		return r.Size().X < r2.Size().X && r.Size().Y < r2.Size().Y, nil
	}}
	zoomOut = ptzControl{&cca.ZoomOutButton, func(r, r2 *image.Rectangle) (bool, error) {
		return r.Size().X > r2.Size().X && r.Size().Y > r2.Size().Y, nil
	}}
)

// testToggle tests toggling the control |ctrl|.
func (ctrl *ptzControl) testToggle(ctx context.Context, app *cca.App) error {
	pRect, err := findPattern(ctx, app)
	if err != nil {
		return errors.Wrapf(err, "failed to find pattern before clicking %v: %v", ctrl.ui.Name, err)
	}
	if err := app.ClickPTZButton(ctx, *ctrl.ui); err != nil {
		return errors.Wrapf(err, "failed to click: %v", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		rect, err := findPattern(ctx, app)
		if err != nil {
			return errors.Wrapf(err, "failed to find pattern after clicking %v: %v", ctrl.ui.Name, err)
		}
		result, err := ctrl.testFunc(pRect, rect)
		if err != nil {
			return testing.PollBreak(err)
		}
		if result {
			return nil
		}
		return errors.Errorf("failed on testing UI with region before %v ; after %v", pRect, rect)
	}, &testing.PollOptions{Interval: time.Second}); err != nil {
		return errors.Wrapf(err, "failed to run %v test func: %v", ctrl.ui.Name, err)
	}
	return nil
}

func CCAUIPTZ(ctx context.Context, s *testing.State) {
	app := s.FixtValue().(cca.FixtureData).App()
	if err := app.Click(ctx, cca.OpenPTZPanelButton); err != nil {
		s.Fatal("Failed to open ptz panel: ", err)
	}

	// Check cannot pan/tilt when zoom at initial level 0.
	for _, control := range []ptzControl{
		zoomOut,
		panLeft,
		panRight,
		tiltUp,
		tiltDown,
	} {
		disabled, err := app.Disabled(ctx, *control.ui)
		if err != nil {
			s.Fatalf("Failed to get disabled state of %v: %v", control.ui.Name, err)
		}
		if !disabled {
			s.Fatalf("UI %v is not disabled at initial zoom level", control.ui.Name)
		}
	}

	// Test move all controls. The controls need to be tested in order such
	// that |zoomIn| before all other controls(For all other controls will
	// be disabled in minimal zoom level as behavior of digital zoom
	// camera), |panLeft| before |panRight| (For the initial pan level is 0
	// with range [0, 15]) with initial mirror state, |tiltDown| before
	// |tiltUp| (For the initial tilt level is 0 with range[0, 8]).
	for _, control := range []ptzControl{
		zoomIn,
		panLeft,
		panRight,
		tiltDown,
		tiltUp,
		zoomOut,
	} {
		if err := control.testToggle(ctx, app); err != nil {
			s.Fatal("Failed: ", err)
		}
	}

	// Check cannot pan/tilt when zoom reset to initial level 0.
	if err := app.Click(ctx, cca.PTZResetAllButton); err != nil {
		s.Fatal("Failed to reset ptz: ", err)
	}
	for _, control := range []ptzControl{
		zoomOut,
		panLeft,
		panRight,
		tiltUp,
		tiltDown,
	} {
		if err := app.WaitForDisabled(ctx, *control.ui, true); err != nil {
			s.Fatalf("Failed to wait for ui %v disabled : %v", control.ui.Name, err)
		}
	}
}
