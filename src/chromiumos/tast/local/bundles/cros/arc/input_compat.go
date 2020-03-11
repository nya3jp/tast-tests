// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputCompat,
		Desc:         "Checks input compatibility for M and games working",
		Contacts:     []string{"tetsui@chromium.org", "arc-framework@google.com"},
		Attr:         []string{"informational", "group:mainline"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Pre:          arc.Booted(),
		Timeout:      3 * time.Minute,
	})
}

func InputCompat(ctx context.Context, s *testing.State) {
	p := s.PreValue().(arc.PreData)
	cr := p.Chrome
	a := p.ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

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

		sourceMouse       = 0x2002
		sourceTouchscreen = 0x1002

		numPointersID = pkg + ":id/num_pointers"
		inputSourceID = pkg + ":id/input_source"
	)

	type CompatSettings struct {
		Name        string
		Apk         string
		InputSource int
	}

	runTest := func(ctx context.Context, s *testing.State, settings *CompatSettings) {
		s.Log("Installing apk ", settings.Apk)
		if err := a.Install(ctx, arc.APKPath(settings.Apk)); err != nil {
			s.Fatalf("Failed installing %s: %v", settings.Apk, err)
		}

		act, err := arc.NewActivity(a, pkg, cls)
		if err != nil {
			s.Fatal("Failed to create an activity: ", err)
		}
		defer act.Close()

		if err := act.Start(ctx); err != nil {
			s.Fatal("Failed to start an activity: ", err)
		}
		defer act.Stop(ctx)

		tabletMode, err := ash.TabletModeEnabled(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get tablet mode: ", err)
		}
		if !tabletMode {
			if err := act.SetWindowState(ctx, arc.WindowStateMaximized); err != nil {
				s.Fatal("Failed to set the window state to maximized: ", err)
			}
		}

		if err := ash.MouseMove(ctx, tconn, coords.Point{0, 0}, 0); err != nil {
			s.Fatal("Failed to move mouse: ", err)
		}
		center := info.Bounds.CenterPoint()
		if err := ash.MouseMove(ctx, tconn, center, 200*time.Millisecond); err != nil {
			s.Fatal("Failed to move mouse: ", err)
		}

		if err := d.Object(ui.ID(inputSourceID), ui.Text(strconv.Itoa(settings.InputSource))).WaitForExists(ctx, 30*time.Second); err != nil {
			if actual, err := d.Object(ui.ID(inputSourceID)).GetText(ctx); err != nil {
				s.Fatal("Failed to get input source: ", err)
			} else {
				s.Fatalf("Wrong input source: got %s, want %d", actual, settings.InputSource)
			}
		}
		if err := d.Object(ui.ID(numPointersID), ui.Text("1")).WaitForExists(ctx, 30*time.Second); err != nil {
			if actual, err := d.Object(ui.ID(numPointersID)).GetText(ctx); err != nil {
				s.Fatal("Failed to get the number of pointers: ", err)
			} else {
				s.Fatalf("Wrong number of pointers: got %s, want 1", actual)
			}
		}

		// TODO(tetsui): Implement a virtual trackpad device and test scroll gesture
		// In MNC apps, scroll gesture should have two pointers.
	}

	for _, settings := range []CompatSettings{
		{
			Name:        "InputCompatDisabled",
			Apk:         "ArcInputCompatDisabledTest.apk",
			InputSource: sourceMouse,
		},
		{
			Name:        "InputCompatGame",
			Apk:         "ArcInputCompatGameTest.apk",
			InputSource: sourceTouchscreen,
		},
		{
			Name:        "InputCompatM",
			Apk:         "ArcInputCompatMTest.apk",
			InputSource: sourceMouse,
		},
	} {
		s.Run(ctx, settings.Name, func(ctx context.Context, s *testing.State) {
			runTest(ctx, s, &settings)
		})
	}

}
