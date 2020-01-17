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

const (
	setBoundsApk = "ArcSetBoundsTest.apk"
	setBoundsPkg = "org.chromium.arc.testapp.setbounds"

	regularButtonID       = setBoundsPkg + ":id/regular_button"
	smallerButtonID       = setBoundsPkg + ":id/smaller_button"
	appControlledButtonID = setBoundsPkg + ":id/controlled_toggle_button"

	// TODO(hirokisato) find a reliable way to share constants
	initialHeight = 600
	initialWidth  = 700
)

// The bounds below are specified in
// pi-arc/vendor/google_arc/packages/development/ArcSetBoundsTest/src/org/chromium/arc/testapp/setbounds/BaseActivity.java
var regularBounds = arc.Rect{Left: 100, Top: 100, Width: 800, Height: 800}

// When the activity requests smaller bounds than its min-size, ARC framework expands the bounds to the its min-size.
// The min-size is specified in AndroidManifest.xml.
var smallerBounds = arc.Rect{Left: 200, Top: 200, Width: 600, Height: 500}

func SetBounds(ctx context.Context, s *testing.State) {
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

	if err := a.Install(ctx, s.DataPath(setBoundsApk)); err != nil {
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
		{".ResizableActivity", true},
		{".UnresizableActivity", false},
	} {
		if err := runSubTest(ctx, a, d, test.act, test.resizable); err != nil {
			s.Errorf("Subtest(%s) failed: %v", test.act, err)
		}
	}
}

func runSubTest(ctx context.Context, a *arc.ARC, d *ui.Device, actName string, resizable bool) error {
	testing.ContextLogf(ctx, "Starting %s", actName)

	act, err := arc.NewActivity(a, setBoundsPkg, actName)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", actName)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		return err
	}
	// Stop activity at exit time so that the next WM test can launch a different activity from the same package.
	defer act.Stop(ctx)

	if err := act.WaitForResumed(ctx, time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for activity to resume")
	}

	// Validate initial window size.
	activityBounds, err := act.SurfaceBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get window bounds")
	}

	if activityBounds.Height != initialHeight || activityBounds.Width != initialWidth {
		return errors.Errorf("unexpected initial window size: got (%d, %d); want (%d, %d)", activityBounds.Width, activityBounds.Height, initialWidth, initialHeight)
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
				return err
			}
			if bounds != expected {
				return errors.Errorf("window bounds has not changed yet: got %v; want %v", &bounds, &expected)
			}
			return nil
		}, &testing.PollOptions{Timeout: 4 * time.Second})
	}

	for _, appControlled := range []bool{false, true} {
		testing.ContextLogf(ctx, "Testing resizable=%t, appControlled=%t", resizable, appControlled)

		clickButtonAndValidateBounds(regularButtonID, regularBounds)
		clickButtonAndValidateBounds(smallerButtonID, smallerBounds)

		// The resizablity depends on its configuration.
		// TODO(hirokisato): take Chrome-side value, instead of Android-side value.
		actual, err := act.Resizable(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get isResizable state")
		}
		if actual != resizable {
			return errors.Errorf("window resizability is not expected: got %t; want %t", actual, resizable)
		}

		// Toggle App-Controlled state.
		if err := d.Object(ui.ID(appControlledButtonID)).Click(ctx); err != nil {
			return errors.Wrap(err, "could not click the button")
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if controlled, err := act.AppControlled(ctx); err != nil {
				return err
			} else if controlled == appControlled {
				return errors.New("app controlled state hasn't been changed yet")
			}
			return nil
		}, &testing.PollOptions{Timeout: 4 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to toggle app controlled state")
		}
	}
	return nil
}
