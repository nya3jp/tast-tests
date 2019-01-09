// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// Activity holds resources associated with an ARC activity.
type Activity struct {
	ctx          context.Context
	a            *ARC
	pkgName      string
	activityName string
	disp         *Display

	// cached values
	tsw *input.TouchscreenEventWriter
}

const (
	// BorderTop is the top border.
	BorderTop uint = 1 << 0
	// BorderBottom is the bottom border.
	BorderBottom uint = 1 << 1
	// BorderLeft is the left border.
	BorderLeft uint = 1 << 2
	// BorderRight is the right border.
	BorderRight uint = 1 << 3
	// BorderTopLeft is the top-left corner.
	BorderTopLeft = (BorderTop | BorderLeft)
	// BorderTopRight is the top-right corner.
	BorderTopRight = (BorderTop | BorderRight)
	// BorderBottomLeft is the bottom-left corner.
	BorderBottomLeft = (BorderBottom | BorderLeft)
	// BorderBottomRight is the bottom-right corner.
	BorderBottomRight = (BorderBottom | BorderRight)

	// WindowNormal is the "unmaximized / unminimized" window state.
	WindowNormal = 0
	// WindowMaximized  is the maximized window state.
	WindowMaximized = 1
	// WindowFullscreen is the fullscreen window state.
	WindowFullscreen = 2
	// WindowMinimized is the minimized window state.
	WindowMinimized = 3
)

const (
	// borderOffset represents the pixels outside the border should be used to grab the window.
	borderOffset = 3
	// touchFrequency is the minimum time that should elapse between touches.
	touchFrequency = 5 * time.Millisecond
)

type vec2i struct {
	x, y int
}

type vec2f struct {
	x, y float64
}

// Rect represents a rectangle
type Rect struct {
	Left, Top, Right, Bottom int
}

// NewActivity returns a new Activity instance.
// Returned Activity instance must be closed when the test is finished.
func NewActivity(ctx context.Context, a *ARC, pkgName string, activityName string) (*Activity, error) {
	disp, err := NewDisplay(ctx, a, DefaultDisplayID)
	if err != nil {
		return nil, errors.Wrap(err, "could not create a new Display")
	}
	return &Activity{ctx: ctx, a: a, pkgName: pkgName, activityName: activityName, disp: disp}, nil
}

// Start starts the activity by invoking "am start".
func (ac *Activity) Start() error {
	cmd := ac.a.Command(ac.ctx, "am", "start", "-W", ac.pkgName+"/"+ac.activityName)
	output, err := cmd.Output()
	if err != nil {
		return errors.Wrap(err, "failed to start activity")
	}
	// "adb shell" doesn't distinguish between a failed/successful run. For that we have to parse the output.
	re := regexp.MustCompile("(?m)^Error:")
	if re.MatchString(string(output)) {
		testing.ContextLog(ac.ctx, "Failed to start activity: ", string(output))
		return errors.New("failed to start activity")
	}

	return nil
}

// Stop stops the activity by invoking "am force-stop" with the package name.
// If there are multiple activities that belong to the same package name, all of
// them will be stopped.
func (ac *Activity) Stop() error {
	// "adb shell am force-stop" has no output. So the error from Run() is returned.
	return ac.a.Command(ac.ctx, "am", "force-stop", ac.pkgName).Run()
}

// Bounds returns the activity bounds in pixels, including the caption height.
func (ac *Activity) Bounds() (Rect, error) {
	cmd := ac.a.Command(ac.ctx, "dumpsys", "window", "displays")
	output, err := cmd.Output()
	if err != nil {
		return Rect{}, errors.Wrap(err, "failed to launch dumpsys")
	}

	// Line that we are interested in parsing:
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
	// Fullscreen apps start at 0 and alread include the caption height.
	// If it is not in fullscreen, caption is not included in the dumpsys
	// and should be added.
	if bounds[1] != 0 {
		captionHeight, err := ac.disp.CaptionHeight()
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
	// ac.Stop()
	ac.disp.Close()
	if ac.tsw != nil {
		ac.tsw.Close()
	}
}

// lazyTouchscreenInit lazily initializes the touchscreen. Touchscreen initialization
// is not needed, unless Move() or Resize() is called.
func (ac *Activity) lazyTouchscreenInit() error {
	if ac.tsw == nil {
		var err error
		ac.tsw, err = input.Touchscreen(ac.ctx)
		if err != nil {
			return errors.Wrap(err, "could not open touchscreen device")
		}
	}
	return nil
}

// Move moves the activity to a new location. toX and toY should be in pixel coordinates.
// Move performs the movement by injecting Touch events in the kernel. If the
// device does not have a touchscreen, Move() will fail.
func (ac *Activity) Move(toX, toY int, t time.Duration) error {
	bounds, err := ac.Bounds()
	if err != nil {
		return errors.Wrap(err, "could not get activity bounds")
	}

	captionHeight, err := ac.disp.CaptionHeight()
	if err != nil {
		return errors.Wrap(err, "could not get caption height")
	}

	halfWidth := (bounds.Right - bounds.Left) / 2
	fromX := bounds.Left + halfWidth
	fromY := bounds.Top + captionHeight/2
	toX += halfWidth
	toY += captionHeight / 2
	// testing.ContextLogf(ac.ctx, "--> Moving from(%d,%d) to (%d,%d)", fromX, fromY, toX, toY)
	numTouches := int(t/touchFrequency) + 1
	return ac.generateTouches(lineIter(vec2i{fromX, fromY}, vec2i{toX, toY}, numTouches))
}

// Resize resizes the activity. The resize could be done from any of the 8 possible
// borders: top, bottom, left, right; plus the four corners. toX and toY represents
// the destination for the resize in pixel coordinates.
// Resize performs the resizing by injecting Touch events in the kernel. If the
// device does not have a touchscreen, Resize() will fail.
func (ac *Activity) Resize(border uint, toX, toY int, t time.Duration) error {
	bounds, err := ac.Bounds()
	if err != nil {
		return errors.Wrap(err, "could not get activity bounds")
	}

	from := coordsForBorder(border, bounds)
	// testing.ContextLogf(ac.ctx, "--> Resizing from(%d,%d) to (%d,%d)", from.x, from.y, toX, toY)
	numTouches := int(t/touchFrequency) + 1
	return ac.generateTouches(lineIter(from, vec2i{toX, toY}, numTouches))
}

// SetWindowState sets the window state to any of these states:
// WindowNormal, WindowMaximized, WindowFullscreen, WindowMinimized.
func (ac *Activity) SetWindowState(state int) error {
	taskID, err := ac.taskID()
	if err != nil {
		errors.Wrap(err, "could not get taskID")
	}
	stateToRun := ""
	switch state {
	case WindowNormal:
		stateToRun = "0"
	case WindowMaximized:
		stateToRun = "1"
	case WindowFullscreen:
		stateToRun = "2"
	case WindowMinimized:
		stateToRun = "3"
	default:
		return errors.Errorf("Input wrong window state value %d", state)
	}

	if err = ac.a.Command(ac.ctx, "am", "task", "set-winstate", strconv.Itoa(taskID), stateToRun).Run(); err != nil {
		return errors.Wrap(err, "could not obtain 'am task set-winstate' output")
	}
	return nil
}

func (ac *Activity) taskID() (int, error) {
	cmd := ac.a.Command(ac.ctx, "dumpsys", "activity", "activities")
	out, err := cmd.Output()
	if err != nil {
		return -1, err
	}
	output := string(out)
	regStr := fmt.Sprintf("#([0-9]*) [A-Z]=%v", ac.pkgName)
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

func (ac *Activity) generateTouches(iter <-chan vec2f) error {
	if err := ac.lazyTouchscreenInit(); err != nil {
		return errors.Wrap(err, "could not initialize touchscreen device")
	}

	stw, err := ac.tsw.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "could not get a new TouchEventWriter")
	}
	defer stw.Close()

	// Using "non-rotated" display bounds for calculate the scale factor since
	// touchscreen bounds are also "non-rotated".
	dispWidth, dispHeight, err := ac.disp.stableBounds()
	if err != nil {
		return errors.Wrap(err, "could not get stable bounds for display")
	}

	// Get displayscreen-to-touchscreen factor. Touchscreen might have different
	// resolution than the displayscreen.
	pixelToTouchFactorX := float64(ac.tsw.Width()) / float64(dispWidth)
	pixelToTouchFactorY := float64(ac.tsw.Height()) / float64(dispHeight)

	for val := range iter {
		// testing.ContextLogf(ac.ctx, "--> Touches at: %+v", val)
		stw.Move(
			input.TouchCoord(val.x*pixelToTouchFactorX),
			input.TouchCoord(val.y*pixelToTouchFactorY))

		sleep(ac.ctx, touchFrequency)
	}
	stw.End()
	return nil
}

// Helper functions

// coordsForBorder returns the coordinates that should be used
// to grab the activity for the given border.
func coordsForBorder(border uint, bounds Rect) vec2i {
	// Default value: center of application
	src := vec2i{
		bounds.Left + (bounds.Right-bounds.Left)/2,
		bounds.Top + (bounds.Bottom-bounds.Top)/2}

	// Top & Bottom are exclusive
	if border&BorderTop != 0 {
		src.y = bounds.Top - borderOffset
	} else if border&BorderBottom != 0 {
		src.y = bounds.Bottom + borderOffset
	}

	// Left & Right are exclusive
	if border&BorderLeft != 0 {
		src.x = bounds.Left - borderOffset
	} else if border&BorderRight != 0 {
		src.x = bounds.Right + borderOffset
	}
	return src
}

func sleep(ctx context.Context, t time.Duration) error {
	select {
	case <-time.After(t):
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// lineIter returns an iterator to generate a line. numPoints is the number of
// points that should be generated.
func lineIter(from, to vec2i, numPoints int) <-chan vec2f {
	ch := make(chan vec2f)

	deltaX := float64(to.x-from.x) / float64(numPoints)
	deltaY := float64(to.y-from.y) / float64(numPoints)
	offsetX := 0.0
	offsetY := 0.0

	go func() {
		for i := 0; i <= numPoints; i++ {
			ch <- vec2f{float64(from.x) + offsetX, float64(from.y) + offsetY}
			offsetX += deltaX
			offsetY += deltaY
		}
		// One final extra touch in the destination in case it was missed due to rounding errors.
		ch <- vec2f{float64(to.x), float64(to.y)}
		close(ch)
	}()
	return ch
}
