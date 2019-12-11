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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
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
	// The hitbox size is harcoded to 48dp. See PipDragHandleController.isInDragHandleHitbox().
	// http://cs/pi-arc-dev/frameworks/base/packages/SystemUI/src/com/android/systemui/pip/phone/PipDragHandleController.java
	borderOffsetForPIP = -5
	// delayToPreventGesture represents the delay used in swipe() to prevent triggering gestures like "minimize".
	delayToPreventGesture = 150 * time.Millisecond
)

// Point represents an point.
type Point struct {
	// X and Y are the point coordinates.
	X, Y int
}

// NewPoint returns a new Point.
func NewPoint(x, y int) Point {
	return Point{x, y}
}

// String returns the string representation of Point.
func (p *Point) String() string {
	return fmt.Sprintf("(%d, %d)", p.X, p.Y)
}

// Rect represents a rectangle, as defined here:
// https://developers.chrome.com/extensions/automation#type-Rect
type Rect struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// String returns the string representation of Rect.
func (r *Rect) String() string {
	return fmt.Sprintf("(%d, %d) - (%d x %d)", r.Left, r.Top, r.Width, r.Height)
}

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

// Start starts the activity by invoking "am start".
func (ac *Activity) Start(ctx context.Context) error {
	cmd := ac.a.Command(ctx, "am", "start", "-W", ac.pkgName+"/"+ac.activityName)
	return ac.startHelper(ctx, cmd)
}

// Start starts the activity by invoking "am start" with prefixes and suffixes
// to pkgName/activityName. This is useful for intent arguments.
// https://developer.android.com/studio/command-line/adb.html#IntentSpec
func (ac *Activity) StartWithArgs(ctx context.Context, prefixes, suffixes []string) error {
	args := []string{"start"}
	args = append(args, prefixes...)
	args = append(args, ac.pkgName+"/"+ac.activityName)
	args = append(args, suffixes...)
	cmd := ac.a.Command(ctx, "am", args...)
	return ac.startHelper(ctx, cmd)
}

// Start starts the activity by invoking "am start".
func (ac *Activity) startHelper(ctx context.Context, cmd *testexec.Cmd) error {
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
	return nil
}

// Stop stops the activity by invoking "am force-stop" with the package name.
// If there are multiple activities that belong to the same package name, all of
// them will be stopped.
func (ac *Activity) Stop(ctx context.Context) error {
	// "adb shell am force-stop" has no output. So the error from Run() is returned.
	return ac.a.Command(ctx, "am", "force-stop", ac.pkgName).Run()
}

// WindowBounds returns the window bounding box of the activity in pixels.
// The caption bounds, in case it is present, is included as part of the window bounds.
// This is the same size as the one reported by Chrome/Aura.
// See: SurfaceBounds
func (ac *Activity) WindowBounds(ctx context.Context) (Rect, error) {
	t, err := ac.getTaskInfo(ctx)
	if err != nil {
		return Rect{}, errors.Wrap(err, "failed to get task info")
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
		return Rect{}, errors.Wrap(err, "failed to get caption height")
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
func (ac *Activity) SurfaceBounds(ctx context.Context) (Rect, error) {
	t, err := ac.getTaskInfo(ctx)
	if err != nil {
		return Rect{}, errors.Wrap(err, "failed to get task info")
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
// to represents the coordinates (top-left) for the new position, in pixels.
// t represents the duration of the movement.
// MoveWindow only works with WindowStateNormal and WindowStatePIP windows. Will fail otherwise.
// MoveWindow performs the movement by injecting Touch events in the kernel.
// If the device does not have a touchscreen, it will fail.
func (ac *Activity) MoveWindow(ctx context.Context, to Point, t time.Duration) error {
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

	var from Point
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

// ResizeWindow resizes the activity's window.
// border represents from where the resize should start.
// to represents the coordinates for for the new border's position, in pixels.
// t represents the duration of the resize.
// ResizeWindow only works with WindowStateNormal and WindowStatePIP windows. Will fail otherwise.
// For PiP windows, they must have the PiP Menu Activity displayed. Will fail otherwise.
// ResizeWindow performs the resizing by injecting Touch events in the kernel.
// If the device does not have a touchscreen, it will fail.
func (ac *Activity) ResizeWindow(ctx context.Context, border BorderType, to Point, t time.Duration) error {
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
	src := Point{
		bounds.Left + bounds.Width/2,
		bounds.Top + bounds.Height/2,
	}

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
	src.X = int(math.Max(0, math.Min(float64(ds.W-1), float64(src.X))))
	src.Y = int(math.Max(0, math.Min(float64(ds.H-1), float64(src.Y))))

	return ac.swipe(ctx, src, to, t)
}

// SetWindowState sets the window state. Note this method is async, so ensure to call ash.WaitForArcAppWindowState after this.
// Supported states: WindowStateNormal, WindowStateMaximized, WindowStateFullscreen, WindowStateMinimized
func (ac *Activity) SetWindowState(ctx context.Context, state WindowState) error {
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

// WaitForResumed returns whether the activity is resumed.
// If more than one activity belonging to the same task are present, it returns the resumed state
// of the most recent one.
func (ac *Activity) WaitForResumed(ctx context.Context, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		task, err := ac.getTaskInfo(ctx)
		if err != nil {
			return err
		}
		// Examine the resumed attribute. This is set to true when an app finishes
		// resuming.
		if !task.resumed {
			return errors.New("activity is not resumed yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

// WaitForFinish waits till all the activities beloninging to this task are
// inactive. Active means anywhere between activity launched and activity shut
// down in the activity lifecycle. This function cannot tell if the activity was
// launched at all.
//
// Activity lifecycle:
// https://developer.android.com/guide/components/activities/activity-lifecycle#alc
func (ac *Activity) WaitForFinish(ctx context.Context, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		task, err := ac.getTaskInfo(ctx)
		if err != nil {
			return nil
		}
		return errors.New("activity is still active")
	}, &testing.PollOptions{Timeout: timeout})
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
func (ac *Activity) swipe(ctx context.Context, from, to Point, t time.Duration) error {
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

	// Get pixel-to-tuxel factor (tuxel == touching element).
	// Touchscreen might have different resolution than the displayscreen.
	pixelToTuxelScaleX := float64(ac.tew.Width()) / float64(dispSize.W)
	pixelToTuxelScaleY := float64(ac.tew.Height()) / float64(dispSize.H)

	if err := stw.Swipe(ctx,
		input.TouchCoord(float64(from.X)*pixelToTuxelScaleX),
		input.TouchCoord(float64(from.Y)*pixelToTuxelScaleY),
		input.TouchCoord(float64(to.X)*pixelToTuxelScaleX),
		input.TouchCoord(float64(to.Y)*pixelToTuxelScaleY),
		t); err != nil {
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

// getTaskInfo returns the task record associated for the current activity.
func (ac *Activity) getTaskInfo(ctx context.Context) (TaskInfo, error) {
	tasks, err := ac.a.DumpsysActivityActivities(ctx)
	if err != nil {
		return TaskInfo{}, errors.Wrap(err, "could not get task info")
	}
	for _, task := range tasks {
		if task.PkgName == ac.pkgName && task.ActivityName == ac.activityName {
			return task, nil
		}
	}
	return TaskInfo{}, errors.Errorf("could not find task info for %s/%s", ac.pkgName, ac.activityName)
}
