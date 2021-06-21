// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// Activity holds resources associated with an ARC activity.
type Activity struct {
	a            *ARC // Close is not called here
	pkgName      string
	activityName string
	disp         *Display
	tew          *input.TouchscreenEventWriter // nil until first use
}

// BorderType represents the 8 different border types that a window can have.
type BorderType uint

const (
	// BorderTop is the top border.
	BorderTop BorderType = 1 << 0
	// BorderBottom is the bottom border.
	BorderBottom = 1 << 1
	// BorderLeft is the left border.
	BorderLeft = 1 << 2
	// BorderRight is the right border.
	BorderRight = 1 << 3
	// BorderTopLeft is the top-left corner.
	BorderTopLeft = (BorderTop | BorderLeft)
	// BorderTopRight is the top-right corner.
	BorderTopRight = (BorderTop | BorderRight)
	// BorderBottomLeft is the bottom-left corner.
	BorderBottomLeft = (BorderBottom | BorderLeft)
	// BorderBottomRight is the bottom-right corner.
	BorderBottomRight = (BorderBottom | BorderRight)
)

// WindowState represents the different states a window can have.
type WindowState int

// Constants taken from WindowPositioner.java. See:
// http://cs/pi-arc-dev/frameworks/base/services/core/arc/java/com/android/server/am/WindowPositioner.java
const (
	// WindowStateNormal represents the "not maximized" state, but users can maximize it if they want.
	WindowStateNormal WindowState = 0
	// WindowStateMaximized is the maximized window state.
	WindowStateMaximized WindowState = 1
	// WindowStateFullscreen is the fullscreen window state.
	WindowStateFullscreen WindowState = 2
	// WindowStateMinimized is the minimized window state.
	WindowStateMinimized WindowState = 3
	// WindowStatePrimarySnapped is the primary snapped state.
	WindowStatePrimarySnapped WindowState = 4
	// WindowStateSecondarySnapped is the secondary snapped state.
	WindowStateSecondarySnapped WindowState = 5
	// WindowStatePIP is the Picture-in-Picture state.
	WindowStatePIP WindowState = 6
)

// String returns a human-readable string representation for type WindowState.
func (s WindowState) String() string {
	switch s {
	case WindowStateNormal:
		return "WindowStateNormal"
	case WindowStateMaximized:
		return "WindowStateMaximized"
	case WindowStateFullscreen:
		return "WindowStateFullscreen"
	case WindowStateMinimized:
		return "WindowStateMinimized"
	case WindowStatePrimarySnapped:
		return "WindowStatePrimarySnapped"
	case WindowStateSecondarySnapped:
		return "WindowStateSecondarySnapped"
	case WindowStatePIP:
		return "WindowStatePIP"
	default:
		return fmt.Sprintf("Unknown window state: %d", s)
	}
}

const (
	// borderOffsetForNormal represents the distance in pixels outside the border
	// at which a "normal" window should be grabbed from.
	// The value, in theory, should be between -1 (kResizeInsideBoundsSize) and
	// 30 (kResizeOutsideBoundsSize * kResizeOutsideBoundsScaleForTouch).
	// Internal tests proved that using -1 or 0 is unreliable, and values >= 1 should be used instead.
	// See: https://cs.chromium.org/chromium/src/ash/public/cpp/ash_constants.h
	borderOffsetForNormal = 5
	// borderOffsetForPIP is like borderOffsetForNormal, but for Picture-in-Picture windows.
	// PiP windows are dragged from the inside, and that's why it has a negative value.
	// The hitbox size is hardcoded to 48dp. See PipDragHandleController.isInDragHandleHitbox().
	// http://cs/pi-arc-dev/frameworks/base/packages/SystemUI/src/com/android/systemui/pip/phone/PipDragHandleController.java
	borderOffsetForPIP = -5
	// delayToPreventGesture represents the delay used in swipe() to prevent triggering gestures like "minimize".
	delayToPreventGesture = 150 * time.Millisecond
)

// NewActivity returns a new Activity instance.
// The caller is responsible for closing a.
// Returned Activity instance must be closed when the test is finished.
func NewActivity(a *ARC, pkgName, activityName string) (*Activity, error) {
	disp, err := NewDisplay(a, DefaultDisplayID)
	if err != nil {
		return nil, errors.Wrap(err, "could not create a new Display")
	}
	return &Activity{
		a:            a,
		pkgName:      pkgName,
		activityName: activityName,
		disp:         disp,
	}, nil
}

// NewActivityOnDisplay returns a new Activity instance on specific display.
// The caller is responsible for closing a.
// Returned Activity instance must be closed when the test is finished.
func NewActivityOnDisplay(a *ARC, pkgName, activityName string, displayID int) (*Activity, error) {
	disp, err := NewDisplay(a, displayID)
	if err != nil {
		return nil, errors.Wrap(err, "could not create a new Display")
	}
	return &Activity{
		a:            a,
		pkgName:      pkgName,
		activityName: activityName,
		disp:         disp,
	}, nil
}

// Start starts the activity by invoking "am start" and waits for it to be visible on the Chrome side.
func (ac *Activity) Start(ctx context.Context, tconn *chrome.TestConn) error {
	cmd := ac.a.Command(ctx, "am", "start", "--display", strconv.Itoa(ac.disp.DisplayID), "-W", ac.pkgName+"/"+ac.activityName)
	return ac.startHelper(ctx, tconn, cmd)
}

// StartWithArgs starts the activity by invoking "am start" with prefixes and suffixes
// to pkgName/activityName. This is useful for intent arguments.
// https://developer.android.com/studio/command-line/adb.html#IntentSpec
func (ac *Activity) StartWithArgs(ctx context.Context, tconn *chrome.TestConn, prefixes, suffixes []string) error {
	args := []string{"start"}
	args = append(args, prefixes...)
	args = append(args, ac.pkgName+"/"+ac.activityName)
	args = append(args, suffixes...)
	cmd := ac.a.Command(ctx, "am", args...)
	return ac.startHelper(ctx, tconn, cmd)
}

// startHelper starts the activity by invoking "am start".
func (ac *Activity) startHelper(ctx context.Context, tconn *chrome.TestConn, cmd *testexec.Cmd) error {
	output, err := cmd.Output()
	if err != nil {
		return errors.Wrap(err, "failed to start activity")
	}
	// "adb shell" doesn't distinguish between a failed/successful run. For that we have to parse the output.
	// Looking for:
	//  Starting: Intent { act=android.intent.action.MAIN cat=[android.intent.category.LAUNCHER] cmp=com.example.name/.ActvityName }
	//  Error type 3
	//  Error: Activity class {com.example.name/com.example.name.ActvityName} does not exist.
	re := regexp.MustCompile(`(?m)^Error:\s*(.*)$`)
	groups := re.FindStringSubmatch(string(output))
	if len(groups) == 2 {
		testing.ContextLog(ctx, "Failed to start activity: ", groups[1])
		return errors.New("failed to start activity")
	}

	if err := ash.WaitForVisible(ctx, tconn, ac.PackageName()); err != nil {
		return errors.Wrap(err, "failed to wait for visible activity")
	}
	return nil
}

// Stop stops the activity by invoking "am force-stop" with the package name.
// If there are multiple activities that belong to the same package name, all of
// them will be stopped.
func (ac *Activity) Stop(ctx context.Context, tconn *chrome.TestConn) error {
	// "adb shell am force-stop" has no output. So the error from Run() is returned.
	if err := ac.a.Command(ctx, "am", "force-stop", ac.pkgName).Run(); err != nil {
		return errors.Wrap(err, "failed to stop activity")
	}
	if err := ash.WaitForHidden(ctx, tconn, ac.pkgName); err != nil {
		return errors.Wrap(err, "failed to wait for the activity to be dismissed")
	}
	return nil
}

// WindowBounds returns the window bounding box of the activity in pixels.
// The caption bounds, in case it is present, is included as part of the window bounds.
// This is the same size as the one reported by Chrome/Aura.
// See: SurfaceBounds
func (ac *Activity) WindowBounds(ctx context.Context) (coords.Rect, error) {
	t, err := ac.getTaskInfo(ctx)
	if err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to get task info")
	}

	// Fullscreen and maximized windows already include the caption height. PiP windows don't have caption.
	if t.windowState == WindowStateFullscreen ||
		t.windowState == WindowStateMaximized ||
		t.windowState == WindowStatePIP ||
		// TODO(b/141175230): Freeform windows should not include caption. Remove check once bug gets fixed.
		(t.windowState == WindowStateNormal && t.Bounds.Top == 0) {
		return t.Bounds, nil
	}

	// But the rest must have the caption height added to their bounds.
	captionHeight, err := ac.disp.CaptionHeight(ctx)
	if err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to get caption height")
	}
	t.Bounds.Top -= captionHeight
	t.Bounds.Height += captionHeight
	return t.Bounds, nil
}

// SurfaceBounds returns the surface bounds in pixels. A surface represents the buffer used to store
// the window content. This is the buffer used by SurfaceFlinger and Wayland.
// The surface bounds might be smaller than the window bounds since the surface does not
// include the caption.
// And does not include the shelf size if the activity is fullscreen/maximized and the shelf is in "always show" mode.
// See: WindowBounds
func (ac *Activity) SurfaceBounds(ctx context.Context) (coords.Rect, error) {
	t, err := ac.getTaskInfo(ctx)
	if err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to get task info")
	}
	return t.Bounds, nil
}

// Close closes the resources associated with the Activity instance.
// Calling Close() does not stop the activity.
func (ac *Activity) Close() {
	ac.disp.Close()
	if ac.tew != nil {
		ac.tew.Close()
	}
}

// MoveWindow moves the activity's window to a new location.
// t represents the duration of the movement.
// toBounds represent the destination bounds (in px).
// fromBounds represent the source bounds (in px).
func (ac *Activity) MoveWindow(ctx context.Context, tconn *chrome.TestConn, t time.Duration, toBounds, fromBounds coords.Rect) error {
	sdkVer, err := SDKVersion()
	if err != nil {
		return errors.Wrap(err, "failed to get the SDK version")
	}

	switch sdkVer {
	case SDKP:
		return ac.moveWindowP(ctx, coords.NewPoint(toBounds.Left, toBounds.Top), t)
	case SDKR:
		return ac.moveWindowR(ctx, tconn, t, toBounds, fromBounds)
	default:
		return errors.Errorf("unsupported SDK version: %d", sdkVer)
	}
}

// moveWindowP moves the activity's window to a new location.
// to represents the coordinates (top-left) for the new position, in pixels.
// t represents the duration of the movement.
// moveWindowP only works with WindowStateNormal and WindowStatePIP windows. Will fail otherwise.
// moveWindowP performs the movement by injecting Touch events in the kernel.
// If the device does not have a touchscreen, it will fail.
func (ac *Activity) moveWindowP(ctx context.Context, to coords.Point, t time.Duration) error {
	task, err := ac.getTaskInfo(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get task info")
	}

	if task.windowState != WindowStateNormal && task.windowState != WindowStatePIP {
		return errors.Errorf("cannot move window in state %d", int(task.windowState))
	}

	bounds, err := ac.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get activity bounds")
	}

	var from coords.Point
	captionHeight, err := ac.disp.CaptionHeight(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get caption height")
	}
	halfWidth := bounds.Width / 2
	from.X = bounds.Left + halfWidth
	to.X += halfWidth
	if task.windowState == WindowStatePIP {
		// PiP windows are dragged from its center
		halfHeight := bounds.Height / 2
		from.Y = bounds.Top + halfHeight
		to.Y += halfHeight
	} else {
		// Normal-state windows are dragged from its caption
		from.Y = bounds.Top + captionHeight/2
		to.Y += captionHeight / 2
	}
	return ac.swipe(ctx, from, to, t)
}

// moveWindowR moves the activity's window to a new location.
// t represents the duration of the movement.
// toBounds represent the destination bounds (in px).
// fromBounds represent the source bounds (in px).
// moveWindowR only works with WindowStateNormal and WindowStatePIP windows. Will fail otherwise.
// moveWindowR performs the movement using a mouse drag.
func (ac *Activity) moveWindowR(ctx context.Context, tconn *chrome.TestConn, t time.Duration, toBounds, fromBounds coords.Rect) error {
	windowStates, err := ash.GetAllARCAppWindowStates(ctx, tconn, ac.PackageName())
	if err != nil {
		return errors.Wrap(err, "could not get app window state")
	}

	supportsMove := false
	for _, windowState := range windowStates {
		if windowState == ash.WindowStatePIP || windowState == ash.WindowStateNormal {
			supportsMove = true
			break
		}
	}
	if !supportsMove {
		return errors.New("move window only supports Normal and PIP windows")
	}

	// We'll drag the window from the top-left quadrant.
	from := coords.NewPoint(fromBounds.Left+(fromBounds.Width/4), fromBounds.Top+(fromBounds.Height/4))
	to := coords.NewPoint(toBounds.Left+(toBounds.Width/4), toBounds.Top+(toBounds.Height/4))

	dispMode, err := ash.InternalDisplayMode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}
	dsf := dispMode.DeviceScaleFactor

	// Convert points back to dp to perform drag.
	from.X = int(math.Round(float64(from.X) / dsf))
	from.Y = int(math.Round(float64(from.Y) / dsf))
	to.X = int(math.Round(float64(to.X) / dsf))
	to.Y = int(math.Round(float64(to.Y) / dsf))

	// There needs to be a brief pause before the drag or the mouse won't pick up the pip window.
	return dragWithPause(ctx, tconn, from, to, t)
}

// ResizeWindow resizes the activity's window.
// border represents from where the resize should start.
// to represents the coordinates for for the new border's position, in pixels.
// t represents the duration of the resize.
// ResizeWindow only works with WindowStateNormal and WindowStatePIP windows. Will fail otherwise.
// For PiP windows, they must have the PiP Menu Activity displayed. Will fail otherwise.
func (ac *Activity) ResizeWindow(ctx context.Context, tconn *chrome.TestConn, border BorderType, to coords.Point, t time.Duration) error {
	sdkVer, err := SDKVersion()
	if err != nil {
		return errors.Wrap(err, "failed to get the SDK version")
	}

	switch sdkVer {
	case SDKP:
		if err := ac.resizeWindowP(ctx, border, to, time.Second); err != nil {
			return errors.Wrap(err, "could not resize window")
		}
		return nil
	case SDKR:
		if err := ac.resizeWindowR(ctx, tconn, border, to, time.Second); err != nil {
			return errors.Wrap(err, "could not resize window")
		}
		return nil
	default:
		return errors.Errorf("unsupported SDK version: %d", sdkVer)
	}
}

// resizeWindowP resizes the activity's window.
// border represents from where the resize should start.
// to represents the coordinates for for the new border's position, in pixels.
// t represents the duration of the resize.
// resizeWindowP only works with WindowStateNormal and WindowStatePIP windows. Will fail otherwise.
// For PiP windows, they must have the PiP Menu Activity displayed. Will fail otherwise.
// resizeWindowP performs the resizing by injecting Touch events in the kernel.
// If the device does not have a touchscreen, it will fail.
func (ac *Activity) resizeWindowP(ctx context.Context, border BorderType, to coords.Point, t time.Duration) error {
	task, err := ac.getTaskInfo(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get task info")
	}

	if task.windowState != WindowStateNormal && task.windowState != WindowStatePIP {
		return errors.Errorf("cannot move window in state %d", int(task.windowState))
	}

	// Default value: center of window.
	bounds, err := ac.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get activity bounds")
	}
	src := bounds.CenterPoint()

	borderOffset := borderOffsetForNormal
	if task.windowState == WindowStatePIP {
		borderOffset = borderOffsetForPIP
	}

	// Top & Bottom are exclusive.
	if border&BorderTop != 0 {
		src.Y = bounds.Top - borderOffset
	} else if border&BorderBottom != 0 {
		src.Y = bounds.Top + bounds.Height + borderOffset
	}

	// Left & Right are exclusive.
	if border&BorderLeft != 0 {
		src.X = bounds.Left - borderOffset
	} else if border&BorderRight != 0 {
		src.X = bounds.Left + bounds.Width + borderOffset
	}

	// After updating src, clamp it to valid display bounds.
	ds, err := ac.disp.Size(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get display size")
	}
	src.X = int(math.Max(0, math.Min(float64(ds.Width-1), float64(src.X))))
	src.Y = int(math.Max(0, math.Min(float64(ds.Height-1), float64(src.Y))))

	return ac.swipe(ctx, src, to, t)
}

// resizeWindowR resizes the activity's window.
// border represents from where the resize should start.
// to represents the coordinates for for the new border's position, in pixels.
// t represents the duration of the resize.
// resizeWindowR only works with WindowStateNormal and WindowStatePIP windows. Will fail otherwise.
// resizeWindowR performs the resizing using a mouse drag.
func (ac *Activity) resizeWindowR(ctx context.Context, tconn *chrome.TestConn, border BorderType, to coords.Point, t time.Duration) error {
	windowStates, err := ash.GetAllARCAppWindowStates(ctx, tconn, ac.PackageName())
	if err != nil {
		return errors.Wrap(err, "could not get app window state")
	}

	supportsMove := false
	hasPIPWindow := false
	for _, windowState := range windowStates {
		if windowState == ash.WindowStatePIP {
			hasPIPWindow = true
		}
		if windowState == ash.WindowStatePIP || windowState == ash.WindowStateNormal {
			supportsMove = true
			break
		}
	}
	if !supportsMove {
		return errors.New("resize window only supports Normal and PIP windows")
	}

	// Default value: center of window.
	bounds, err := ac.WindowBounds(ctx)
	src := bounds.CenterPoint()

	borderOffset := borderOffsetForNormal
	if hasPIPWindow {
		borderOffset = borderOffsetForPIP
	}

	// Top & Bottom are exclusive.
	if border&BorderTop != 0 {
		src.Y = bounds.Top - borderOffset
	} else if border&BorderBottom != 0 {
		src.Y = bounds.Top + bounds.Height + borderOffset
	}

	// Left & Right are exclusive.
	if border&BorderLeft != 0 {
		src.X = bounds.Left - borderOffset
	} else if border&BorderRight != 0 {
		src.X = bounds.Left + bounds.Width + borderOffset
	}

	dispMode, err := ash.InternalDisplayMode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}
	displaySize := coords.NewSize(dispMode.WidthInNativePixels, dispMode.WidthInNativePixels)
	dsf := dispMode.DeviceScaleFactor
	// After updating src, clamp it to valid display bounds.
	src.X = int(math.Max(0, math.Min(float64(displaySize.Width-1), float64(src.X))))
	src.Y = int(math.Max(0, math.Min(float64(displaySize.Height-1), float64(src.Y))))

	// Convert points back to dp to perform drag.
	src.X = int(math.Round(float64(src.X) / dsf))
	src.Y = int(math.Round(float64(src.Y) / dsf))
	to.X = int(math.Round(float64(to.X) / dsf))
	to.Y = int(math.Round(float64(to.Y) / dsf))

	return mouse.Drag(ctx, tconn, src, to, t)
}

// SetWindowState sets the window state. Note this method is async, so ensure to call ash.WaitForArcAppWindowState after this.
// Supported states: WindowStateNormal, WindowStateMaximized, WindowStateFullscreen, WindowStateMinimized
func (ac *Activity) SetWindowState(ctx context.Context, tconn *chrome.TestConn, state WindowState) error {
	sdkVer, err := SDKVersion()
	if err != nil {
		return errors.Wrap(err, "failed to get the SDK version")
	}

	switch sdkVer {
	case SDKP:
		return ac.setWindowStateP(ctx, state)
	case SDKR:
		return ac.setWindowStateR(ctx, tconn, state)
	case SDKS:
		return ac.setWindowStateS(ctx, tconn, state)
	default:
		return errors.Errorf("unsupported SDK version: %d", sdkVer)
	}
}

// setWindowStateP sets the window state. Note this method is async, so ensure to call ash.WaitForArcAppWindowState after this.
// Supported states: WindowStateNormal, WindowStateMaximized, WindowStateFullscreen, WindowStateMinimized
func (ac *Activity) setWindowStateP(ctx context.Context, state WindowState) error {
	t, err := ac.getTaskInfo(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get task info")
	}

	switch state {
	case WindowStateNormal, WindowStateMaximized, WindowStateFullscreen, WindowStateMinimized:
	default:
		return errors.Errorf("unsupported window state %d", state)
	}

	if err = ac.a.Command(ctx, "am", "task", "set-winstate", strconv.Itoa(t.ID), strconv.Itoa(int(state))).Run(); err != nil {
		return errors.Wrap(err, "could not execute 'am task set-winstate'")
	}
	return nil
}

// setWindowStateR sets the window state. Note this method is async, so ensure to call ash.WaitForArcAppWindowState after this.
// Supported states: WindowStateNormal, WindowStateMaximized, WindowStateFullscreen, WindowStateMinimized
func (ac *Activity) setWindowStateR(ctx context.Context, tconn *chrome.TestConn, state WindowState) error {
	switch state {
	case WindowStateNormal, WindowStateMaximized, WindowStateFullscreen, WindowStateMinimized:
	default:
		return errors.Errorf("unsupported window state %d", state)
	}

	wmEvent, err := windowStateToWMEvent(state)
	if err != nil {
		return errors.Wrap(err, "failed to get wm event")
	}

	if _, err := ash.SetARCAppWindowState(ctx, tconn, ac.PackageName(), wmEvent); err != nil {
		return errors.Wrap(err, "failed to set window state")
	}
	return nil
}

// setWindowStateS sets the window state. Note this method is async, so ensure to call ash.WaitForArcAppWindowState after this.
// Supported states: WindowStateNormal, WindowStateMaximized, WindowStateFullscreen, WindowStateMinimized
func (ac *Activity) setWindowStateS(ctx context.Context, tconn *chrome.TestConn, state WindowState) error {
	// Delegate to R version because there isn't significant difference.
	return ac.setWindowStateR(ctx, tconn, state)
}

func windowStateToWMEvent(state WindowState) (ash.WMEventType, error) {
	switch state {
	case WindowStateNormal:
		return ash.WMEventNormal, nil
	case WindowStateMaximized:
		return ash.WMEventMaximize, nil
	case WindowStateMinimized:
		return ash.WMEventMinimize, nil
	case WindowStateFullscreen:
		return ash.WMEventFullscreen, nil
	default:
		return ash.WMEventNormal, errors.Errorf("unsupported window state %d", state)
	}
}

// GetWindowState returns the window state.
func (ac *Activity) GetWindowState(ctx context.Context) (WindowState, error) {
	task, err := ac.getTaskInfo(ctx)
	if err != nil {
		return WindowStateNormal, errors.Wrap(err, "could not get task info")
	}
	return task.windowState, nil
}

// CaptionHeight returns the caption height of the activity.
func (ac *Activity) CaptionHeight(ctx context.Context) (int, error) {
	height, err := ac.disp.CaptionHeight(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "could not get caption height")
	}
	return height, nil
}

// DisplayDensity returns the density of activity's physical display.
func (ac *Activity) DisplayDensity(ctx context.Context) (float64, error) {
	density, err := ac.disp.PhysicalDensity(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "could not get density")
	}
	return density, nil
}

// WaitForFinished waits till all the activities beloninging to this task are
// inactive. Active means anywhere between activity launched and activity shut
// down in the activity lifecycle. This function cannot tell if the activity was
// launched at all.
//
// Activity lifecycle:
// https://developer.android.com/guide/components/activities/activity-lifecycle#alc
func (ac *Activity) WaitForFinished(ctx context.Context, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		_, err := ac.getTaskInfo(ctx)
		if errors.Is(err, errNoTaskInfo) {
			return nil
		}
		return errors.New("activity is still active")
	}, &testing.PollOptions{Timeout: timeout})
}

// IsRunning returns true if the activity is running, false otherwise.
func (ac *Activity) IsRunning(ctx context.Context) (bool, error) {
	if _, err := ac.getTaskInfo(ctx); err != nil {
		if errors.Is(err, errNoTaskInfo) {
			return false, nil
		}
		return false, errors.Wrap(err, "cannot tell if the activity is running")
	}
	return true, nil
}

// PackageName returns the activity package name.
func (ac *Activity) PackageName() string {
	return ac.pkgName
}

// Resizable returns the window resizability.
func (ac *Activity) Resizable(ctx context.Context) (bool, error) {
	task, err := ac.getTaskInfo(ctx)
	if err != nil {
		return false, errors.Wrap(err, "could not get task info")
	}
	return task.resizable, nil
}

// swipe injects touch events in a straight line. The line is defined by from and to, in pixels.
// t represents the duration of the swipe.
// The last touch event will be held in its position for a few ms to prevent triggering "minimize" or similar gestures.
func (ac *Activity) swipe(ctx context.Context, from, to coords.Point, t time.Duration) error {
	if err := ac.initTouchscreenLazily(ctx); err != nil {
		return errors.Wrap(err, "could not initialize touchscreen device")
	}

	stw, err := ac.tew.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "could not get a new TouchEventWriter")
	}
	defer stw.Close()

	// TODO(ricardoq): Fetch stableSize directly from Chrome OS, and not from Android.
	// It is not clear whether Android can have a display bounds different than Chrome OS.
	// Using "non-rotated" display bounds for calculating the scale factor since
	// touchscreen bounds are also "non-rotated".
	dispSize, err := ac.disp.stableSize(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get stable bounds for display")
	}

	tcc := ac.tew.NewTouchCoordConverter(dispSize)

	fromX, fromY := tcc.ConvertLocation(from)
	toX, toY := tcc.ConvertLocation(to)
	if err := stw.Swipe(ctx, fromX, fromY, toX, toY, t); err != nil {
		return errors.Wrap(err, "failed to start the swipe gesture")
	}

	if err := testing.Sleep(ctx, delayToPreventGesture); err != nil {
		return errors.Wrap(err, "timeout while sleeping")
	}
	return nil
}

// initTouchscreenLazily lazily initializes the touchscreen.
// Touchscreen initialization is not needed, unless swipe() is called.
func (ac *Activity) initTouchscreenLazily(ctx context.Context) error {
	if ac.tew != nil {
		return nil
	}
	var err error
	ac.tew, err = input.Touchscreen(ctx)
	if err != nil {
		return errors.Wrap(err, "could not open touchscreen device")
	}
	return nil
}

var errNoTaskInfo = errors.New("no task info")

// getTaskInfo returns the task record associated for the current activity.
func (ac *Activity) getTaskInfo(ctx context.Context) (TaskInfo, error) {
	tasks, err := ac.a.TaskInfosFromDumpsys(ctx)
	if err != nil {
		return TaskInfo{}, errors.Wrap(err, "could not get task info")
	}
	for _, task := range tasks {
		for _, activity := range task.ActivityInfos {
			if activity.PackageName == ac.pkgName {
				qualifiedName := activity.PackageName + activity.ActivityName
				if activity.ActivityName == ac.activityName || qualifiedName == ac.activityName {
					return task, nil
				}
			}
		}
	}
	return TaskInfo{}, errors.Wrapf(errNoTaskInfo, "could not find task info for %s/%s", ac.pkgName, ac.activityName)
}

// PackageResizable returns the window resizability of an app package name.
func (ac *Activity) PackageResizable(ctx context.Context) (bool, error) {
	task, err := ac.getPackageTaskInfo(ctx)
	if err != nil {
		return false, errors.Wrap(err, "could not get task info")
	}
	return task.resizable, nil
}

// getPackageTaskInfo returns the task record associated for an app package name.
func (ac *Activity) getPackageTaskInfo(ctx context.Context) (TaskInfo, error) {
	tasks, err := ac.a.TaskInfosFromDumpsys(ctx)
	if err != nil {
		return TaskInfo{}, errors.Wrap(err, "could not get task info")
	}
	for _, task := range tasks {
		for _, activity := range task.ActivityInfos {
			if activity.PackageName == ac.pkgName {
				return task, nil
			}
		}
	}
	return TaskInfo{}, errors.Wrapf(errNoTaskInfo, "could not find task info for %s", ac.pkgName)
}

// Focused returns whether the app's window has focus or not.
// Only works on ARC++ R and later.
func (ac *Activity) Focused(ctx context.Context) (bool, error) {
	n, err := SDKVersion()
	if err != nil {
		return false, errors.Wrap(err, "failed to get the SDK version")
	}
	if n < SDKR {
		return false, errors.Errorf("getting the focused state is not implemented for SDK %d", n)
	}
	task, err := ac.getTaskInfo(ctx)
	if err != nil {
		return false, errors.Wrap(err, "could not get task info")
	}
	return task.focused, nil
}

// Focus focuses the activity.
func (ac *Activity) Focus(ctx context.Context, tconn *chrome.TestConn) error {
	if err := tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.setArcAppWindowFocus)", ac.pkgName); err != nil {
		return errors.Wrap(err, "failed to call setArcAppWindowFocus")
	}
	return nil
}

// dragWithPause performs a regular mouse drag with a brief pause before pressing and moving.
func dragWithPause(ctx context.Context, tconn *chrome.TestConn, from, to coords.Point, t time.Duration) (firstErr error) {
	if firstErr := mouse.Move(ctx, tconn, from, 0); firstErr != nil {
		return firstErr
	}
	if firstErr := testing.Sleep(ctx, time.Second); firstErr != nil {
		return firstErr
	}
	if firstErr := mouse.Press(ctx, tconn, mouse.LeftButton); firstErr != nil {
		return firstErr
	}
	defer func() {
		if err := mouse.Release(ctx, tconn, mouse.LeftButton); err != nil {
			if firstErr == nil {
				firstErr = err
			} else {
				testing.ContextLog(ctx, "Failed to release mouse left button: ", err)
			}
		}
	}()
	return mouse.Move(ctx, tconn, to, t)
}

// ToAshWindowState returns equivalent ash WindowStateType for the arc WindowState.
func (s WindowState) ToAshWindowState() (ash.WindowStateType, error) {
	switch s {
	case WindowStateNormal:
		return ash.WindowStateNormal, nil
	case WindowStateMaximized:
		return ash.WindowStateMaximized, nil
	case WindowStateFullscreen:
		return ash.WindowStateFullscreen, nil
	case WindowStateMinimized:
		return ash.WindowStateMinimized, nil
	case WindowStatePrimarySnapped:
		return ash.WindowStateLeftSnapped, nil
	case WindowStateSecondarySnapped:
		return ash.WindowStateRightSnapped, nil
	case WindowStatePIP:
		return ash.WindowStatePIP, nil
	default:
		return ash.WindowStateNormal, errors.Errorf("unknown window state: %d", s)
	}
}
