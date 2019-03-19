// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PIP,
		Desc:         "Checks that ARC++ Picture-in-Picture works as expected",
		Contacts:     []string{"ricardoq@chromium.org", "edcourtney@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"touch_view", "android_p", "chrome_login"},
		Timeout:      5 * time.Minute,
		Data:         []string{"ArcPipTastTest.apk"},
	})
}

func PIP(ctx context.Context, s *testing.State) {
	must := func(err error) {
		if err != nil {
			s.Fatal("Failed: ", err)
		}
	}

	// For debugging, add chrome.ExtraArgs("--show-taps")
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	const apkName = "ArcPipTastTest.apk"
	s.Log("Installing ", apkName)
	if err := a.Install(ctx, s.DataPath(apkName)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	const pkgName = "org.chromium.arc.testapp.pictureinpicture"
	act, err := arc.NewActivity(a, pkgName, ".MainActivity")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	dev, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer dev.Close()

	tabletModeEnabled, err := isTabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	// Be nice and restore tablet mode to its original state on exit.
	defer setTabletModeEnabled(ctx, tconn, tabletModeEnabled)

	// Run all subtests twice. First, with tablet mode disabled. And then, with it enabled.
	for _, tabletMode := range []bool{false, true} {
		if err := setTabletModeEnabled(ctx, tconn, tabletMode); err != nil {
			s.Fatalf("Failed to set tablet mode enabled to %t: %v", tabletMode, err)
		}

		type testFunc func(context.Context, *chrome.Chrome, *arc.Activity, *ui.Device) error
		for idx, test := range []struct {
			name string
			fn   testFunc
		}{
			{"PIP Move", testPIPMove},
		} {
			s.Logf("Running %q", test.name)
			// Clear task WM state. Windows should be positioned at their default location.
			a.Command(ctx, "am", "broadcast", "-a", "android.intent.action.arc.cleartaskstate")
			must(act.Start(ctx))
			must(act.WaitForIdle(ctx, time.Second))
			// Press button that triggers PIP mode in activity.
			const pipButtonID = pkgName + ":id/enter_pip"
			must(dev.Object(ui.ID(pipButtonID)).Click(ctx))
			must(act.WaitForIdle(ctx, time.Second))

			if err := test.fn(ctx, cr, act, dev); err != nil {
				path := fmt.Sprintf("%s/screenshot-pip-failed-test-%d.png", s.OutDir(), idx)
				if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
					s.Log("Failed to capture screenshot: ", err)
				}
				s.Errorf("%q + tablet mode(%t) failed: %v", test.name, tabletMode, err)
			}
			must(act.Stop(ctx))
		}
	}
}

// testPIPMove verifies that drag-moving the PIP window works as expected.
// It does that by drag-moving that PIP window horizontally 3 times and verifying that the position is correct.
func testPIPMove(ctx context.Context, cr *chrome.Chrome, act *arc.Activity, d *ui.Device) error {
	const (
		// When the drag-move sequence is started, the gesture controller might miss a few pixels before it finally
		// recognizes it as a drag-move gesture. This is specially true for PIP windows.
		// The value varies depending on acceleration/speed of the gesture. 35 works for our purpose.
		missedByGestureControllerDP = 35
		movementDuration            = time.Second
		totalMovements              = 3
	)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get test API connection")
	}

	dispMode, err := getInternalDisplayMode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}

	missedByGestureControllerPX := dispMode.DeviceScaleFactor * missedByGestureControllerDP
	testing.ContextLog(ctx, "Using: missedByGestureControllerPX = ", missedByGestureControllerPX)

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}
	testing.ContextLogf(ctx, "Initial PIP bounds: %+v", bounds)

	deltaX := dispMode.WidthInNativePixels / (totalMovements + 1)
	for i := 0; i < totalMovements; i++ {
		testing.ContextLogf(ctx, "Moving PIP window to %d,%d", bounds.Left-deltaX, bounds.Top)
		if err := act.MoveWindow(ctx, arc.NewPoint(bounds.Left-deltaX, bounds.Top), movementDuration); err != nil {
			return errors.Wrap(err, "could not move PIP window")
		}
		if err := act.WaitForIdle(ctx, time.Second); err != nil {
			return err
		}

		newBounds, err := act.WindowBounds(ctx)
		if err != nil {
			return errors.Wrap(err, "could not get PIP window bounds")
		}
		testing.ContextLogf(ctx, "PIP bounds after move: %+v", newBounds)

		diff := float64(bounds.Left - deltaX - newBounds.Left)
		if diff > missedByGestureControllerPX {
			return errors.Wrapf(err, "invalid PIP bounds: %+v; expected %g < %g", bounds, diff, missedByGestureControllerPX)
		}
		bounds = newBounds
	}
	return nil
}

// helper functions

// getInternalDisplayMode returns the display mode that is currently selected in the internal display.
func getInternalDisplayMode(ctx context.Context, tconn *chrome.Conn) (*display.DisplayMode, error) {
	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get internal display info")
	}
	for _, mode := range dispInfo.Modes {
		if mode.IsSelected {
			return mode, nil
		}
	}
	return nil, errors.New("failed to get selected mode")
}

// setTabletModeEnabled enables / disables tablet mode.
// After calling this function, it won't be possible to physically switch to/from tablet mode since that functionality will be disabled.
func setTabletModeEnabled(ctx context.Context, c *chrome.Conn, enabled bool) error {
	e := strconv.FormatBool(enabled)
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.setTabletModeEnabled(%s, function(enabled) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		      return;
		    }
		    if (enabled == %s) {
		      resolve();
		    } else {
		      reject(new Error("unexpected tablet mode: " + enabled));
		    }
		  })
		})`, e, e)
	return c.EvalPromise(ctx, expr, nil)
}

// isTabletModeEnabled gets the tablet mode enabled status.
func isTabletModeEnabled(ctx context.Context, tconn *chrome.Conn) (bool, error) {
	var enabled bool
	err := tconn.EvalPromise(ctx,
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.isTabletModeEnabled(function(enabled) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve(enabled);
		    }
		  })
		})`, &enabled)
	return enabled, err
}
