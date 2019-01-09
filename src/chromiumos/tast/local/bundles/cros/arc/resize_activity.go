// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ResizeActivity,
		Desc:         "Verifies that resizing ARC++ applications work",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "android_p", "chrome_login"},
		Timeout:      4 * time.Minute,
	})
}

type activity struct {
	ctx          context.Context
	a            *arc.ARC
	pkgName      string
	activityName string
}

type rect struct {
	left, top, right, bottom int
}

func newActivity(ctx context.Context, a *arc.ARC, pkgName string, activityName string) (*activity, error) {
	return &activity{ctx, a, pkgName, activityName}, nil
}

func (ac *activity) start() error {
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

func (ac *activity) stop() error {
	// "adb shell am force-stop" has no output. So the error from Run() is returned.
	return ac.a.Command(ac.ctx, "am", "force-stop", ac.pkgName).Run()
}

func (ac *activity) bounds() (rect, error) {
	cmd := ac.a.Command(ac.ctx, "dumpsys", "window", "displays")
	output, err := cmd.Output()
	if err != nil {
		return rect{}, errors.Wrap(err, "failed to launch dumpsys")
	}

	// Line that we are interested in parsing:
	//  mBounds=[0,0][2400,1600]
	//  mdr=false
	//  appTokens=[AppWindowToken{85a61b token=Token{42ff82a ActivityRecord{e8d1d15 u0 org.chromium.arc.home/.HomeActivity t2}}}]
	// We are interested in "mBounds="
	regStr := `\s*mBounds=\[([0-9]*),([0-9]*)\]\[([0-9]*),([0-9]*)\]\n\s*mdr=.*\n\s*appTokens=.*` + ac.pkgName + "/" + ac.activityName + `.*\n`
	re := regexp.MustCompile(regStr)
	groups := re.FindStringSubmatch(string(output))
	if len(groups) != 5 {
		return rect{}, errors.New("failed to parse dumpsys output; activity not running perhaps?")
	}
	// left, top, right, bottom
	var bounds [4]int
	for i := 0; i < 4; i++ {
		bounds[i], err = strconv.Atoi(groups[i+1])
		if err != nil {
			return rect{}, errors.Wrap(err, "could not parse bounds")
		}
	}
	return rect{bounds[0], bounds[1], bounds[2], bounds[3]}, nil
}

func (ac *activity) resize(toX, toY int) error {
	bounds, _ := ac.bounds()

	tsw, err := input.Touchscreen(ac.ctx)
	if err != nil {
		return errors.Wrap(err, "could not open touchscreen device")
	}
	defer tsw.Close()

	// Touchscreen bounds: The size of the touchscreen might not be the same
	// as the display size. In fact, might be even up to 4x bigger.
	touchWidth := tsw.Width()
	touchHeight := tsw.Height()

	// Display bounds
	displayWidth := 2400
	displayHeight := 1600

	pixelToTouchFactorX := float64(touchWidth) / float64(displayWidth)
	pixelToTouchFactorY := float64(touchHeight) / float64(displayHeight)

	fromX := float64(bounds.right) * pixelToTouchFactorX
	fromY := float64(bounds.bottom) * pixelToTouchFactorY

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "could not get a new TouchEventWriter")
	}
	defer stw.Close()

	numTouches := 15
	offsetX := 0.0
	offsetY := 0.0
	deltaX := (float64(toX) - fromX) * pixelToTouchFactorX / float64(numTouches)
	deltaY := (float64(toY) - fromY) * pixelToTouchFactorY / float64(numTouches)
	for i := 0; i < numTouches; i++ {
		// Values must be in "touchscreen coordinates", not pixel coordinates.
		stw.Move(input.TouchCoord(fromX+offsetX), input.TouchCoord(fromY+offsetY))
		offsetX += deltaX
		offsetY += deltaY
		sleep(ac.ctx, 30*time.Millisecond)
	}
	return nil
}

func (ac *activity) getTaskID() (string, error) {
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

func (ac *activity) setWindowState(state string) (string, error) {
	taskID, err := ac.getTaskID()
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
		result = fmt.Sprintf("Input wrong window state value %q [normal|maximized|fullscreen|minimized]", state)
		err = errors.New(result)
	}
	if len(result) == 0 {
		result = string(output)
	}
	// s.Logf("Setting window state: %v", result)
	return result, err
}

func ResizeActivity(ctx context.Context, s *testing.State) {
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

	ac, _ := newActivity(ctx, a, "com.android.settings", ".Settings")
	if err := ac.start(); err != nil {
		s.Fatal("Could not launch settings: ", err)
	}

	if result, err := ac.setWindowState("normal"); err != nil {
		s.Fatal("Failed to set window state: ", err)
	} else {
		s.Log(result)
	}

	rect, err := ac.bounds()
	if err != nil {
		s.Fatal("Error getting bounds: ", err)
	}
	s.Logf("Bounds = %v", rect)

	ac.resize(rect.right+50, rect.bottom+50)

	screenshotName := "screenshot.png"
	path := filepath.Join(s.OutDir(), screenshotName)
	s.Logf("Screenshot should be placed: %s\n", path)

	if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
		s.Fatal("Error taking screenshot: ", err)
	}

	s.Log("Sleeping for 10 seconds...")
	sleep(ctx, 10*time.Second)
}

func sleep(ctx context.Context, t time.Duration) error {
	select {
	case <-time.After(t):
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
