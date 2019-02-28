// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Pip,
		Desc:         "Checks that ARC++ Picture-in-Pictures works as expected.",
		Contacts:     []string{"ricardoq@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"disabled", "informational"},
		SoftwareDeps: []string{"touch_view", "android", "android_p", "chrome_login"},
		Timeout:      4 * time.Minute,
		Data:         []string{"piptest.apk"},
	})
}

func Pip(ctx context.Context, s *testing.State) {
	const (
		apk = "piptest.apk"
		pkg = "com.example.edcourtney.pictureinpicturetest"

		idPrefix = pkg + ":id/"
		fabID    = idPrefix + "fab"
	)

	must := func(err error) {
		if err != nil {
			s.Fatal("Failed: ", err)
		}
	}

	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	s.Log("Installing APK")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, ".MainActivity")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()
	d.EnableDebug()

	firstTime := true
	type testFunc func(ctx context.Context, cr *chrome.Chrome, act *arc.Activity, d *ui.Device) error
	for idx, entry := range []struct {
		name string
		fn   testFunc
	}{
		{"PiP Move", testPIPMove},
		{"PiP Resize", testPIPResize},
		{"PIP Avoid Obstable", testPIPAvoidObstable},
		// {"PiP Fling", testPIPFling},
		// {"PiP ShelfAutoHide", testPIPShelfAutoHide},
	} {
		s.Log("Testing: " + entry.name)

		if firstTime {
			must(act.Start(ctx))
			firstTime = false
		} else {
			must(act.Restart(ctx))
		}
		must(d.Object(ui.ID(fabID)).Click(ctx))
		d.WaitForIdle(ctx, 3*time.Second)

		if err := entry.fn(ctx, cr, act, d); err != nil {
			path := fmt.Sprintf("%s/screenshot-pip-failed-test-%d.png", s.OutDir(), idx)
			if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
				s.Log("Failed to capture screenshot")
			}
			s.Error("Test failed: ", err)
		}
	}
}

func setTabletModeEnabled(ctx context.Context, c *chrome.Conn, enabled bool) error {
	e := strconv.FormatBool(enabled)
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
			chrome.autotestPrivate.setTabletModeEnabled(%s, 
				function(enabled) {
					if (enabled == %s) {
						resolve(chrome.runtime.lastError ? chrome.runtime.lastError.message : "");
					} else {
						reject();
					}
				});
		})`, e, e)

	if err := c.EvalPromise(ctx, expr, nil); err != nil {
		return err
	}
	return nil
}

func isTabletModeEnabled(ctx context.Context, c *chrome.Conn) (enabled bool, err error) {
	const expr = `new Promise(function(resolve, reject) {
			chrome.autotestPrivate.isTabletModeEnabled(function(enabled) {
					resolve(chrome.runtime.lastError ? chrome.runtime.lastError.message : enabled);
				});
		})`

	if err := c.EvalPromise(ctx, expr, &enabled); err != nil {
		return false, err
	}
	return enabled, nil
}

func setShelfAutoHideBehavior(ctx context.Context, c *chrome.Conn, displayID string, behavior string) error {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
			chrome.autotestPrivate.setShelfAutoHideBehavior(%q, %q, () => {
				if (chrome.runtime.lastError === undefined) {
					resolve();
				} else {
					reject(chrome.runtime.lastError.message);
				}
			});
		})`, displayID, behavior)

	if err := c.EvalPromise(ctx, expr, nil); err != nil {
		return err
	}
	return nil
}

func getShelfAutoHideBehavior(ctx context.Context, c *chrome.Conn, displayID string) (behavior string, err error) {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
			chrome.autotestPrivate.getShelfAutoHideBehavior(%q, (behavior) => {
				if (chrome.runtime.lastError === undefined) {
					resolve(behavior);
				} else {
					reject(chrome.runtime.lastError.message);
				}
			});
		})`, displayID)

	if err := c.EvalPromise(ctx, expr, &behavior); err != nil {
		return "", err
	}
	return behavior, nil
}

func testPIPShelfAutoHide(ctx context.Context, cr *chrome.Chrome, act *arc.Activity) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not internal info")
	}

	for _, b := range []string{"always", "never", "hidden"} {
		// enabled, err := isTabletModeEnabled(ctx, tconn)
		// if err != nil {
		// 	s.Fatal("Failed to get tablet mode status: ", err)
		// }
		// s.Logf("Tablet mode enabled = %v", enabled)
		// if err := setTabletModeEnabled(ctx, tconn, !enabled); err != nil {
		// 	s.Fatal("Failed to set tablet mode: ", err)
		// }

		testing.ContextLogf(ctx, "Setting behavior: %s", b)
		if err := setShelfAutoHideBehavior(ctx, tconn, info.ID, b); err != nil {
			return errors.Wrap(err, "failed to set shelf autohide behavior")
		}

		behavior, err := getShelfAutoHideBehavior(ctx, tconn, info.ID)
		if err != nil {
			return errors.Wrap(err, "failed to get shelf auto hide behavior")
		}
		testing.ContextLogf(ctx, "Behavior is: %q", behavior)

		// Small delay.
		select {
		case <-time.After(3 * time.Second):
		case <-ctx.Done():
			return errors.Wrap(err, "timeout")
		}
	}
	return nil
}

// testPIPMove tests that dragging the PiP window works as expected.
func testPIPMove(ctx context.Context, cr *chrome.Chrome, act *arc.Activity, d *ui.Device) error {
	// Moving window very slowly to prevent triggering any possible gesture.
	if err := act.MoveWindow(ctx, arc.Point{X: 0, Y: 0}, 5*time.Second); err != nil {
		return errors.Wrap(err, "could not move Pip window")
	}

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}

	// A PiP window doesn't have caption. According to Android the Y position will be negative.
	// PiP windows don't snap to borders. A margin is used.
	if bounds.Top > 0 || bounds.Left > 50 {
		return errors.Wrapf(err, "invalid PiP bounds: %+v", bounds)
	}
	return nil
}

func testPIPResize(ctx context.Context, cr *chrome.Chrome, act *arc.Activity, d *ui.Device) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get a Test API connection")
	}

	// Activate "resize handler", otherwise resize will fail.
	if err := activatePIPMenuActivity(ctx, d); err != nil {
		return errors.Wrap(err, "could not activate PiP menu")
	}

	// The PiP window should be on the bottom-right corner. Resizing it from the top-left border to its max size.
	if err := act.ResizeWindow(ctx, arc.BorderTopLeft, arc.Point{X: 0, Y: 0}, 300*time.Millisecond); err != nil {
		return errors.Wrap(err, "could not move Pip window")
	}
	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display info")
	}

	if len(dispInfo.Modes) == 0 {
		return errors.New("invalid size for dispInfo.Modes")
	}

	// Ignore the selected mode, just use mode[0]. We only care about the native pixels which must be the same for all modes.
	dispW := float64(dispInfo.Modes[0].WidthInNativePixels)
	dispH := float64(dispInfo.Modes[0].HeightInNativePixels)

	// Give a 2% error margin. Resizing by injecting touches might not be %100 precise.
	marginW := dispW * 0.02
	marginH := dispH * 0.02

	w := float64(bounds.Right - bounds.Left)
	h := float64(bounds.Bottom - bounds.Top)

	// PiP will honor its aspect ratio after resize. It is enough to check only one of the dimensions.
	if dispH < dispW {
		if math.Abs(dispH/2-h) >= marginH {
			testing.ContextLogf(ctx, "%q - %q >= %q (%v)", dispH/2, h, marginH, bounds)
			return errors.Wrapf(err, "unexpected bounds size (%+v)", bounds)
		}
	} else {
		if math.Abs(dispW/2-w) >= marginW {
			testing.ContextLogf(ctx, "%q - %q >= %q", dispW/2, h, marginW)
			return errors.Wrapf(err, "unexpected bounds size (%+v)", bounds)
		}
	}
	return nil
}

func testPIPAvoidObstable(ctx context.Context, cr *chrome.Chrome, act *arc.Activity, d *ui.Device) error {
	// PiP window should automatically move away from system windows, like the the system status area.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open keyboard device")
	}
	defer ew.Close()

	// FIXME(ricardoq): verify status
	// Small delay.
	select {
	case <-time.After(1 * time.Second):
	case <-ctx.Done():
		return errors.Wrap(err, "timeout")
	}

	// Display system status area
	if err = ew.Accel(ctx, "Alt+Shift+s"); err != nil {
		return errors.Wrap(err, "failed")
	}

	// FIXME(ricardoq): verify status
	// Small delay.
	select {
	case <-time.After(1 * time.Second):
	case <-ctx.Done():
		return errors.Wrap(err, "timeout")
	}

	// Hide system status area
	if err = ew.Accel(ctx, "Alt+Shift+s"); err != nil {
		return errors.Wrap(err, "failed")
	}
	return nil
}

func testPIPFling(ctx context.Context, cr *chrome.Chrome, act *arc.Activity) error {
	return nil
}

// helper functions

// activatePIPMenuActivity will activate the PiP Menu overlay by injecting KEYCODE_WINDOW event in Android.
// Undefined behavior if there are no active PiP windows.
// TODO(ricardoq): should be inside arc.Activity, but that will create a circular dependency between arc and arc.ui.
func activatePIPMenuActivity(ctx context.Context, d *ui.Device) error {
	// Corresponds to Android's KEYCODE_WINDOW. See:
	// https://cs.corp.google.com/android/frameworks/base/core/java/android/view/KeyEvent.java
	const keyCodeWindow = 171

	// Activate PiP menu. See:
	// https://cs.corp.google.com/android/frameworks/base/services/core/java/com/android/server/policy/PhoneWindowManager.java?type=cs&g=0&l=4007
	d.PressKeyCode(ctx, keyCodeWindow, 0)

	// Wait a few milliseconds until the PiP Menu animation is complete.
	select {
	case <-time.After(300 * time.Millisecond):
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "timeout while doing sleep")
	}
	return nil
}
