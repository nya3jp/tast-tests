// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SetBounds,
		Desc:         "Test to handle SetTaskWindowBounds in ARC++ companion library",
		Contacts:     []string{"hirokisato@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcSetBoundsTest.apk"},
		Timeout:      4 * time.Minute,
	})
}

func SetBounds(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcSetBoundsTest.apk"
		pkg = "org.chromium.arc.testapp.setbounds"

		resizableActivity   = ".ResizableActivity"
		unresizableActivity = ".UnresizableActivity"

		regularButtonID       = pkg + ":id/regular_button"
		smallerButtonID       = pkg + ":id/smaller_button"
		appControlledButtonID = pkg + ":id/controlled_toggle_button"
		unresizableButtonID   = pkg + ":id/go_unresizable_button"
		resizableButtonID     = pkg + ":id/go_resizable_button"

		// TODO(hirokisato) find a reliable way to share constants
		initialHeight = 600
		initialWidth  = 700
	)

	// The bounds below are specified in
	// pi-arc/vendor/google_arc/packages/development/ArcSetBoundsTest/src/org/chromium/arc/testapp/setbounds/BaseActivity.java
	regularBounds := arc.Rect{
		Left: 100, Top: 100, Width: 800, Height: 800,
	}

	// When the activity requests smaller bounds than its min-size, ARC framework expands the bounds to the its min-size.
	// The min-size is specified in AndrodiManifest.xml.
	smallerBounds := arc.Rect{
		Left: 200, Top: 200, Width: 600, Height: 500,
	}

	// TODO(crbug.com/1002958) Replace with Ash API to enable clamshell mode once it gets fixed.
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--force-tablet-mode=clamshell"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, resizableActivity)
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
	defer d.Close()

	if err := act.WaitForResumed(ctx, time.Second); err != nil {
		s.Fatal("Failed to wait for activity to resume: ", err)
	}

	// Validate initial window size.
	activityBounds, err := act.SurfaceBounds(ctx)
	if err != nil {
		s.Fatal("Failed to get window bounds: ", err)
	}

	if activityBounds.Height != initialHeight || activityBounds.Width != initialWidth {
		s.Fatalf("Unexpected window size: got (%d, %d); want (%d, %d)", activityBounds.Width, activityBounds.Height, initialWidth, initialHeight)
	}

	clickButtonAndValidateBounds := func(buttonId string, expected arc.Rect) {
		// Touch button.
		if err := d.Object(ui.ID(buttonId)).Click(ctx); err != nil {
			s.Fatalf("Could not click the button with id %q", buttonId)
		}

		// Wait until the bounds to be the expected one.
		err := testing.Poll(ctx, func(ctx context.Context) error {
			bounds, err := act.SurfaceBounds(ctx)
			if err != nil {
				s.Fatal("Failed to get window bounds: ", err)
			}
			if bounds != expected {
				return errors.Errorf("window bounds has not changed yet: got %s; want %s", bounds.String(), expected.String())
			}
			return nil
		}, &testing.PollOptions{Timeout: 4 * time.Second})

		if err != nil {
			s.Fatal("Error while waiting for bounds update: ", err)
		}
	}

	for _, test := range []struct {
		// resizable represents current window resizability.
		resizable bool
		// appControlled represents current appControlled flag of the task.
		appControlled bool
		// buttonIDs represents the id of buttons to be clicked in order to go to the next test state.
		buttonIDs []string
	}{
		{resizable: true, appControlled: false, buttonIDs: []string{appControlledButtonID}},
		{resizable: true, appControlled: true, buttonIDs: []string{appControlledButtonID, unresizableButtonID}},
		{resizable: false, appControlled: false, buttonIDs: []string{appControlledButtonID}},
		{resizable: false, appControlled: true, buttonIDs: nil},
	} {
		s.Logf("Testing resizable=%t, appControlled=%t", test.resizable, test.appControlled)

		clickButtonAndValidateBounds(regularButtonID, regularBounds)
		clickButtonAndValidateBounds(smallerButtonID, smallerBounds)

		// Even if app specified its bounds, the resizablity depends on its configuration.
		actual, err := act.Resizable(ctx)
		if err != nil {
			s.Fatal("Failed to get isResizable state: ", err)
		}
		if actual != test.resizable {
			s.Fatalf("window resizability is not expected: got %t; want %t", actual, test.resizable)
		}

		for _, id := range test.buttonIDs {
			if err := d.Object(ui.ID(id)).Click(ctx); err != nil {
				s.Fatalf("Could not click the button %q: %v", id, err)
			}
		}
	}
}
