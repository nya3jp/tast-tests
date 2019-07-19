// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SetBounds,
		Desc:         "Test to handle SetTaskWindowBounds in ARC++ companion library",
		Contacts:     []string{"hirokisato@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcSetBoundsTest.apk"},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

func SetBounds(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcSetBoundsTest.apk"
		pkg = "org.chromium.arc.testapp.setbounds"
		cls = ".MainActivity"

		regularButtonID = pkg + ":id/regular_button"
		smallerButtonID = pkg + ":id/smaller_button"

		initialHeight = 500
		initialWidth  = 600
	)

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

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed to get device: ", err)
	}

	disp, err := arc.NewDisplay(a, arc.DefaultDisplayID)
	if err != nil {
		s.Fatal("Failed to obtain a default display: ", err)
	}

	captionHeight, err := disp.CaptionHeight(ctx)
	if err != nil {
		s.Fatal("Failed to get arc size: ", err)
	}

	// Validate initial window size.
	activityBounds, err := act.WindowBounds(ctx)
	if err != nil {
		s.Fatal("Failed to get window bounds: ", err)
	}

	if activityBounds.Height != initialHeight+captionHeight || activityBounds.Width != initialWidth {
		s.Fatalf("Unexpected window size: got (%d, %d); want (%d, %d)", activityBounds.Width, activityBounds.Height, initialWidth, initialHeight+captionHeight)
	}

	clickButtonAndValidateBounds := func(buttonId string, expected arc.Rect) {
		// Touch button.
		if err := d.Object(ui.ID(buttonId)).Click(ctx); err != nil {
			s.Fatalf("Could not click the button with id %q", buttonId)
		}

		if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
			s.Fatal("Error while waiting for idle: ", err)
		}

		// Validate the window size.
		bounds, err := act.WindowBounds(ctx)
		if err != nil {
			s.Fatal("Failed to get window bounds: ", err)
		}
		if bounds != expected {
			s.Fatalf("Unexpected bounds: got %s; want %s", bounds.String(), expected.String())
		}
	}

	// The bounds below bounds are specified in
	// pi-arc/vendor/google_arc/packages/development/ArcSetBoundsTest/src/org/chromium/arc/testapp/setbounds/MainActivity.java
	clickButtonAndValidateBounds(regularButtonID, arc.Rect{
		Left: 100, Top: 100, Width: 800, Height: 800,
	})

	// In the second action, the activity requests smaller bounds than its min-size.
	// The framework expands the bounds to the its min-size (which is also specified in AndrodiManifest.xml).
	clickButtonAndValidateBounds(smallerButtonID, arc.Rect{
		Left: 200, Top: 200, Width: 300, Height: 400,
	})
}
