// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// Activity holds resources associated with an ARC activity.
type Activity struct {
	a            *ARC // weak ref.
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

const (
	// WindowStateNormal is the "unmaximized / unminimized" window state.
	WindowStateNormal WindowState = 0
	// WindowStateMaximized  is the maximized window state.
	WindowStateMaximized = 1
	// WindowStateFullscreen is the fullscreen window state.
	WindowStateFullscreen = 2
	// WindowStateMinimized is the minimized window state.
	WindowStateMinimized = 3
)

const (
	// borderOffset represents the pixels outside the border should be used to grab the window.
	// This is an arbitrary value, and allows triggering the resize events.
	// If it is 0, then the touch will land inside the activity's bounds and resize
	// won't be triggered. If the value is too big, the touch event will land too far away
	// form the activity and won't trigger the resize event.
	borderOffset = 3
	// touchFrequency is the minimum time that should elapse between touches.
	touchFrequency = 5 * time.Millisecond
)

// IntegerPoint represents an point.
type IntegerPoint struct {
	// X and Y are the point coordinates.
	X, Y int
}

type floatPoint struct {
	x, y float64
}

// Rect represents a rectangle.
type Rect struct {
	Left, Top, Right, Bottom int
}

// NewActivity returns a new Activity instance.
// |a| argument is NOT owned, and the caller is responsible for closing it.
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
		testing.ContextLogf(ctx, "Failed to start activity: %v", groups[1])
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
func (ac *Activity) WindowBounds(ctx context.Context) (Rect, error) {
	cmd := ac.a.Command(ctx, "dumpsys", "window", "displays")
	output, err := cmd.Output()
	if err != nil {
		return Rect{}, errors.Wrap(err, "failed to launch dumpsys")
	}

	// Looking for:
	//  mBounds=[0,0][2400,1600]
	//  mdr=false
	//  appTokens=[AppWindowToken{85a61b token=Token{42ff82a activityRecord{e8d1d15 u0 org.chromium.arc.home/.HomeActivity t2}}}]
	// We are interested in "mBounds="
	regStr := `(?m)` + // Enable multiline.
		`^\s*mBounds=\[([0-9]*),([0-9]*)\]\[([0-9]*),([0-9]*)\]$` + // Each mBounds's value in a group.
		`\s*mdr=.*$` + // skip this line
		`\s*appTokens=.*` + ac.pkgName + "/" + ac.activityName // Make sure it matches the correct activity.
	re := regexp.MustCompile(regStr)
	groups := re.FindStringSubmatch(string(output))
	if len(groups) != 5 {
		return Rect{}, errors.New("failed to parse dumpsys output; activity not running perhaps?")
	}
	// bounds = left, top, right, bottom
	var bounds [4]int
	for i := 0; i < 4; i++ {
		bounds[i], err = strconv.Atoi(groups[i+1])
		if err != nil {
			return Rect{}, errors.Wrap(err, "could not parse bounds")
		}
	}

	// bounds[1] == top coordinate.
	// Fullscreen apps start at 0 and already include the caption height.
	// If it is not in fullscreen, caption is not included in the dumpsys
	// and should be added.
	if bounds[1] != 0 {
		captionHeight, err := ac.disp.CaptionHeight(ctx)
		if err != nil {
			return Rect{}, errors.Wrap(err, "failed to get caption height")
		}
		bounds[1] -= captionHeight
	}
	return Rect{bounds[0], bounds[1], bounds[2], bounds[3]}, nil
}

// Close closes the resources associated with the Activity instance.
// Calling Close() does not stop the activity.
func (ac *Activity) Close() {
	ac.disp.Close()
	if ac.tew != nil {
		ac.tew.Close()
	}
}

// initTouchscreenLazily lazily initializes the touchscreen. Touchscreen initialization
// is not needed, unless MoveWindow() or ResizeWindow() is called.
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

// MoveWindow moves the activity's window to a new location.
// |to| represents the destination of the movement in ChromeOS display coordinates.
// |t| represents how long the movement should last.
// MoveWindow performs the movement by injecting Touch events in the kernel. If the
// device does not have a touchscreen, MoveWindow() will fail.
func (ac *Activity) MoveWindow(ctx context.Context, to IntegerPoint, t time.Duration) error {

	winState, err := ac.windowState(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get window state")
	}

	if winState != WindowStateNormal {
		return errors.New("could not move window if it is not in freeform mode")
	}

	bounds, err := ac.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get activity bounds")
	}

	captionHeight, err := ac.disp.CaptionHeight(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get caption height")
	}

	halfWidth := (bounds.Right - bounds.Left) / 2
	// fromX/fromY represent the middle point of the caption.
	fromX := bounds.Left + halfWidth
	fromY := bounds.Top + captionHeight/2
	to.X += halfWidth
	to.Y += captionHeight / 2
	numTouches := int(t/touchFrequency) + 1
	return ac.generateTouches(ctx, IntegerPoint{fromX, fromY}, to, numTouches)
}

// ResizeWindow resizes the activity's window. The resize could be done from any of the 8 possible
// borders: top, bottom, left, right; plus the four corners.
// |to| represents the destination for the resize in ChromeOS display coordinates.
// |t| represents how long the resize should last.
// ResizeWindow performs the resizing by injecting Touch events in the kernel. If the
// device does not have a touchscreen, ResizeWindow() will fail.
func (ac *Activity) ResizeWindow(ctx context.Context, border BorderType, to IntegerPoint, t time.Duration) error {
	winState, err := ac.windowState(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get window state")
	}

	if winState != WindowStateNormal {
		return errors.New("could not resize window if it is not in freeform mode")
	}

	bounds, err := ac.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get activity bounds")
	}

	from := coordsForBorder(border, bounds)
	numTouches := int(t/touchFrequency) + 1
	return ac.generateTouches(ctx, from, to, numTouches)
}

// SetWindowState sets the window state to any of these states:
// WindowStateNormal, WindowStateMaximized, WindowStateFullscreen, WindowStateMinimized.
func (ac *Activity) SetWindowState(ctx context.Context, state WindowState) error {
	taskID, err := ac.taskID(ctx)
	if err != nil {
		errors.Wrap(err, "could not get taskID")
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
		return errors.Errorf("input wrong window state value %d", state)
	}

	if err = ac.a.Command(ctx, "am", "task", "set-winstate", strconv.Itoa(taskID), stateToRun).Run(); err != nil {
		return errors.Wrap(err, "could not execute 'am task set-winstate'")
	}
	return nil
}

// windowState returns the window mode state: WindowStateNormal, WindowStateMaximized,
// WindowStateFullscreen or WindowStateMinimized.
func (ac *Activity) windowState(ctx context.Context) (WindowState, error) {
	taskID, err := ac.taskID(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "could not get taskID")
	}

	cmd := ac.a.Command(ctx, "am", "task", "get-winstate", strconv.Itoa(taskID))
	out, err := cmd.Output()
	if err != nil {
		return 0, errors.Wrap(err, "could not get 'am task get-winstate' output")
	}

	state := strings.TrimSpace(string(out))
	switch state {
	case "maximized":
		return WindowStateMaximized, nil
	case "minimized":
		return WindowStateMinimized, nil
	case "normal":
		return WindowStateNormal, nil
	case "fullscreen":
		return WindowStateFullscreen, nil
	}
	testing.ContextLogf(ctx, "Unsupported output from 'am task get-winstate': %s", state)
	return 0, errors.New("could not get valid winstate")
}

// taskID returns the activity's task record ID. TaskRecord is an internal Android's structure
// that represents the collection of all activies launched by the task.
func (ac *Activity) taskID(ctx context.Context) (int, error) {
	cmd := ac.a.Command(ctx, "dumpsys", "activity", "activities")
	out, err := cmd.Output()
	if err != nil {
		return -1, errors.Wrap(err, "could not get 'dumpsys activity activities' output")
	}
	output := string(out)
	// Looking for:
	// frontOfTask=true task=TaskRecordArc{TaskRecord{965abeb #2 A=org.chromium.arc.home U=0 StackId=0 sz=1}, WindowState{fullscreen force-maximized restore-bounds=null}}
	regStr := fmt.Sprintf(`TaskRecord{.*#([0-9]+) [A-Z]=%s`, ac.pkgName)
	re := regexp.MustCompile(regStr)
	groups := re.FindStringSubmatch(output)
	if len(groups) != 2 {
		return -1, errors.New("failed to parse taskID")
	}
	taskID, err := strconv.Atoi(groups[1])
	if err != nil {
		return -1, errors.Wrap(err, "failed to convert taskID to integer")
	}
	return taskID, nil
}

// generateTouches injects touch events in a straight line. The line is defined
// by |from| and |to|. |numTouches| represents the number of touches that will be injected.
// If |numTouches| is less than 2, then 2 points will be used.
func (ac *Activity) generateTouches(ctx context.Context, from, to IntegerPoint, numTouches int) error {
	// A minimum of two points are required to form a line
	if numTouches < 2 {
		numTouches = 2
	}

	if err := ac.initTouchscreenLazily(ctx); err != nil {
		return errors.Wrap(err, "could not initialize touchscreen device")
	}

	stw, err := ac.tew.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "could not get a new TouchEventWriter")
	}
	defer stw.Close()

	// Using "non-rotated" display bounds for calculate the scale factor since
	// touchscreen bounds are also "non-rotated".
	dispSize, err := ac.disp.stableSize(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get stable bounds for display")
	}

	// Get displayscreen-to-touchscreen factor. Touchscreen might have different
	// resolution than the displayscreen.
	pixelToTouchFactorX := float64(ac.tew.Width()) / float64(dispSize.W)
	pixelToTouchFactorY := float64(ac.tew.Height()) / float64(dispSize.H)

	// numTouches-1 since we guarantee that one point will be at the beginning of
	// the line, and another one at the end.
	deltaX := float64(to.X-from.X) / float64(numTouches-1)
	deltaY := float64(to.Y-from.Y) / float64(numTouches-1)

	for i := 0; i < numTouches; i++ {
		x := float64(from.X) + deltaX*float64(i)
		y := float64(from.Y) + deltaY*float64(i)
		stw.Move(input.TouchCoord(x*pixelToTouchFactorX), input.TouchCoord(y*pixelToTouchFactorY))

		// Small delay.
		select {
		case <-time.After(touchFrequency):
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "timeout while doing sleep")
		}
	}
	stw.End()
	return nil
}

// Helper functions

// coordsForBorder returns the coordinates that should be used
// to grab the activity for the given border.
func coordsForBorder(border BorderType, bounds Rect) IntegerPoint {
	// Default value: center of application
	src := IntegerPoint{
		bounds.Left + (bounds.Right-bounds.Left)/2,
		bounds.Top + (bounds.Bottom-bounds.Top)/2,
	}

	// Top & Bottom are exclusive
	if border&BorderTop != 0 {
		src.Y = bounds.Top - borderOffset
	} else if border&BorderBottom != 0 {
		src.Y = bounds.Bottom + borderOffset
	}

	// Left & Right are exclusive
	if border&BorderLeft != 0 {
		src.X = bounds.Left - borderOffset
	} else if border&BorderRight != 0 {
		src.X = bounds.Right + borderOffset
	}
	return src
}
