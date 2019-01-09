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

type activity struct {
	ctx           context.Context
	a             *ARC
	pkgName       string
	activityName  string
	displayWidth  int
	displayHeight int
}

type vertex struct {
	x, y int
}

// Rect XXX
type Rect struct {
	Left, Top, Right, Bottom int
}

// NewActivity XXX
func NewActivity(ctx context.Context, a *ARC, pkgName string, activityName string) (*activity, error) {
	return &activity{ctx, a, pkgName, activityName, 0, 0}, nil
}

func (ac *activity) Start() error {
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

func (ac *activity) Stop() error {
	// "adb shell am force-stop" has no output. So the error from Run() is returned.
	return ac.a.Command(ac.ctx, "am", "force-stop", ac.pkgName).Run()
}

func (ac *activity) Bounds() (Rect, error) {
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

const (
	// BorderTop XXX
	BorderTop = 1 << 0
	// BorderBottom XXX
	BorderBottom = 1 << 1
	// BorderLeft XXX
	BorderLeft = 1 << 2
	// BorderRight XXX
	BorderRight = 1 << 3
	// BorderTopLeft XXX
	BorderTopLeft = (BorderTop | BorderLeft)
	// BorderTopRight XXX
	BorderTopRight = (BorderTop | BorderRight)
	// BorderBottomLeft XXX
	BorderBottomLeft = (BorderBottom | BorderLeft)
	// BorderBottomRight XXX
	BorderBottomRight = (BorderBottom | BorderRight)

	numBorders = 4 // left, right, top, bottom
)

const (
	borderOffset  = 3
	captionHeight = 64
)

func originAndDestForBorder(border int, bounds Rect, toX, toY int) (vertex, vertex) {
	// Default value: center of application
	orig := vertex{
		bounds.Left + (bounds.Right-bounds.Left)/2,
		bounds.Top + (bounds.Bottom-bounds.Top)/2}
	offset := vertex{0, 0}

	for mask := 0; mask < numBorders; mask++ {
		maskedBorder := border & (1 << uint(mask))
		switch maskedBorder {
		case 0:
			// nothing
		case BorderTop:
			orig.y = bounds.Top
			offset.y = -captionHeight - borderOffset
		case BorderBottom:
			orig.y = bounds.Bottom
			offset.y = borderOffset
		case BorderLeft:
			orig.x = bounds.Left
			offset.x = -borderOffset
		case BorderRight:
			orig.x = bounds.Right
			offset.x = borderOffset
		default:
			panic(fmt.Sprintf("invalid border constant: 0x%x", maskedBorder))
		}
	}

	orig.x += offset.x
	orig.y += offset.y
	dest := vertex{toX + offset.x, toY + offset.y}
	return orig, dest
}

func (ac *activity) Resize(border, toX, toY int, t time.Duration) error {
	bounds, _ := ac.Bounds()

	tsw, err := input.Touchscreen(ac.ctx)
	if err != nil {
		return errors.Wrap(err, "could not open touchscreen device")
	}
	defer tsw.Close()

	if ac.displayWidth == 0 && ac.displayHeight == 0 {
		if err := ac.updateDisplayBounds(); err != nil {
			return errors.Wrap(err, "Could not get display bounds")
		}
	}

	// Touchscreen bounds: The size of the touchscreen might not be the same
	// as the display size. In fact, might be even up to 4x bigger.
	touchWidth := tsw.Width()
	touchHeight := tsw.Height()

	pixelToTouchFactorX := float64(touchWidth) / float64(ac.displayWidth)
	pixelToTouchFactorY := float64(touchHeight) / float64(ac.displayHeight)

	from, to := originAndDestForBorder(border, bounds, toX, toY)
	fromX := float64(from.x)
	fromY := float64(from.y)

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "could not get a new TouchEventWriter")
	}
	defer stw.Close()

	// Touch and hold the touch for a few ms just to grab the handle. seems to be more robust.
	stw.Move(input.TouchCoord(fromX*pixelToTouchFactorX), input.TouchCoord(fromY*pixelToTouchFactorY))
	sleep(ac.ctx, 200*time.Millisecond)

	const touchFrequency = 10 * time.Millisecond
	numTouches := int((t / touchFrequency) + 1)
	testing.ContextLogf(ac.ctx, "---> Resizing from (%d,%d) to (%d,%d) with numTouches=%d\n",
		from.x, from.y, to.x, to.x, numTouches)

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
	stw.End()
	return nil
}

func (ac *activity) taskID() (string, error) {
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

func (ac *activity) SetWindowState(state string) (string, error) {
	taskID, err := ac.taskID()
	if err != nil {
		return "", err
	}
	var output []byte
	var result string
	switch state {
	case "normal":
		output, err = ac.a.Command(ac.ctx, "am", "task", "set-winstate", taskID, "0").Output()
	case "maximized":
		output, err = ac.a.Command(ac.ctx, "am", "task", "set-winstate", taskID, "1").Output()
	case "fullscreen":
		output, err = ac.a.Command(ac.ctx, "am", "task", "set-winstate", taskID, "2").Output()
	case "minimized":
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

func (ac *activity) updateDisplayBounds() error {
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

func sleep(ctx context.Context, t time.Duration) error {
	select {
	case <-time.After(t):
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
