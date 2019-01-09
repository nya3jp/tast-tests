// Copyright 2018 The Chromium OS Authors. All rights reserved.
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

	// cached values
	displayWidth  int
	displayHeight int
	tsw           *input.TouchscreenEventWriter
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
	captionHeight  = 64
	touchFrequency = 5 * time.Millisecond
)

type vertex struct {
	x, y int
}

// Rect XXX
type Rect struct {
	Left, Top, Right, Bottom int
}

// NewActivity XXX
func NewActivity(ctx context.Context, a *ARC, pkgName string, activityName string) (*Activity, error) {
	return &Activity{ctx, a, pkgName, activityName, 0, 0, nil}, nil
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

// Stop XXX
func (ac *Activity) Stop() error {
	// "adb shell am force-stop" has no output. So the error from Run() is returned.
	return ac.a.Command(ac.ctx, "am", "force-stop", ac.pkgName).Run()
}

// Bounds XXX
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
	regStr := `\s*mBounds=\[([0-9]*),([0-9]*)\]\[([0-9]*),([0-9]*)\]\n\s*mdr=.*\n\s*appTokens=.*` + ac.pkgName + "/" + ac.activityName + `.*\n`
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
	return Rect{bounds[0], bounds[1], bounds[2], bounds[3]}, nil
}

// Close XXX
func (ac *Activity) Close() {
	if ac.tsw != nil {
		ac.tsw.Close()
	}
}

// Resize XXX
func (ac *Activity) Resize(border uint, toX, toY int, t time.Duration) error {
	if ac.tsw == nil {
		var err error
		ac.tsw, err = input.Touchscreen(ac.ctx)
		if err != nil {
			return errors.Wrap(err, "could not open touchscreen device")
		}
	}

	// Lazy-get for display bounds. We do not expect that the display bounds
	// are going to change so it is safe to store them in activity.
	if ac.displayWidth == 0 && ac.displayHeight == 0 {
		if err := ac.updateDisplayBounds(); err != nil {
			return errors.Wrap(err, "Could not get display bounds")
		}
	}

	bounds, err := ac.Bounds()
	if err != nil {
		return errors.Wrap(err, "could not get activity bounds")
	}

	// Get displayscreen-to-touchscreen factor. Touchscreen might have different
	// resolution than the displayscreen.
	pixelToTouchFactorX := float64(ac.tsw.Width()) / float64(ac.displayWidth)
	pixelToTouchFactorY := float64(ac.tsw.Height()) / float64(ac.displayHeight)

	from, to := coordsForBorder(border, bounds, vertex{toX, toY})
	fromX := float64(from.x)
	fromY := float64(from.y)

	stw, err := ac.tsw.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "could not get a new TouchEventWriter")
	}
	defer stw.Close()

	numTouches := int((t / touchFrequency) + 1)
	testing.ContextLogf(ac.ctx, "---> Resizing from (%d,%d) to (%d,%d) with numTouches=%d\n",
		from.x, from.y, to.x, to.y, numTouches)

	offsetX := 0.0
	offsetY := 0.0
	deltaX := (float64(to.x) - fromX) / float64(numTouches)
	deltaY := (float64(to.y) - fromY) / float64(numTouches)
	for i := 0; i < numTouches; i++ {
		// Values must be in "touchscreen coordinates", not pixel coordinates.
		stw.Move(
			input.TouchCoord((fromX+offsetX)*pixelToTouchFactorX),
			input.TouchCoord((fromY+offsetY)*pixelToTouchFactorY))
		offsetX += deltaX
		offsetY += deltaY
		sleep(ac.ctx, touchFrequency)
	}
	// Send extra touch at destination in case it was missed with deltaX/deltaY's rounding errors.
	stw.Move(input.TouchCoord(float64(to.x)*pixelToTouchFactorX),
		input.TouchCoord(float64(to.y)*pixelToTouchFactorY))
	sleep(ac.ctx, touchFrequency)
	stw.End()
	return nil
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

func (ac *Activity) updateDisplayBounds() error {
	cmd := ac.a.Command(ac.ctx, "dumpsys", "window", "displays")
	output, err := cmd.Output()
	if err != nil {
		return errors.Wrap(err, "failed to launch dumpsys")
	}

	// Line that we are interested in parsing:
	// Display: mDisplayId=0
	//   init=2400x1600 240dpi cur=2400x1600 app=2400x1424 rng=1600x1424-2400x2224
	// We are interested in "init="
	regStr := `\s*mDisplayId=0\n\s*init=([0-9]+)x([0-9]+)`
	re := regexp.MustCompile(regStr)
	groups := re.FindStringSubmatch(string(output))
	if len(groups) != 3 {
		return errors.New("failed to parse dumpsys output")
	}

	width, err := strconv.Atoi(groups[1])
	if err != nil {
		return errors.Wrap(err, "could not parse bounds")
	}
	height, err := strconv.Atoi(groups[2])
	if err != nil {
		return errors.Wrap(err, "could not parse bounds")
	}

	ac.displayWidth = width
	ac.displayHeight = height
	return nil
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

func coordsForBorder(border uint, bounds Rect, dst vertex) (vertex, vertex) {
	// Default value: center of application
	src := vertex{
		bounds.Left + (bounds.Right-bounds.Left)/2,
		bounds.Top + (bounds.Bottom-bounds.Top)/2}

	// Top & Bottom are exclusive
	if border&BorderTop != 0 {
		src.y = bounds.Top - borderOffset - captionHeight
		dst.y -= captionHeight
	} else if border&BorderBottom != 0 {
		src.y = bounds.Bottom + borderOffset
	}

	// Left & Right are exclusive
	if border&BorderLeft != 0 {
		src.x = bounds.Left - borderOffset
	} else if border&BorderRight != 0 {
		src.x = bounds.Right + borderOffset
	}
	return src, dst
}

func sleep(ctx context.Context, t time.Duration) error {
	select {
	case <-time.After(t):
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
