// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testutil contains functions and structs used for testing the gaming input overlay.
package testutil

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	apk = "ArcInputOverlayTest.apk"
	pkg = "org.chromium.arc.testapp.inputoverlay"
	cls = "org.chromium.arc.testapp.inputoverlay.MainActivity"

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
	// pressReleaseMode is the number of expected logcat lines from a press-release event.
	pressReleaseMode mode = 3
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
	lastTimestamp     string
	windowContentSize coords.Point
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

	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create a new ArcInputOverlayTest activity: ", err)
	}
	defer act.Close()

	// Start timing and launch the activity.
	startTime := time.Now()

	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatal("Failed to start ArcInputOverlayTest: ", err)
	}
	defer act.Stop(ctx, tconn)

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
		lastTimestamp:     "00:00:00.000",
	}); err != nil {
		s.Fatal("Failed to perform test: ", err)
	}
}

// RelaunchActivity opens the test application again.
func RelaunchActivity(ctx context.Context, params *TestParams) (*arc.Activity, error) {
	act, err := arc.NewActivity(params.Arc, pkg, cls)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a new ArcInputOverlayTest activity")
	}
	if err := act.StartWithDefaultOptions(ctx, params.TestConn); err != nil {
		return nil, errors.Wrap(err, "failed to restart ArcInputOverlayTest")
	}
	return act, nil
}

// MoveOverlayButton returns a function that takes in the given character corresponding
// to a move keystroke and returns an error if tapping the keystroke did not result in
// the correct feedback.
func MoveOverlayButton(kb *input.KeyboardEventWriter, key string, params *TestParams) action.Action {
	return func(ctx context.Context) error {
		// Hold and release given key, which is associated to an overlay button.
		if err := uiauto.Combine("Tap overlay keys and ensure proper behavior",
			kb.AccelPressAction(key),
			action.Sleep(WaitForActiveInputTime),
			kb.AccelReleaseAction(key),
		)(ctx); err != nil {
			return errors.Wrapf(err, "hold and release key %s failed", key)
		}
		// Poll for move action pressed; return error if feedback not received correctly;
		// here, we do not check for correct tap location.
		if err := pollTappedCorrectly(ctx, params, pressReleaseMode, emptyHeuristic); err != nil {
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
		if err := pollTappedCorrectly(ctx, params, tapMode, heuristic); err != nil {
			return errors.Wrapf(err, "failed to check key %s", key)
		}
		return nil
	}
}

// pollTappedCorrectly makes sure the feedback for a tap touch injection is correct.
// The mode parameter specifies whether three lines (move action) or two lines (tap action)
// should be checked for in logcat, while the x and y heuristic are percentages that
// denote the approximate location of an input overlay button on the screen, in
// the context of the Android phone window.
func pollTappedCorrectly(ctx context.Context, params *TestParams, m mode, heuristic ButtonHeuristics) error {
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
	if m == pressReleaseMode {
		if !strings.Contains(lines[1], "ACTION_MOVE") {
			return errors.Errorf("ACTION_MOVE not found: %s", lines[1])
		}
	}
	if !strings.Contains(lines[int(m)-1], "ACTION_UP") {
		return errors.Errorf("ACTION_UP not found: %s", lines[int(m)-1])
	}

	// No need to check positioning for joystick controls.
	if m == pressReleaseMode {
		return nil
	}

	// Get coordinate of tap reported in logcat for relative positioning.
	newPoint, err := parsePoint(firstLine)
	if err != nil {
		return errors.Wrapf(err, "failed to parse for a coordinate: %s", lines[0])
	}

	// Get window bounds, if necessary
	if err := calculateWindowContentSize(ctx, params); err != nil {
		return errors.Wrap(err, "failed to get application window bounds")
	}

	// Check that the tapped location is close enough.
	if err := approximateLocation(newPoint, params, heuristic); err != nil {
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
		if strings.Contains(str, "x[0]=") {
			xIdx = i
		}
		if strings.Contains(str, "y[0]=") {
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

// calculateWindowContentSize saves the window bounds of the GIO test application in the provided
// TestParams pointer.
func calculateWindowContentSize(ctx context.Context, params *TestParams) error {
	if params.windowContentSize != (coords.Point{}) {
		return nil
	}

	loc, err := params.Activity.SurfaceBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to obtain activity window bounds")
	}

	appWidth := loc.BottomRight().X - loc.TopLeft().X
	appHeight := loc.BottomRight().Y - loc.TopLeft().Y
	params.windowContentSize = coords.NewPoint(appWidth, appHeight)

	return nil
}

// approximateLocation returns an error if the given point does not fall within the
// approximate location in the activity given by the heuristic parameters.
func approximateLocation(point coords.Point, params *TestParams, heuristic ButtonHeuristics) error {
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
