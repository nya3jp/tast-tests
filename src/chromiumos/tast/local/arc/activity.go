// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/errors"
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
	// WindowStateNormal represents the "not maximized" state, but the user can maximize it if he wants.
	WindowStateNormal WindowState = 0
	// WindowStateMaximized  is the maximized window state.
	WindowStateMaximized = 1
	// WindowStateFullscreen is the fullscreen window state.
	WindowStateFullscreen = 2
	// WindowStateMinimized is the minimized window state.
	WindowStateMinimized = 3
	// WindowStatePrimarySnapped is primary snapped state.
	WindowStatePrimarySnapped = 4
	// WindowStateSecondarySnapped is the secondary snapped state.
	WindowStateSecondarySnapped = 5
	// WindowStatePIP is the Picture-in-Picture state.
	WindowStatePIP = 6
)

// taskInfo contains the information found in TaskRecord. See:
// https://cs.corp.google.com/pi-arc-dev/frameworks/base/services/core/java/com/android/server/am/TaskRecord.java
type taskInfo struct {
	// id represents the TaskRecord ID.
	id int
	// stackID represents the stack ID.
	stackID int
	// stackSize represents how many activities are in the stack.
	stackSize int
	// bounds represents the task bounds in pixels.
	bounds Rect
	// windowState represents the window state.
	windowState WindowState
	// pkgName is the package name.
	pkgName string
	// activityName is the activity name.
	activityName string
}

const (
	// borderOffsetForNormal represents the the distance in pixels outside the border
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
	// touchFrequency is the minimum time that should elapse between touches.
	touchFrequency = 5 * time.Millisecond
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

// Rect represents a rectangle.
type Rect struct {
	Left, Top, Right, Bottom int
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
// This is the size that the activity thinks it has, although the surface size could be smaller.
// See: SurfaceBounds
func (ac *Activity) WindowBounds(ctx context.Context) (Rect, error) {
	t, err := ac.getTaskInfo(ctx)
	if err != nil {
		return Rect{}, errors.Wrap(err, "failed to get task info")
	}

	// Fullscreen windows start at 0 and already include the caption height. PiP windows don't have caption.
	// Assuming that Always-on-top + Pinned == PiP
	if t.windowState == WindowStateFullscreen || t.windowState == WindowStatePIP {
		return t.bounds, nil
	}

	// But the rest must have the caption height added to their bounds.
	captionHeight, err := ac.disp.CaptionHeight(ctx)
	if err != nil {
		return Rect{}, errors.Wrap(err, "failed to get caption height")
	}
	t.bounds.Top -= captionHeight
	return t.bounds, nil
}

// SurfaceBounds returns the surface bounds in pixels. A surface represents the buffer used to store
// the window content. This is the buffer used by SurfaceFlinger and Wayland.
// The surface bounds might be smaller than the window bounds since the surface does not
// include the caption.
// And does not include the shelf size if the activity is fullscreen/maximized and the shelf is in "always show" mode.
// See: WindowBounds
func (ac *Activity) SurfaceBounds(ctx context.Context) (Rect, error) {
	cmd := ac.a.Command(ctx, "dumpsys", "window", "windows")
	output, err := cmd.Output()
	if err != nil {
		return Rect{}, errors.Wrap(err, "failed to launch dumpsys")
	}

	// Looking for:
	//   Window #0 Window{a486f07 u0 com.android.settings/com.android.settings.Settings}:
	//     mDisplayId=0 stackId=2 mSession=Session{dd34b88 2586:1000} mClient=android.os.BinderProxy@705e146
	//     mHasSurface=true isReadyForDisplay()=true mWindowRemovalAllowed=false
	//     [...many other properties...]
	//     mFrame=[0,0][1536,1936] last=[0,0][1536,1936]
	// We are interested in "mFrame="
	regStr := `(?m)` + // Enable multiline.
		`^\s*Window #\d+ Window{\S+ \S+ ` + regexp.QuoteMeta(ac.pkgName+"/"+ac.pkgName+ac.activityName) + `}:$` + // Match our activity
		`(?:\n.*?)*` + // Skip entire lines with a non-greedy search...
		`^\s*mFrame=\[(\d+),(\d+)\]\[(\d+),(\d+)\]` // ...until we match the first mFrame=
	re := regexp.MustCompile(regStr)
	groups := re.FindStringSubmatch(string(output))
	if len(groups) != 5 {
		testing.ContextLog(ctx, string(output))
		return Rect{}, errors.New("failed to parse dumpsys output; activity not running perhaps?")
	}
	bounds, err := parseBounds(groups[1:5])
	if err != nil {
		return Rect{}, err
	}
	return bounds, nil
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
// to represents the destination of the movement in pixels (ChromeOS display coordinates).
// t represents how long the movement should last.
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

	from := Point{}
	captionHeight, err := ac.disp.CaptionHeight(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get caption height")
	}
	halfWidth := (bounds.Right - bounds.Left) / 2
	from.X = bounds.Left + halfWidth
	to.X += halfWidth
	if task.windowState == WindowStatePIP {
		// PiP windows are dragged from its center
		halfHeight := (bounds.Bottom - bounds.Top) / 2
		from.Y = bounds.Top + halfHeight
		// PiP windows don't have caption. Adjust destination accordingly.
		to.Y += halfHeight
	} else {
		// Normal-state windows are dragged from its caption
		from.Y = bounds.Top + captionHeight/2
		to.Y += captionHeight / 2
	}
	steps := int(t/touchFrequency) + 1
	return ac.swipe(ctx, from, to, steps)
}

// ResizeWindow resizes the activity's window.
// to represents the destination for the resize in pixels (ChromeOS display coordinates).
// t represents how long the resize should last.
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
	bounds := task.bounds
	src := Point{
		bounds.Left + (bounds.Right-bounds.Left)/2,
		bounds.Top + (bounds.Bottom-bounds.Top)/2,
	}

	borderOffset := borderOffsetForNormal
	if task.windowState == WindowStatePIP {
		borderOffset = borderOffsetForPIP
	}

	// Top & Bottom are exclusive.
	if border&BorderTop != 0 {
		src.Y = bounds.Top - borderOffset
	} else if border&BorderBottom != 0 {
		src.Y = bounds.Bottom + borderOffset
	}

	// Left & Right are exclusive.
	if border&BorderLeft != 0 {
		src.X = bounds.Left - borderOffset
	} else if border&BorderRight != 0 {
		src.X = bounds.Right + borderOffset
	}

	steps := int(t/touchFrequency) + 1
	return ac.swipe(ctx, src, to, steps)
}

// SetWindowState sets the window state.
// Supported states: WindowStateNormal, WindowStateMaximized, WindowStateFullscreen, WindowStateMinimized
func (ac *Activity) SetWindowState(ctx context.Context, state WindowState) error {
	t, err := ac.getTaskInfo(ctx)
	if err != nil {
		errors.Wrap(err, "could not get task info")
	}
	stateToRun := ""
	switch state {
	case WindowStateNormal:
		stateToRun = "0"
	case WindowStateMaximized:
		stateToRun = "1"
	case WindowStateFullscreen:
		stateToRun = "2"
	case WindowStateMinimized:
		stateToRun = "3"
	default:
		return errors.Errorf("unsupported window state %d", state)
	}

	if err = ac.a.Command(ctx, "am", "task", "set-winstate", strconv.Itoa(t.id), stateToRun).Run(); err != nil {
		return errors.Wrap(err, "could not execute 'am task set-winstate'")
	}
	return nil
}

// swipe injects touch events in a straight line. The line is defined by from and to.
// steps represents the number of touches that will be injected.
// The last touch event will be held in its position for a few ms to prevent triggering "minimize" or similar gestures.
func (ac *Activity) swipe(ctx context.Context, from, to Point, steps int) error {
	if err := ac.initTouchscreenLazily(ctx); err != nil {
		return errors.Wrap(err, "could not initialize touchscreen device")
	}

	stw, err := ac.tew.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "could not get a new TouchEventWriter")
	}
	defer stw.Close()

	// TODO(ricardoq): Fetch stableSize directly from ChromeOS, and not from Android.
	// It is not clear whether Android can have a display bounds different than ChromeOS.
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

	stw.Move(input.TouchCoord(float64(from.X)*pixelToTuxelScaleX),
		input.TouchCoord(float64(from.Y)*pixelToTuxelScaleY))

	if err := stw.Swipe(ctx,
		input.TouchCoord(float64(from.X)*pixelToTuxelScaleX),
		input.TouchCoord(float64(from.Y)*pixelToTuxelScaleY),
		input.TouchCoord(float64(to.X)*pixelToTuxelScaleX),
		input.TouchCoord(float64(to.Y)*pixelToTuxelScaleY),
		steps); err != nil {
		return errors.Wrap(err, "failed to start the swipe gesture")
	}

	select {
	case <-time.After(delayToPreventGesture):
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "timeout while sleeping")
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

// getTasksInfo returns a list of all the active ARC tasks records.
func (ac *Activity) getTasksInfo(ctx context.Context) (tasks []taskInfo, err error) {
	cmd := ac.a.Command(ctx, "dumpsys", "activity", "activities")
	out, err := cmd.Output()
	if err != nil {
		return []taskInfo{}, errors.Wrap(err, "could not get 'dumpsys activity activities' output")
	}
	output := string(out)
	// Looking for:
	// Stack #2: type=standard mode=freeform
	// isSleeping=false
	// mBounds=Rect(0, 0 - 0, 0)
	//   Task id #5
	//   mBounds=Rect(1139, 359 - 1860, 1640)
	//   mMinWidth=-1
	//   mMinHeight=-1
	//   mLastNonFullscreenBounds=Rect(1139, 359 - 1860, 1640)
	//   * TaskRecordArc{TaskRecordArc{TaskRecord{54ef88b #5 A=com.android.settings.root U=0 StackId=2 sz=1}, WindowState{freeform restore-bounds=Rect(1139, 359 - 1860, 1640)}} , WindowState{freeform restore-bounds=Rect(1139, 359 - 1860, 1640)}}
	// 	userId=0 effectiveUid=1000 mCallingUid=1000 mUserSetupComplete=true mCallingPackage=org.chromium.arc.applauncher
	// 	affinity=com.android.settings.root
	// 	intent={act=android.intent.action.MAIN cat=[android.intent.category.LAUNCHER] flg=0x10210000 cmp=com.android.settings/.Settings}
	// 	origActivity=com.android.settings/.Settings
	// 	realActivity=com.android.settings/.Settings
	// 	autoRemoveRecents=false isPersistable=true numFullscreen=1 activityType=1
	// 	rootWasReset=true mNeverRelinquishIdentity=true mReuseTask=false mLockTaskAuth=LOCK_TASK_AUTH_PINNABLE
	// 	Activities=[ActivityRecord{64b5e83 u0 com.android.settings/.Settings t5}]
	// 	askedCompatMode=false inRecents=true isAvailable=true
	// 	mRootProcess=ProcessRecord{8dc5d68 5809:com.android.settings/1000}
	// 	stackId=2
	// 	hasBeenVisible=true mResizeMode=RESIZE_MODE_RESIZEABLE_VIA_SDK_VERSION mSupportsPictureInPicture=false isResizeable=true lastActiveTime=1470240 (inactive for 4s)
	// 	Arc Window State:
	// 	mWindowMode=5 mRestoreBounds=Rect(1139, 359 - 1860, 1640) taskWindowState=0
	regStr := `(?m)` + // Enable multiline.
		`^\s+Task id #(\d+)` + // Grab task id (group 1).
		`\s+mBounds=Rect\((\d+),\s*(\d+)\s*-\s*(\d+),\s*(\d+)\)` + // Grab bounds (groups 2-5).
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`.*TaskRecord{.*StackId=(\d+)\s+sz=(\d*)}.*$` + // Grab stack Id (group 6) and stack size (group 7).
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`\s+realActivity=(.*)\/(.*)` + // Grab package name (group 8) and activity name (group 9).
		`(?:\n.*?)*` + // Non-greedy skip lines.
		`\s+mWindowMode=\d+.*taskWindowState=(\d+)` // Grab window state (group 10).
	re := regexp.MustCompile(regStr)
	matches := re.FindAllStringSubmatch(string(output), -1)
	// At least it must match one activity. Home and/or Dummy activities must be present.
	if len(matches) == 0 {
		testing.ContextLog(ctx, "Using regexp: ", regStr)
		testing.ContextLog(ctx, "Output for regexp: ", string(output))
		return []taskInfo{}, errors.New("could not match any activity; regexp outdated perhaps?")
	}
	for _, groups := range matches {
		var t taskInfo
		var windowState int
		t.bounds, err = parseBounds(groups[2:6])
		if err != nil {
			return []taskInfo{}, err
		}

		for _, dst := range []struct {
			v     *int
			group int
		}{
			{&t.id, 1},
			{&t.stackID, 6},
			{&t.stackSize, 7},
			{&windowState, 10},
		} {
			*dst.v, err = strconv.Atoi(groups[dst.group])
			if err != nil {
				return []taskInfo{}, errors.Wrapf(err, "could not parse %q", groups[dst.group])
			}
		}
		t.pkgName = groups[8]
		t.activityName = groups[9]
		t.windowState = WindowState(windowState)
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// getTaskInfo returns the task record associated for the current activity.
func (ac *Activity) getTaskInfo(ctx context.Context) (task taskInfo, err error) {
	tasks, err := ac.getTasksInfo(ctx)
	if err != nil {
		return taskInfo{}, errors.Wrap(err, "could not get task info")
	}
	for _, task := range tasks {
		if task.pkgName == ac.pkgName && task.activityName == ac.activityName {
			return task, nil
		}
	}
	return taskInfo{}, errors.Errorf("could not find task info for %s/%s", ac.pkgName, ac.activityName)
}

// Helper functions.

// parseBounds returns a Rect by parsing a slice of 4 strings.
// Each string represents the left, top, right and bottom values, in that order.
func parseBounds(s []string) (bounds Rect, err error) {
	if len(s) != 4 {
		return Rect{}, errors.Errorf("expecting a slice of length 4, got %d", len(s))
	}
	for i, dst := range []*int{&bounds.Left, &bounds.Top, &bounds.Right, &bounds.Bottom} {
		*dst, err = strconv.Atoi(s[i])
		if err != nil {
			return Rect{}, errors.Wrapf(err, "could not parse %q", s[i])
		}
	}
	return bounds, nil
}
