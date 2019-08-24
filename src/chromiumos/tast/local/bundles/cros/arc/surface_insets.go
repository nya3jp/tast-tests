// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SurfaceInsets,
		Desc:         "Test to handle SurfaceInsets not to exceed android window frame",
		Contacts:     []string{"hirokisato@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcSurfaceInsetsTestApp.apk"},
		Pre:          arc.Booted(),
	})
}

func SurfaceInsets(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcSurfaceInsetsTestApp.apk"
		pkg = "org.chromium.arc.testapp.surfaceinsets"
		cls = ".MainActivity"
	)

	// Prepare TouchScreen.
	// Touchscreen bounds: The size of the touchscreen might not be the same
	// as the display size.
	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to open touchscreen device: ", err)
	}
	defer tsw.Close()

	touchWidth := float64(tsw.Width())
	touchHeight := float64(tsw.Height())

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Could not get a new TouchEventWriter: ", err)
	}
	defer stw.Close()

	// Prepare ARC.
	a := s.PreValue().(arc.PreData).ARC
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}

	if err := act.SetWindowState(ctx, arc.WindowStateNormal); err != nil {
		s.Fatal("Failed to set window state to Normal: ", err)
	}

	disp, err := arc.NewDisplay(a, arc.DefaultDisplayID)
	if err != nil {
		s.Fatal("Failed to obtain a default display: ", err)
	}

	arcDisplaySize, err := disp.Size(ctx)
	if err != nil {
		s.Fatal("Failed to get arc size: ", err)
	}

	arcCaptionHeight, err := disp.CaptionHeight(ctx)
	if err != nil {
		s.Fatal("Failed to get arc size: ", err)
	}

	arcPixelToTouchFactorW := touchWidth / float64(arcDisplaySize.W)
	arcPixelToTouchFactorH := touchHeight / float64(arcDisplaySize.H)

	// Repeat to validate b/80441010.
	for _, op := range []string{"maximize", "restore", "maximize", "restore"} {
		s.Logf("Pressing %q", op)

		activityBounds, err := act.WindowBounds(ctx)
		if err != nil {
			s.Fatal("Failed to get window bounds: ", err)
		}

		initialState, err := act.GetWindowState(ctx)
		if err != nil {
			s.Fatal("Failed to get window state: ", err)
		}

		// Touch lower edge of the restore/maximize button to validate that any surface does not cover the caption button.
		// TODO(hirokisato) : Do not hard code the calculation of caption position below.
		// Instead, we should get Chrome constants in real time.
		buttonCoordX := float64(activityBounds.Left+activityBounds.Width-arcCaptionHeight/2-arcCaptionHeight) * arcPixelToTouchFactorW
		buttonCoordY := float64(activityBounds.Top+arcCaptionHeight) * arcPixelToTouchFactorH

		stw.Move(input.TouchCoord(buttonCoordX), input.TouchCoord(buttonCoordY))
		stw.End()

		err = testing.Poll(ctx, func(ctx context.Context) error {
			newState, err := act.GetWindowState(ctx)
			if err != nil {
				s.Fatal("Failed to get window state: ", err)
			}
			if newState == initialState {
				return errors.New("state has not changed yet")
			}
			return nil
		}, &testing.PollOptions{Timeout: 4 * time.Second})

		if err != nil {
			s.Fatalf("Pressing %q did not work. Tried to touch (%f, %f), state = %q", op, buttonCoordX, buttonCoordY, initialState.String())
		}
	}
}
