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

// Activity XXX
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
	// BorderTop XXX
	BorderTop uint = 1 << 0
	// BorderBottom XXX
	BorderBottom uint = 1 << 1
	// BorderLeft XXX
	BorderLeft uint = 1 << 2
	// BorderRight XXX
	BorderRight uint = 1 << 3
	// BorderTopLeft XXX
	BorderTopLeft = (BorderTop | BorderLeft)
	// BorderTopRight XXX
	BorderTopRight = (BorderTop | BorderRight)
	// BorderBottomLeft XXX
	BorderBottomLeft = (BorderBottom | BorderLeft)
	// BorderBottomRight XXX
	BorderBottomRight = (BorderBottom | BorderRight)
)

const (
	// WindowNormal XXX
	WindowNormal = 0
	// WindowMaximized  XXX
	WindowMaximized = 1
	// WindowFullscreen XXX
	WindowFullscreen = 2
	// WindowMinimized XXX
	WindowMinimized = 3
)

const (
	borderOffset   = 3
	touchFrequency = 5 * time.Millisecond
)

type vec2i struct {
	x, y int
}

type vec2f struct {
	x, y float64
}

// Rect XXX
type Rect struct {
	Left, Top, Right, Bottom int
}

// NewActivity XXX
func NewActivity(ctx context.Context, a *ARC, pkgName string, activityName string) (*Activity, error) {
	disp, err := NewDisplay(ctx, a, DefaultDisplayID)
	if err != nil {
		return nil, errors.Wrap(err, "could not create a new Display")
	}
	return &Activity{ctx: ctx, a: a, pkgName: pkgName, activityName: activityName, disp: disp}, nil
}

// Start XXX
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

// Display XXX
func (ac *Activity) Display() *Display {
	return ac.disp
}

// Stop XXX
func (ac *Activity) Stop() error {
	// "adb shell am force-stop" has no output. So the error from Run() is returned.
	return ac.a.Command(ac.ctx, "am", "force-stop", ac.pkgName).Run()
}

// Bounds returns the activity bounds, including the caption size.
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
		`\s*mdr=.*$` +
		`\s*appTokens=.*` + ac.pkgName + "/" + ac.activityName // Make sure it matches the correct activity.
	re := regexp.MustCompile(regStr)
	groups := re.FindStringSubmatch(string(output))
	if len(groups) != 5 {
		return Rect{}, errors.New("failed to parse dumpsys output; activity not running perhaps?")
	}
	// left, top, right, bottom
	var bounds [4]int
	for i := 0; i < 4; i++ {
		bounds[i], err = strconv.Atoi(groups[i+1])
		if err != nil {
			return Rect{}, errors.Wrap(err, "could not parse bounds")
		}
	}

	// bounds[1] = top
	// weird how it is being reported. fullscreen apps start at 0 and includes the caption height.
	// if it is not fullscreen, they don't include the caption
	if bounds[1] != 0 {
		captionHeight, err := ac.disp.CaptionHeight()
		if err != nil {
			return Rect{}, errors.Wrap(err, "failed to get caption height")
		}
		bounds[1] -= captionHeight
	}
	return Rect{bounds[0], bounds[1], bounds[2], bounds[3]}, nil
}

// Close XXX
func (ac *Activity) Close() {
	// ac.Stop()
	ac.disp.Close()
	if ac.tsw != nil {
		ac.tsw.Close()
	}
}

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

// Move XXX
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
	testing.ContextLogf(ac.ctx, "--> Moving from(%d,%d) to (%d,%d)", fromX, fromY, toX, toY)
	return ac.generateTouches(lineIter(vec2i{fromX, fromY}, vec2i{toX, toY}, t))
}

// Resize XXX
func (ac *Activity) Resize(border uint, toX, toY int, t time.Duration) error {
	bounds, err := ac.Bounds()
	if err != nil {
		return errors.Wrap(err, "could not get activity bounds")
	}

	from := coordsForBorder(border, bounds)
	testing.ContextLogf(ac.ctx, "--> Resizing from(%d,%d) to (%d,%d)", from.x, from.y, toX, toY)
	return ac.generateTouches(lineIter(from, vec2i{toX, toY}, t))
}

// SetWindowState XXX
func (ac *Activity) SetWindowState(state int) (string, error) {
	taskID, err := ac.taskID()
	if err != nil {
		return "", err
	}
	var output []byte
	var result string
	switch state {
	case WindowNormal:
		output, err = ac.a.Command(ac.ctx, "am", "task", "set-winstate", taskID, "0").Output()
	case WindowMaximized:
		output, err = ac.a.Command(ac.ctx, "am", "task", "set-winstate", taskID, "1").Output()
	case WindowFullscreen:
		output, err = ac.a.Command(ac.ctx, "am", "task", "set-winstate", taskID, "2").Output()
	case WindowMinimized:
		output, err = ac.a.Command(ac.ctx, "am", "task", "set-winstate", taskID, "3").Output()
	default:
		err = errors.Errorf("Input wrong window state value %q [normal|maximized|fullscreen|minimized]", state)
	}
	if len(result) == 0 {
		result = string(output)
	}
	// s.Logf("Setting window state: %v", result)
	return result, err
}

func (ac *Activity) taskID() (string, error) {
	cmd := ac.a.Command(ac.ctx, "dumpsys", "activity", "activities")
	out, err := cmd.Output()
	if err != nil {
		return "-1", err
	}
	output := string(out)
	regStr := fmt.Sprintf("#([0-9]*) [A-Z]=%v", ac.pkgName)
	re := regexp.MustCompile(regStr)
	taskIDs := re.FindStringSubmatch(output)
	if len(taskIDs) < 1 {
		return "-1", errors.New("can't find the taskID for this activity")
	}
	return taskIDs[1], nil
}

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

func lineIter(from, to vec2i, t time.Duration) <-chan vec2f {
	ch := make(chan vec2f)

	numTouches := int((t / touchFrequency) + 1)
	deltaX := float64(to.x-from.x) / float64(numTouches)
	deltaY := float64(to.y-from.y) / float64(numTouches)
	offsetX := 0.0
	offsetY := 0.0

	go func() {
		for i := 0; i <= numTouches; i++ {
			ch <- vec2f{float64(from.x) + offsetX, float64(from.y) + offsetY}
			offsetX += deltaX
			offsetY += deltaY
		}
		// One final extra touch in the destination in case it was missed due to rounding errors
		ch <- vec2f{float64(to.x), float64(to.y)}
		close(ch)
	}()
	return ch
}

func (ac *Activity) generateTouches(iter <-chan vec2f) error {
	if err := ac.lazyTouchscreenInit(); err != nil {
		return errors.Wrap(err, "could not initialize touchscreen device")
	}

	dispWidth, dispHeight, err := ac.disp.StableBounds()
	if err != nil {
		return errors.Wrap(err, "could not get stable bounds for display")
	}

	stw, err := ac.tsw.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "could not get a new TouchEventWriter")
	}
	defer stw.Close()

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
