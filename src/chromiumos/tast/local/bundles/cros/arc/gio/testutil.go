// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package gio contains functions and structs used for testing the gaming input overlay.
package gio

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/inputlatency"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	apk = "ArcInputOverlayTest.apk"
	pkg = "org.chromium.arc.testapp.inputoverlay"
	cls = "org.chromium.arc.testapp.inputoverlay.MainActivity"

	// inputOverlayFilename is the directory where input overlay files are stored.
	inputOverlayFilename = "google_gio"
	// cleanupOnErrorTime reserves time for cleanup in case of an error.
	cleanupOnErrorTime = time.Second * 30
	// errorMargin denotes the allowable +/- difference from the calculated x and
	// y coordinate.
	errorMargin = 3
	// WaitForActiveInputTime reserves time between and for hold-release controls
	// to ensure stability.
	WaitForActiveInputTime = time.Second
	// tapMode is the number of expected logcat lines from a tap event.
	tapMode mode = 2
	// moveMode is the number of expected logcat lines from a press-release event.
	moveMode mode = 3
)

var (
	// TopTap denotes the heuristics for the top tap input overlay mapping.
	TopTap = ButtonHeuristics{0.5, 0.5}
	// BotTap denotes the heuristic for the bottom tap input overlay mapping.
	BotTap = ButtonHeuristics{0.9, 0.9}
	// emptyHeuristic is used to pass in an empty ButtonHeuristics.
	emptyHeuristic = ButtonHeuristics{}
)

// ButtonHeuristics contains heuristics regarding the percentages on the ARC
// phone window where input mappings are located.
type ButtonHeuristics struct {
	xHeuristic float64
	yHeuristic float64
}

// TestParams stores data common to the tests run in this package.
type TestParams struct {
	TestConn          *chrome.TestConn
	Arc               *arc.ARC
	Device            *ui.Device
	Activity          *arc.Activity
	ActivityStartTime time.Time
	windowContentSize coords.Point
	lastTimestamp     string
}

// mode specifies the type of tap event via the number of expected logcat lines.
type mode int

// coolDownConfig returns the config to wait for the machine to cooldown for game performance tests.
// This overrides the default config timeout (5 minutes) and temperature threshold (46 C)
// settings to reduce test flakes on low-end devices.
func coolDownConfig() cpu.CoolDownConfig {
	cdConfig := cpu.DefaultCoolDownConfig(cpu.CoolDownPreserveUI)
	cdConfig.PollTimeout = 7 * time.Minute
	cdConfig.TemperatureThreshold = 61000
	return cdConfig
}

// PerformTestFunc allows callers to run their desired test after a provided activity has been launched.
type PerformTestFunc func(params TestParams) (err error)

// SetupTestApp installs the input overlay test application, starts the activity, and defers to the caller to perform a test.
func SetupTestApp(ctx context.Context, s *testing.State, testFunc PerformTestFunc) {
	// Shorten the test context so that even if the test times out
	// there will be time to clean up.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cleanupOnErrorTime)
	defer cancel()

	// Pull out the common values.
	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Could not open Test API connection: ", err)
	}

	// Make sure the device is clamshell mode.
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	if tabletModeEnabled {
		// Be nice and restore tablet mode to its original state on exit.
		defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)
		if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
			s.Fatal("Failed to set tablet mode disabled: ", err)
		}
		// TODO(b/187788935): Wait for "tablet mode animation is finished" in a reliable way.
		// If an activity is launched while the tablet mode animation is active, the activity
		// will be launched in un undefined state, making the test flaky.
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to wait until tablet-mode animation finished: ", err)
		}
	}

	// Install the gaming input overlay test application.
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing ArcInputOverlayTest: ", err)
	}

	// Wait for the CPU to idle before performing the test.
	if _, err := cpu.WaitUntilCoolDown(ctx, coolDownConfig()); err != nil {
		s.Fatal("Failed to wait until CPU is cooled down: ", err)
	}

	// Take screenshot on failure.
	defer func(ctx context.Context) {
		if s.HasError() {
			captureScreenshot(ctx, s, cr, "failed-launch-test.png")
		}
	}(cleanupCtx)

	// Clear input overlay files.
	userPath, err := cryptohome.UserPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's home directory path: ", err)
	}
	defer os.RemoveAll(filepath.Join(userPath, inputOverlayFilename))

	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create a new ArcInputOverlayTest activity: ", err)
	}
	defer act.Close()

	// Start timing and launch the activity.
	startTime := time.Now()

	if err := act.Start(ctx, tconn, arc.WithWindowingMode(arc.WindowingModeFreeform), arc.WithWaitForLaunch()); err != nil {
		s.Fatal("Failed to start ArcInputOverlayTest: ", err)
	}
	defer act.Stop(ctx, tconn)

	// Obtain window surface bounds.
	loc, err := act.SurfaceBounds(ctx)
	if err != nil {
		s.Error("Failed to obtain activity window bounds: ", err)
	}
	appWidth := loc.BottomRight().X - loc.TopLeft().X
	appHeight := loc.BottomRight().Y - loc.TopLeft().Y

	// Always take a screenshot of the final state for debugging purposes.
	// This is done with the cleanup context so the main flow is not interrupted.
	defer captureScreenshot(cleanupCtx, s, cr, "final-state.png")

	// Defer to the caller to determine when the game is launched.
	if err := testFunc(TestParams{
		TestConn:          tconn,
		Arc:               a,
		Device:            d,
		Activity:          act,
		ActivityStartTime: startTime,
		windowContentSize: coords.NewPoint(appWidth, appHeight),
		lastTimestamp:     "00:00:00.000",
	}); err != nil {
		s.Fatal("Failed to perform test: ", err)
	}
}

// CloseAndRelaunchActivity closes and reopens the test application again.
func CloseAndRelaunchActivity(ctx context.Context, params *TestParams) error {
	// Close current test application instance.
	params.Activity.Stop(ctx, params.TestConn)
	// Relaunch another test application instance.
	act, err := arc.NewActivity(params.Arc, pkg, cls)
	if err != nil {
		return errors.Wrap(err, "failed to create a new ArcInputOverlayTest activity")
	}
	if err := act.StartWithDefaultOptions(ctx, params.TestConn); err != nil {
		return errors.Wrap(err, "failed to restart ArcInputOverlayTest")
	}
	// Reassign "Activity" field in params.
	*params.Activity = *act

	return nil
}

// MoveOverlayButton returns a function that takes in the given character corresponding
// to a move keystroke and returns an error if tapping the keystroke did not result in
// the correct feedback.
func MoveOverlayButton(kb *input.KeyboardEventWriter, key string, params *TestParams) action.Action {
	return func(ctx context.Context) error {
		// Hold and release given key, which is associated to an overlay button.
		if err := uiauto.Combine("Tap overlay keys and ensure proper behavior",
			kb.AccelPressAction(key),
			// Add a sleep for one second to simulate user behavior on a key press.
			action.Sleep(WaitForActiveInputTime),
			kb.AccelReleaseAction(key),
		)(ctx); err != nil {
			return errors.Wrapf(err, "hold and release key %s failed", key)
		}
		// Poll for move action pressed; return error if feedback not received correctly;
		// here, we do not check for correct tap location.
		if err := pollTouchedCorrectly(ctx, params, moveMode, emptyHeuristic); err != nil {
			return errors.Wrapf(err, "failed to check key %s", key)
		}
		return nil
	}
}

// TapOverlayButton returns a function that takes in the given character corresponding
// to a tap keystroke and returns an error if tapping the keystroke did not result in
// the correct feedback.
func TapOverlayButton(kb *input.KeyboardEventWriter, key string, params *TestParams, heuristic ButtonHeuristics) action.Action {
	return func(ctx context.Context) error {
		// Tap given key, which is associated to an overlay button.
		if err := kb.Type(ctx, key); err != nil {
			return errors.Wrap(err, "failed to type key")
		}
		// Poll for tap action pressed; return error if feedback not received correctly.
		if err := pollTouchedCorrectly(ctx, params, tapMode, heuristic); err != nil {
			return errors.Wrapf(err, "failed to check key %s", key)
		}
		return nil
	}
}

// PopulateReceivedTimes populates the given array of events with event timestamps,
// as presented in logcat.
func PopulateReceivedTimes(ctx context.Context, params TestParams, numLines int) ([]inputlatency.InputEvent, error) {
	out, err := params.Arc.OutputLogcatGrep(ctx, "InputOverlayPerf")
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute logcat command")
	}
	lines := strings.Split(strings.Replace(string(out), ",", "", -1), "\n")
	// Last line can be empty.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	// Make sure that the length of the array is at least as long as expected.
	if len(lines) < numLines {
		return nil, errors.Errorf("only %v lines returned by logcat: %s", len(lines), lines[0])
	}
	lines = lines[len(lines)-numLines:]

	/*
	  An example line is shown below:

	  "09-19 13:01:22.298  4049  4049 V InputOverlayPerf: ACTION_UP 4146633898335"

	  For this test, all we do is to extract the timestamp shown at the end.
	*/

	events := make([]inputlatency.InputEvent, 0, numLines)
	for _, line := range lines {
		lineSplit := strings.Split(line, " ")
		timestamp, err := strconv.ParseInt(lineSplit[len(lineSplit)-1], 10, 64)
		if err != nil {
			return nil, errors.Wrap(err, "could not parse timestamp")
		}
		events = append(events, inputlatency.InputEvent{EventTimeNS: 0, RecvTimeNS: timestamp})
	}
	return events, nil
}

// pollTouchedCorrectly makes sure the feedback for a tap touch injection is correct.
// The mode parameter specifies whether three lines (move action) or two lines (tap action)
// should be checked for in logcat, while the x and y heuristic are percentages that
// denote the approximate location of an input overlay button on the screen, in
// the context of the Android phone window.
func pollTouchedCorrectly(ctx context.Context, params *TestParams, m mode, heuristic ButtonHeuristics) error {
	out, err := params.Arc.OutputLogcatGrep(ctx, "InputOverlayTest")
	if err != nil {
		return errors.Wrap(err, "failed to execute logcat command")
	}

	lines := strings.Split(strings.Replace(string(out), ",", "", -1), "\n")
	// Last line can be empty.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	// Make sure that the length of the array is at least as long as expected.
	if len(lines) < int(m) {
		return errors.Errorf("only %v lines returned by logcat: %s", len(lines), lines[0])
	}
	lines = lines[len(lines)-int(m):]
	firstLine := strings.Split(lines[0], " ")

	/*
	  An example line is shown below:

	  "05-03 09:03:45.233  2634  2634 V InputOverlayTest: MotionEvent { action=ACTION_UP,
	    actionButton=0, id[0]=0, x[0]=362.0, y[0]=642.0, toolType[0]=TOOL_TYPE_FINGER,
	    buttonState=0, classification=NONE, metaState=META_NUM_LOCK_ON, flags=0x1,
	    edgeFlags=0x0, pointerCount=1, historySize=0, eventTime=39410, downTime=39359,
	    deviceId=1, source=0x1002, displayId=0 }"

	  For this test, first of all, we care about the freshness of the log, which we
	  can extract from the second item (e.g. "09:03:45.233"). We also want to make note
	  of the action to verify that the correct sequence of actions took place. Finally,
	  for the first line, we care about the coordinates of the tap location, which are
	  given by the "x[0]=" and "y[0]=" elements. We parse this line to look for or
	  obtain all the above information.
	*/

	// Check for fresh timestamp.
	if firstLine[1] < params.lastTimestamp {
		return errors.New("action timestamp not fresh")
	}
	params.lastTimestamp = firstLine[1]

	// Check that the first line has "ACTION_UP" and the last line has "ACTION_DOWN".
	if !strings.Contains(lines[0], "ACTION_DOWN") {
		return errors.Errorf("ACTION_DOWN not found: %s", lines[0])
	}
	// Press-release buttons will also have an "ACTION_MOVE" motion event.
	if m == moveMode {
		if !strings.Contains(lines[1], "ACTION_MOVE") {
			return errors.Errorf("ACTION_MOVE not found: %s", lines[1])
		}
	}
	if !strings.Contains(lines[int(m)-1], "ACTION_UP") {
		return errors.Errorf("ACTION_UP not found: %s", lines[int(m)-1])
	}

	// No need to check positioning for joystick controls.
	if m == moveMode {
		return nil
	}

	// Get coordinate of tap reported in logcat for relative positioning.
	newPoint, err := parsePoint(firstLine)
	if err != nil {
		return errors.Wrapf(err, "failed to parse for a coordinate: %s", lines[0])
	}

	// Check that the tapped location is close enough.
	if err := confirmApproximateLocation(newPoint, params, heuristic); err != nil {
		return errors.Wrap(err, "failed to confirm approximate location")
	}

	return nil
}

// parsePoint returns the x and y coordinate contained within a logcat output line.
func parsePoint(line []string) (coords.Point, error) {
	empty := coords.Point{}
	xIdx := -1
	yIdx := -1
	for i, str := range line {
		if strings.HasPrefix(str, "x[0]=") {
			xIdx = i
		}
		if strings.HasPrefix(str, "y[0]=") {
			yIdx = i
			break
		}
	}
	if xIdx < 0 {
		return empty, errors.New("x coordinate for tap not found")
	}
	if yIdx < 0 {
		return empty, errors.New("y coordinate for tap not found")
	}
	// Both strings "x[0]=" and "y[0]=" are 5 characters long, and we exclude them
	// to extract the coordinate.
	x, err := strconv.ParseFloat(line[xIdx][5:], 32)
	if err != nil {
		return empty, errors.Wrap(err, "failed to parse x coordinate")
	}
	y, err := strconv.ParseFloat(line[yIdx][5:], 32)
	if err != nil {
		return empty, errors.Wrap(err, "failed to parse x coordinate")
	}
	return coords.NewPoint(int(x), int(y)), nil
}

// confirmApproximateLocation returns an error if the given point does not fall within the
// approximate location in the activity given by the heuristic parameters.
func confirmApproximateLocation(point coords.Point, params *TestParams, heuristic ButtonHeuristics) error {
	x := int(float64(params.windowContentSize.X) * heuristic.xHeuristic)
	y := int(float64(params.windowContentSize.Y) * heuristic.yHeuristic)
	if point.X < (x-errorMargin) || point.X > (x+errorMargin) {
		return errors.Errorf("x coordinate of tap (%d) not close enough to UI element on screen (%d)", point.X, x)
	}
	if point.Y < (y-errorMargin) || point.Y > (y+errorMargin) {
		return errors.Errorf("y coordinate of tap (%d) not close enough to UI element on screen (%d)", point.Y, y)
	}
	return nil
}

// captureScreenshot takes a screenshot and saves it with the provided filename.
// Since screenshots are useful in debugging but not important to the flow of the test,
// errors are logged rather than bubbled up.
func captureScreenshot(ctx context.Context, s *testing.State, cr *chrome.Chrome, filename string) {
	path := filepath.Join(s.OutDir(), filename)
	if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
		testing.ContextLog(ctx, "Failed to capture screenshot, info: ", err)
	} else {
		testing.ContextLogf(ctx, "Saved screenshot to %s", filename)
	}
}
