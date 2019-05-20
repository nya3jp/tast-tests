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

const (
	// kCollisionWindowWorkAreaInsetsDp is hardcoded to 8dp. Using a bigger value to safe.
	// See: https://cs.chromium.org/chromium/src/ash/wm/collision_detection/collision_detection_utils.h
	// TODO(crbug.com/949754): Get this value in runtime.
	collisionWindowWorkAreaInsetsDP = 8 + 5

	// pipPositionErrorMarginPX represents the error margin in pixels when comparing positions. See b/129976114 for more info.
	// TODO(ricardoq): Remove this constant once the bug gets fixed.
	pipPositionErrorMarginPX = 1
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PIP,
		Desc:         "Checks that ARC++ Picture-in-Picture works as expected",
		Contacts:     []string{"ricardoq@chromium.org", "edcourtney@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"tablet_mode", "android_p", "chrome"},
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
	// --use-test-config is needed to enable Shelf's Mojo testing interface.
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--use-test-config"))
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

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}

	origShelfAlignment, err := getShelfAlignment(ctx, tconn, dispInfo.ID)
	if err != nil {
		s.Fatal("Failed to get shelf alignment: ", err)
	}
	if err := setShelfAlignment(ctx, tconn, dispInfo.ID, shelfAlignmentBottom); err != nil {
		s.Fatal("Failed to set shelf alignment to Bottom: ", err)
	}
	// Be nice and restore shelf alignment to its original state on exit.
	defer setShelfAlignment(ctx, tconn, dispInfo.ID, origShelfAlignment)

	origShelfBehavior, err := getShelfBehavior(ctx, tconn, dispInfo.ID)
	if err != nil {
		s.Fatal("Failed to get shelf behavior: ", err)
	}
	if err := setShelfBehavior(ctx, tconn, dispInfo.ID, shelfBehaviorNeverAutoHide); err != nil {
		s.Fatal("Failed to set shelf behavior to Never Auto Hide: ", err)
	}
	// Be nice and restore shelf behavior to its original state on exit.
	defer setShelfBehavior(ctx, tconn, dispInfo.ID, origShelfBehavior)

	tabletModeEnabled, err := isTabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	// Be nice and restore tablet mode to its original state on exit.
	defer setTabletModeEnabled(ctx, tconn, tabletModeEnabled)

	dispMode, err := getInternalDisplayMode(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display mode: ", err)
	}

	// Run all subtests twice. First, with tablet mode disabled. And then, with it enabled.
	for _, tabletMode := range []bool{false, true} {
		s.Logf("Running tests with tablet mode enabled=%t", tabletMode)
		if err := setTabletModeEnabled(ctx, tconn, tabletMode); err != nil {
			s.Fatalf("Failed to set tablet mode enabled to %t: %v", tabletMode, err)
		}

		type testFunc func(context.Context, *chrome.Conn, *arc.Activity, *ui.Device, *display.DisplayMode) error
		for idx, test := range []struct {
			name string
			fn   testFunc
		}{
			{"PIP Move", testPIPMove},
			{"PIP Resize", testPIPResize},
			{"PIP Fling", testPIPFling},
			{"PIP Toggle Tablet mode", testPIPToggleTabletMode},
		} {
			s.Logf("Running %q", test.name)
			// Clear task WM state. Windows should be positioned at their default location.
			if err := a.Command(ctx, "am", "broadcast", "-a", "android.intent.action.arc.cleartaskstate").Run(); err != nil {
				s.Error("Failed to clear WM state: ", err)
			}
			must(act.Start(ctx))
			must(act.WaitForIdle(ctx, time.Second))
			// Press button that triggers PIP mode in activity.
			const pipButtonID = pkgName + ":id/enter_pip"
			must(dev.Object(ui.ID(pipButtonID)).Click(ctx))
			// TODO(b/131248000) WaitForIdle doesn't catch all PIP possible animations.
			// Add temporary delay until it gets fixed.
			must(testing.Sleep(ctx, 200*time.Millisecond))
			must(act.WaitForIdle(ctx, time.Second))

			if err := test.fn(ctx, tconn, act, dev, dispMode); err != nil {
				path := fmt.Sprintf("%s/screenshot-pip-failed-test-%d.png", s.OutDir(), idx)
				if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
					s.Log("Failed to capture screenshot: ", err)
				}
				s.Errorf("%s test with tablet mode(%t) failed: %v", test.name, tabletMode, err)
			}
			must(act.Stop(ctx))
		}
	}
}

// testPIPMove verifies that drag-moving the PIP window works as expected.
// It does that by drag-moving that PIP window horizontally 3 times and verifying that the position is correct.
func testPIPMove(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	const (
		// When the drag-move sequence is started, the gesture controller might miss a few pixels before it finally
		// recognizes it as a drag-move gesture. This is specially true for PIP windows.
		// The value varies depending on acceleration/speed of the gesture. 35 works for our purpose.
		missedByGestureControllerDP = 35
		movementDuration            = time.Second
		totalMovements              = 3
	)

	missedByGestureControllerPX := int(math.Round(missedByGestureControllerDP * dispMode.DeviceScaleFactor))
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

		diff := bounds.Left - deltaX - newBounds.Left
		if diff > missedByGestureControllerPX {
			return errors.Wrapf(err, "invalid PIP bounds: %+v; expected %d < %d", bounds, diff, missedByGestureControllerPX)
		}
		bounds = newBounds
	}
	return nil
}

// testPIPResize verifies that resizing the PIP window works as expected.
// It performs a drag-resize from PIP's left-top corner and compares the resized-PIP size with the expected one.
func testPIPResize(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	// Activate PIP "resize handler", otherwise resize will fail. See:
	// https://android.googlesource.com/platform/frameworks/base/+/refs/heads/pie-release/services/core/java/com/android/server/policy/PhoneWindowManager.java#6387
	if err := dev.PressKeyCode(ctx, ui.KEYCODE_WINDOW, 0); err != nil {
		return errors.Wrap(err, "could not activate PIP menu")
	}
	if err := act.WaitForIdle(ctx, time.Second); err != nil {
		return err
	}

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}
	testing.ContextLogf(ctx, "Bounds before resize: %+v", bounds)

	testing.ContextLog(ctx, "Resizing window to x=0, y=0")
	// Resizing PIP to x=0, y=0, but it should stop once it reaches its max size.
	if err := act.ResizeWindow(ctx, arc.BorderTopLeft, arc.NewPoint(0, 0), time.Second); err != nil {
		return errors.Wrap(err, "could not resize PIP window")
	}

	bounds, err = act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}
	testing.ContextLogf(ctx, "Bounds after resize: %+v", bounds)

	rectDP, err := getShelfRect(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get shelf rect")
	}
	shelfHeightPX := int(math.Round(float64(rectDP.Height) * dispMode.DeviceScaleFactor))

	// Max PIP window size relative to the display size, as defined in WindowPosition.getMaximumSizeForPip().
	// See: https://cs.corp.google.com/pi-arc-dev/frameworks/base/services/core/arc/java/com/android/server/am/WindowPositioner.java
	// Dividing by integer 2 could loose the fraction, but so does the Java implementation.
	// TODO(crbug.com/949754): Get this value in runtime.
	const pipMaxSizeFactor = 2
	pipMaxSizeW := dispMode.WidthInNativePixels / pipMaxSizeFactor
	pipMaxSizeH := (dispMode.HeightInNativePixels - shelfHeightPX) / pipMaxSizeFactor

	w := bounds.Width
	h := bounds.Height

	// Aspect ratio gets honored after resize. Only test one dimension.
	if pipMaxSizeH < pipMaxSizeW {
		min := pipMaxSizeH - pipPositionErrorMarginPX
		max := pipMaxSizeH + pipPositionErrorMarginPX
		if h < min || h > max {
			return errors.Wrapf(err, "invalid height %d; want %d <= %d <= %d)", h, min, h, max)
		}
	} else {
		min := pipMaxSizeW - pipPositionErrorMarginPX
		max := pipMaxSizeW + pipPositionErrorMarginPX
		if w < min || w > max {
			return errors.Wrapf(err, "invalid width %d; want %d <= %d <= %d", w, min, w, max)
		}
	}
	return nil
}

// testPIPFling tests that fling works as expected. It tests the fling gesture in four directions: left, up, right and down.
func testPIPFling(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	type borderType int
	const (
		// Borders to check after swipe.
		left borderType = iota
		right
		top
		bottom
	)

	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open touchscreen device")
	}
	defer tsw.Close()

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "could not create TouchEventWriter")
	}
	defer stw.Close()

	collisionWindowWorkAreaInsetsPX := int(math.Round(collisionWindowWorkAreaInsetsDP * dispMode.DeviceScaleFactor))
	testing.ContextLog(ctx, "Using: collisionWindowWorkAreaInsetsPX = ", collisionWindowWorkAreaInsetsPX)

	// Calculate Pixel (screen display) / Tuxel (touch device) ratio.
	dispW := dispMode.WidthInNativePixels
	dispH := dispMode.HeightInNativePixels
	pixelToTuxelX := float64(tsw.Width()) / float64(dispW)
	pixelToTuxelY := float64(tsw.Height()) / float64(dispH)

	for _, dir := range []struct {
		x, y   int
		border borderType
	}{
		{-1, 0, left},  // swipe to left
		{0, -1, top},   // swipe to top
		{1, 0, right},  // swipe to right
		{0, 1, bottom}, // swipe to bottom
	} {
		if err := act.WaitForIdle(ctx, time.Second); err != nil {
			return err
		}

		bounds, err := act.WindowBounds(ctx)
		if err != nil {
			return errors.Wrap(err, "could not get PIP window bounds")
		}

		pipCenterX := float64(bounds.Left + bounds.Width/2)
		pipCenterY := float64(bounds.Top + bounds.Height/2)

		x0 := input.TouchCoord(pipCenterX * pixelToTuxelX)
		y0 := input.TouchCoord(pipCenterY * pixelToTuxelY)
		x1 := input.TouchCoord((pipCenterX + float64(dir.x*dispW/3)) * pixelToTuxelX)
		y1 := input.TouchCoord((pipCenterY + float64(dir.y*dispH/3)) * pixelToTuxelY)

		testing.ContextLogf(ctx, "Running swipe gesture from {%d,%d} to {%d,%d}", x0, y0, x1, y1)
		if err := stw.Swipe(ctx, x0, y0, x1, y1, 200*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to execute a swipe gesture")
		}
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the swipe gesture")
		}

		// TODO(b/131248000): WaitForIdle doesn't catch all PIP possible animations, like fling.
		// Add temporary delay until it gets fixed.
		if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
			return err
		}
		if err := act.WaitForIdle(ctx, time.Second); err != nil {
			return err
		}

		// After swipe, check that the PIP window arrived to destination.
		bounds, err = act.WindowBounds(ctx)
		if err != nil {
			return errors.Wrap(err, "could not get PIP window bounds after swipe")
		}
		switch dir.border {
		case left:
			if bounds.Left < 0 || bounds.Left > collisionWindowWorkAreaInsetsPX {
				return errors.Errorf("failed swipe to left; invalid left bounds %d; want 0 <= %d <= %d",
					bounds.Left, bounds.Left, collisionWindowWorkAreaInsetsPX)
			}
		case right:
			boundsRight := bounds.Left + bounds.Width
			if boundsRight > dispW || boundsRight < dispW-collisionWindowWorkAreaInsetsPX {
				return errors.Errorf("failed swipe to right; invalid right bounds %d; want %d <= %d <= %d",
					boundsRight, dispW-collisionWindowWorkAreaInsetsPX, boundsRight, dispW)
			}
		case top:
			if bounds.Top < 0 || bounds.Top > collisionWindowWorkAreaInsetsPX {
				return errors.Errorf("failed swipe to top; invalid top bounds %d; want 0 <= %d <= %d",
					bounds.Top, bounds.Top, collisionWindowWorkAreaInsetsPX)
			}
		case bottom:
			rectDP, err := getShelfRect(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to get shelf rect")
			}
			shelfTopPX := int(math.Round(float64(rectDP.Top) * dispMode.DeviceScaleFactor))
			boundsBottom := bounds.Top + bounds.Height
			if boundsBottom >= shelfTopPX || boundsBottom < shelfTopPX-collisionWindowWorkAreaInsetsPX {
				return errors.Errorf("failed swipe to bottom; invalid bottom bounds %d; want %d <= %d < %d",
					boundsBottom, boundsBottom-collisionWindowWorkAreaInsetsPX, boundsBottom, shelfTopPX)
			}
		}
	}
	return nil
}

// testPIPToggleTabletMode verifies that the window position is the same after toggling tablet mode.
func testPIPToggleTabletMode(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	origBounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get window bounds")
	}
	testing.ContextLogf(ctx, "Initial bounds: %+v", origBounds)

	tabletEnabled, err := isTabletModeEnabled(ctx, tconn)
	if err != nil {
		return errors.New("failed to get whether tablet mode is enabled")
	}
	defer setTabletModeEnabled(ctx, tconn, tabletEnabled)

	testing.ContextLogf(ctx, "Setting 'tablet mode enabled = %t'", !tabletEnabled)
	if err := setTabletModeEnabled(ctx, tconn, !tabletEnabled); err != nil {
		return errors.New("failed to set tablet mode")
	}

	if err := act.WaitForIdle(ctx, time.Second); err != nil {
		return err
	}

	bounds, err := act.WindowBounds(ctx)
	testing.ContextLogf(ctx, "Bounds after toggling tablet mode: %+v", origBounds)

	if origBounds != bounds {
		return errors.Errorf("invalid position %+v; want %+v", origBounds, bounds)
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
		    if (enabled != %s) {
		      reject(new Error("unexpected tablet mode: " + enabled));
		    } else {
		      resolve();
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

// shelfBehavior represents the different Chrome OS shelf behaviors.
type shelfBehavior string

// As defined in ShelfAutoHideBehavior here:
// https://cs.chromium.org/chromium/src/ash/public/cpp/shelf_types.h
const (
	// shelfBehaviorAlwaysAutoHide represents always auto-hide.
	shelfBehaviorAlwaysAutoHide shelfBehavior = "always"
	//shelfBehaviorNeverAutoHide represents never auto-hide, meaning that it is always visible.
	shelfBehaviorNeverAutoHide = "never"
	// shelfBehaviorHidden represents always hidden, used for debugging, since this state is not exposed to the user.
	shelfBehaviorHidden = "hidden"
	// shelfBehaviorInvalid represents an invalid state.
	shelfBehaviorInvalid = "invalid"
)

// setShelfBehavior sets the shelf visibility behavior.
// displayID is the display that contains the shelf.
func setShelfBehavior(ctx context.Context, c *chrome.Conn, displayID string, b shelfBehavior) error {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.setShelfAutoHideBehavior(%q, %q, function() {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve();
		    }
		  });
		})`, displayID, b)
	return c.EvalPromise(ctx, expr, nil)
}

// getShelfBehavior returns the shelf visibility behavior.
// displayID is the display that contains the shelf.
func getShelfBehavior(ctx context.Context, c *chrome.Conn, displayID string) (shelfBehavior, error) {
	var b shelfBehavior
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.getShelfAutoHideBehavior(%q, function(behavior) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve(behavior);
		    }
		  });
		})`, displayID)
	if err := c.EvalPromise(ctx, expr, &b); err != nil {
		return shelfBehaviorInvalid, err
	}
	switch b {
	case shelfBehaviorAlwaysAutoHide, shelfBehaviorNeverAutoHide, shelfBehaviorHidden:
	default:
		return shelfBehaviorInvalid, errors.Errorf("invalid shelf behavior %q", b)
	}
	return b, nil
}

// shelfAlignment represents the different Chrome OS shelf alignments.
type shelfAlignment string

// As defined in ShelfAlignment here:
// https://cs.chromium.org/chromium/src/ash/public/cpp/shelf_types.h
const (
	shelfAlignmentBottom       shelfAlignment = "Bottom"
	shelfAlignmentLeft                        = "Left"
	shelfAlignmentRight                       = "Right"
	shelfAlignmentBottomLocked                = "BottomLocked"
	shelfAlignmentInvalid                     = "Invalid"
)

// setShelfAlignment sets the shelf alignment.
// displayID is the display that contains the shelf.
func setShelfAlignment(ctx context.Context, c *chrome.Conn, displayID string, a shelfAlignment) error {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.setShelfAlignment(%q, %q, function() {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve();
		    }
		  });
		})`, displayID, a)
	return c.EvalPromise(ctx, expr, nil)
}

// getShelfAlignment returns the shelf alignment.
// displayID is the display that contains the shelf.
func getShelfAlignment(ctx context.Context, c *chrome.Conn, displayID string) (shelfAlignment, error) {
	var a shelfAlignment
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.getShelfAlignment(%q, function(alignment) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve(alignment);
		    }
		  });
		})`, displayID)
	if err := c.EvalPromise(ctx, expr, &a); err != nil {
		return shelfAlignmentInvalid, err
	}
	switch a {
	case shelfAlignmentBottom, shelfAlignmentLeft, shelfAlignmentRight, shelfAlignmentBottomLocked:
	default:
		return shelfAlignmentInvalid, errors.Errorf("invalid shelf alignment %q", a)
	}
	return a, nil
}

// getShelfRect returns Chrome OS's shelf rect, in DPs.
func getShelfRect(ctx context.Context, tconn *chrome.Conn) (arc.Rect, error) {
	var r arc.Rect
	err := tconn.EvalPromise(ctx,
		`new Promise(function(resolve, reject) {
		  chrome.automation.getDesktop(function(root) {
		    const appWindow = root.find({attributes: {className: 'ShelfWidget'}});
		    if (!appWindow) {
		      reject(new Error("Failed to locate ShelfWidget"));
		    } else {
		      resolve(appWindow.location);
		    }
		  })
		})`, &r)
	return r, err
}

// getStatusAreaRect returns Chrome OS's Status Area rect, in DPs.
// Returns error if Status Area is not present.
func getStatusAreaRect(ctx context.Context, tconn *chrome.Conn) (arc.Rect, error) {
	var r arc.Rect
	err := tconn.EvalPromise(ctx,
		`new Promise(function(resolve, reject) {
		  chrome.automation.getDesktop(function(root) {
		    const appWindow = root.find({attributes: {className: 'BubbleFrameView'}});
		    if (!appWindow) {
		      reject(new Error("Failed to locate BubbleFrameView"));
		    } else {
		      resolve(appWindow.location);
		    }
		  })
		})`, &r)
	return r, err
}

// showSystemStatusArea shows the System Status Area in case it is not already shown.
func showSystemStatusArea(ctx context.Context, tconn *chrome.Conn) error {
	// Already visible ?
	if _, err := getStatusAreaRect(ctx, tconn); err == nil {
		return nil
	}

	if err := toggleSystemStatusArea(ctx, tconn); err != nil {
		return err
	}

	// Verify that it is visible.
	if _, err := getStatusAreaRect(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to show the Status Area")
	}
	return nil
}

// hideSystemStatusArea hides the System Status Area in case it is not already hidden.
func hideSystemStatusArea(ctx context.Context, tconn *chrome.Conn) error {
	// Already hidden ?
	if _, err := getStatusAreaRect(ctx, tconn); err != nil {
		return nil
	}

	if err := toggleSystemStatusArea(ctx, tconn); err != nil {
		return err
	}

	// Verify that it is hidden.
	if _, err := getStatusAreaRect(ctx, tconn); err == nil {
		return errors.New("failed to hide the Status Area")
	}
	return nil
}

// toggleSystemStatusArea toggles Chrome OS's system status area.
func toggleSystemStatusArea(ctx context.Context, tconn *chrome.Conn) error {
	// A reliable way to toggle the status area is by injecting Alt+Shift+s. But on tablet mode
	// it doesn't work since the keyboard is disabled.
	// Instead, we inject a touch event in the StatusAreaWidget's button. The problem is that in tablet mode
	// there are two buttons and we cannot identify them in a reliable way. We assume that the first button
	// in the StatusAreaWidget hierarchy is the one that toggles the status area.
	// TODO(ricardoq): Find a reliable way to find "status tray" button.

	var r arc.Rect
	err := tconn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
		  chrome.automation.getDesktop(function(root) {
		    const areaWidget = root.find({ attributes: { className: 'StatusAreaWidget'}});
		    if (!areaWidget) {
		      reject("Failed to locate StatusAreaWidget");
		      return;
		    }
		    const button = areaWidget.find({ attributes: { role: 'button'}})
		    if (!button) {
		      reject("Failed to locate button in StatusAreaWidget");
		      return;
		    }
		    resolve(button.location);
		  })
		})`, &r)
	if err != nil {
		return errors.Wrap(err, "failed to find StatusAreaWidget")
	}

	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open touchscreen device")
	}
	defer tsw.Close()

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "could not create TouchEventWriter")
	}
	defer stw.Close()

	dispMode, err := getInternalDisplayMode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}

	// Inject touch at button's center. Coordinates coming from Chrome are in DPs.
	x := float64(r.Left+r.Width/2) * dispMode.DeviceScaleFactor
	y := float64(r.Top+r.Height/2) * dispMode.DeviceScaleFactor

	// Calculate Pixel (screen display) / Tuxel (touch device) ratio.
	pixelToTuxelX := float64(tsw.Width()) / float64(dispMode.WidthInNativePixels)
	pixelToTuxelY := float64(tsw.Height()) / float64(dispMode.HeightInNativePixels)

	if err := stw.Move(input.TouchCoord(x*pixelToTuxelX), input.TouchCoord(y*pixelToTuxelY)); err != nil {
		return err
	}
	const touchDuration = 100 * time.Millisecond
	if err := testing.Sleep(ctx, touchDuration); err != nil {
		return err
	}
	return stw.End()
}
