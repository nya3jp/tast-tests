// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputCompat,
		Desc:         "Checks input compatibility for M and games working",
		Contacts:     []string{"tetsui@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"informational", "group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func InputCompat(ctx context.Context, s *testing.State) {
	tpw, err := input.Trackpad(ctx)
	if err != nil {
		s.Fatal("Failed to create a trackpad device: ", err)
	}
	defer tpw.Close()

	tw, err := tpw.NewMultiTouchWriter(2)
	if err != nil {
		s.Fatal("Failed to create a multi touch writer: ", err)
	}
	defer tw.Close()

	p := s.FixtValue().(*arc.PreData)
	cr := p.Chrome
	a := p.ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display info: ", err)
	}
	if len(infos) == 0 {
		s.Fatal("No display found")
	}
	var info *display.Info
	for i := range infos {
		if infos[i].IsInternal {
			info = &infos[i]
		}
	}
	if info == nil {
		s.Log("No internal display found. Default to the first display")
		info = &infos[0]
	}

	const (
		pkg = "org.chromium.arc.testapp.inputcompat"
		cls = ".MainActivity"

		sourceMouse       = 0x2002 // The value of InputDevice.SOURCE_MOUSE
		sourceTouchscreen = 0x1002 // The value of InputDevice.SOURCE_TOUCHSCREEN

		numPointersID = pkg + ":id/num_pointers"
		inputSourceID = pkg + ":id/input_source"
		isScrollingID = pkg + ":id/is_scrolling"
	)

	expectEvent := func(ctx context.Context, isScrolling bool, inputSource, numPointers int) error {
		if err := d.Object(ui.ID(isScrollingID), ui.Text(strconv.FormatBool(isScrolling))).WaitForExists(ctx, 30*time.Second); err != nil {
			if isScrolling {
				return errors.Wrap(err, "expected a scrolling event")
			}
			return errors.Wrap(err, "expected a non-scrolling event")
		}
		if err := d.Object(ui.ID(inputSourceID), ui.Text(strconv.Itoa(inputSource))).WaitForExists(ctx, 30*time.Second); err != nil {
			actual, err := d.Object(ui.ID(inputSourceID)).GetText(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to get input source")
			}
			return errors.Errorf("wrong input source: got %s, want %d", actual, inputSource)
		}
		if err := d.Object(ui.ID(numPointersID), ui.Text(strconv.Itoa(numPointers))).WaitForExists(ctx, 30*time.Second); err != nil {
			actual, err := d.Object(ui.ID(numPointersID)).GetText(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to get the number of pointers")
			}
			return errors.Errorf("wrong number of pointers: got %s, want %d", actual, numPointers)
		}
		return nil
	}

	// Perform two finger scrolling on the trackpad
	doTrackpadScroll := func(ctx context.Context) error {
		x0 := tpw.Width() / 2
		y0 := tpw.Height() / 4
		x1 := tpw.Width() / 2
		y1 := tpw.Height() / 4 * 3
		d := tpw.Width() / 8 // x-axis distance between two fingers
		const t = time.Second
		return tw.DoubleSwipe(ctx, x0, y0, x1, y1, d, t)
	}

	type compatSettings struct {
		Name                    string
		Apk                     string
		InputSource             int
		NumPointersDuringScroll int
		InputSourceDuringScroll int
	}

	runTest := func(ctx context.Context, s *testing.State, settings *compatSettings) {
		s.Log("Installing apk ", settings.Apk)
		if err := a.Install(ctx, arc.APKPath(settings.Apk)); err != nil {
			s.Fatalf("Failed installing %s: %v", settings.Apk, err)
		}

		act, err := arc.NewActivity(a, pkg, cls)
		if err != nil {
			s.Fatal("Failed to create an activity: ", err)
		}
		defer act.Close()

		if err := act.Start(ctx, tconn); err != nil {
			s.Fatal("Failed to start an activity: ", err)
		}
		defer act.Stop(ctx, tconn)

		tabletMode, err := ash.TabletModeEnabled(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get tablet mode: ", err)
		}
		if !tabletMode {
			if err := act.SetWindowState(ctx, tconn, arc.WindowStateMaximized); err != nil {
				s.Fatal("Failed to set the window state to maximized: ", err)
			}
		}

		if err := mouse.Move(ctx, tconn, coords.Point{X: 0, Y: 0}, 0); err != nil {
			s.Fatal("Failed to move mouse: ", err)
		}
		center := info.Bounds.CenterPoint()
		if err := mouse.Move(ctx, tconn, center, 200*time.Millisecond); err != nil {
			s.Fatal("Failed to move mouse: ", err)
		}

		if err := expectEvent(ctx, false, settings.InputSource, 1); err != nil {
			s.Fatal("Failed to receive the expected event: ", err)
		}

		// Skip testing trackpad scroll for tablet-mode devices.
		if !tabletMode {
			if err := doTrackpadScroll(ctx); err != nil {
				s.Fatal("Failed to perform two finger scroll: ", err)
			}

			if err := expectEvent(ctx, true, settings.InputSourceDuringScroll, settings.NumPointersDuringScroll); err != nil {
				s.Fatal("Failed to receive the expected event: ", err)
			}
		}

		if err := tw.End(); err != nil {
			s.Fatal("Failed to finish trackpad scroll: ", err)
		}
	}

	for _, settings := range []compatSettings{
		{
			Name:                    "InputCompatDisabled",
			Apk:                     "ArcInputCompatDisabledTest.apk",
			InputSource:             sourceMouse,
			NumPointersDuringScroll: 1,
			InputSourceDuringScroll: sourceMouse,
		},
		{
			Name:                    "InputCompatGame",
			Apk:                     "ArcInputCompatGameTest.apk",
			InputSource:             sourceTouchscreen,
			NumPointersDuringScroll: 1,
			InputSourceDuringScroll: sourceTouchscreen,
		},
		{
			Name:                    "InputCompatM",
			Apk:                     "ArcInputCompatMTest.apk",
			InputSource:             sourceMouse,
			NumPointersDuringScroll: 2,
			InputSourceDuringScroll: sourceTouchscreen,
		},
	} {
		s.Run(ctx, settings.Name, func(ctx context.Context, s *testing.State) {
			runTest(ctx, s, &settings)
		})
	}
}
