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
	"chromiumos/tast/local/chrome/ash"
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
		pkg = "org.chromium.arc.testapp.setbounds"

		regularButtonID       = pkg + ":id/regular_button"
		smallerButtonID       = pkg + ":id/smaller_button"
		appControlledButtonID = pkg + ":id/controlled_toggle_button"

		// TODO(hirokisato) find a reliable way to share constants
		initH = 600
		initW = 700
	)

	// The bounds below are specified in
	// pi-arc/vendor/google_arc/packages/development/ArcSetBoundsTest/src/org/chromium/arc/testapp/setbounds/BaseActivity.java
	regularBounds := arc.Rect{Left: 100, Top: 100, Width: 800, Height: 800}

	// When the activity requests smaller bounds than its min-size, ARC framework expands the bounds to the its min-size.
	// The min-size is specified in AndroidManifest.xml.
	smallerBounds := arc.Rect{Left: 200, Top: 200, Width: 600, Height: 500}

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

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	if err := a.Install(ctx, s.DataPath("ArcSetBoundsTest.apk")); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed to get device: ", err)
	}
	defer d.Close()

	for _, test := range []struct {
		act       string
		resizable bool
	}{
		{"ResizableActivity", true},
		{"UnresizableActivity", false},
	} {
		s.Run(ctx, test.act, func(ctx context.Context, s *testing.State) {
			act, err := arc.NewActivity(a, pkg, "."+test.act)
			if err != nil {
				s.Fatal("Failed to create new activity: ", err)
			}
			defer act.Close()

			if err := act.Start(ctx); err != nil {
				s.Fatal("Failed start the activity: ", err)
			}
			// Stop activity at exit time so that the next WM test can launch a different activity from the same package.
			defer act.Stop(ctx)

			if err := act.WaitForResumed(ctx, time.Second); err != nil {
				s.Fatal("Failed to wait for activity to resume: ", err)
			}

			// Validate initial window size.
			actBounds, err := act.SurfaceBounds(ctx)
			if err != nil {
				s.Fatal("Failed to get window bounds: ", err)
			}

			if actBounds.Height != initH || actBounds.Width != initW {
				s.Fatalf("Unexpected initial bounds: got (%d, %d); want (%d, %d)", actBounds.Width, actBounds.Height, initH, initW)
			}

			clickButtonAndValidateBounds := func(buttonId string, expected arc.Rect) error {
				// Touch the button.
				if err := d.Object(ui.ID(buttonId)).Click(ctx); err != nil {
					return errors.Wrapf(err, "could not click the button with id %q", buttonId)
				}

				// Wait until the bounds to be the expected one.
				return testing.Poll(ctx, func(ctx context.Context) error {
					bounds, err := act.SurfaceBounds(ctx)
					if err != nil {
						return testing.PollBreak(err)
					}
					if bounds != expected {
						return errors.Errorf("window bounds has not changed yet: got %v; want %v", &bounds, &expected)
					}
					return nil
				}, &testing.PollOptions{Timeout: 4 * time.Second})
			}

			for _, appControlled := range []bool{false, true} {
				testing.ContextLogf(ctx, "Testing resizable=%t, appControlled=%t", test.resizable, appControlled)

				// Validate that behaviour of chaning bounds doesn't depends on window resizability or app-controlled state.
				if err := clickButtonAndValidateBounds(regularButtonID, regularBounds); err != nil {
					s.Fatal("Failed to run the step to resize normally: ", err)
				}
				if err := clickButtonAndValidateBounds(smallerButtonID, smallerBounds); err != nil {
					s.Fatal("Failed to run the step to resize smaller than the min size: ", err)
				}

				// Validate that changing bounds from Companion lib doesn't change window resizability.
				info, err := ash.GetARCAppWindowInfo(ctx, tconn, pkg)
				if err != nil {
					s.Fatal("Failed to get isResizable state: ", err)
				}
				if info.CanResize != test.resizable {
					s.Fatalf("Window resizability is not expected: got %t; want %t", info.CanResize, test.resizable)
				}

				// Toggle App-Controlled state.
				// TODO(hirokisato): Wait for app controlled state to be updated. This should be done together with b/148116159.
				if err := d.Object(ui.ID(appControlledButtonID)).Click(ctx); err != nil {
					s.Fatal("Could not click the appControlled toggle button: ", err)
				}
			}
		})
	}
}
