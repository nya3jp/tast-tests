// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arc/accessibility"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilityEvent,
		Desc:         "Checks accessibility events in Chrome are as expected with ARC enabled",
		Contacts:     []string{"sarakato@chromium.org", "dtseng@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome"},
		Data:         []string{"ArcAccessibilityTest.apk"},
		Timeout:      4 * time.Minute,
	})
}

// getEventDiff computes difference between two arrays of accessibility events.
// Difference is obtained by taking the diff of these two arrays.
// Returns an array containing event diffs.
func getEventDiff(gotEvents, wantEvents []string) []string {
	eventLength := len(gotEvents)
	if len(gotEvents) < len(wantEvents) {
		eventLength = len(wantEvents)
	}

	var diffs []string
	for i := 0; i < eventLength; i++ {
		// Check if the event is in range.
		var wantEvent, gotEvent string
		if i < len(gotEvents) {
			gotEvent = gotEvents[i]
		}
		if i < len(wantEvents) {
			wantEvent = wantEvents[i]
		}
		if gotEvent != wantEvent {
			diffs = append(diffs, fmt.Sprintf("got %q, want %q", gotEvent, wantEvent))
		}
	}
	return diffs
}

// waitForElementChecked polls until UI element has been checked, otherwise returns error after 30 seconds.
func waitForElementChecked(ctx context.Context, chromeVoxConn *chrome.Conn, className string) error {
	script := fmt.Sprintf(
		`new Promise((resolve, reject) => {
			chrome.automation.getFocus((node) => {
				if (node.className === '%s') {
					resolve(node.checked);
				}
			});
		})`, className)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var checked string
		if err := chromeVoxConn.EvalPromise(ctx, script, &checked); err != nil {
			return err
		}
		if checked == "false" {
			return errors.Errorf("%s is unchecked", className)
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to check if element is checked")
	}
	return nil
}

// waitForValueFocused polls until specified UI element with specified value (expectedValue) has focus.
// Returns error after 30 seconds.
func waitForValueFocused(ctx context.Context, chromeVoxConn *chrome.Conn, className string, expectedValue int) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var gotValue int
		gotValue, err := getValueForFocusedElement(ctx, chromeVoxConn, className)
		if err != nil {
			return err
		}
		if gotValue != expectedValue {
			return errors.Errorf("%q does not have expected value; got %d, want %d", className, gotValue, expectedValue)
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "timed out waiting for element to receive focus")
	}
	return nil
}

// getValueForFocusedElement returns the value of the currently focused element.
func getValueForFocusedElement(ctx context.Context, chromeVoxConn *chrome.Conn, elementClass string) (int, error) {
	var currentValue int
	script := fmt.Sprintf(`
		new Promise((resolve, reject) => {
			chrome.automation.getFocus((node) => {
				if (node.className === %q) {
					resolve(node.valueForRange);
				} else {
					reject();
				}
			});
		})`, elementClass)
	if err := chromeVoxConn.EvalPromise(ctx, script, &currentValue); err != nil {
		return 0, errors.Wrapf(err, "could not get value of focused element %q", elementClass)
	}
	return currentValue, nil
}

// checkOutputLog gets the current ChromeVox log and checks that it matches with expected log.
func checkOutputLog(ctx context.Context, chromeVoxConn *chrome.Conn, expectedOutput []string, outputFilePath string) error {
	var gotOutput string
	if err := chromeVoxConn.Eval(ctx, "LogStore.instance.getLogsOfType(LogStore.LogType.EVENT).toString()", &gotOutput); err != nil {
		return errors.Wrap(err, "failed to get event log")
	}

	// Determine if output matches expected value, and write to file if it does not match.
	if diff := getEventDiff(strings.Split(gotOutput, ","), expectedOutput); len(diff) != 0 {
		if err := ioutil.WriteFile(outputFilePath, []byte(strings.Join(diff, "\n")), 0644); err != nil {
			return errors.Wrapf(err, "failed to write to %q", outputFilePath)
		}
	}
	return nil
}

// focusAndIncrementElement uses ChromeVox navigation (using Tab), to navigate to the next
// UI element (specified by elementClass, and is expected to be a seekBar).
// ChromeVox will then interact with the seekBar, by incrementing its value using '='.
// Returns an error indicating the success of both actions.
func focusAndIncrementElement(ctx context.Context, chromeVoxConn *chrome.Conn, elementClass string, expectedOutput []string, outputFilePath string, initialValue, expectedValue int) error {
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "error with creating EventWriter from keyboard")
	}
	defer ew.Close()

	// Ensure that ChromeVox log is cleared before proceeding.
	if err := chromeVoxConn.EvalPromise(ctx, "LogStore.instance.clearLog()", nil); err != nil {
		return errors.Wrap(err, "error with clearing ChromeVox Log")
	}

	// Move focus to the next UI element.
	if err := ew.Accel(ctx, "Tab"); err != nil {
		return errors.Wrap(err, "Accel(Tab) returned error")
	}

	// Make sure that seekBar is focused with expected initial value.
	if err := waitForValueFocused(ctx, chromeVoxConn, elementClass, initialValue); err != nil {
		return errors.Wrap(err, "timed out polling for element")
	}

	// Increment value of seekBar by ChromeVox key combination.
	if err := ew.Accel(ctx, "="); err != nil {
		return errors.Wrap(err, "Accel(=) returned error")
	}

	// Check that seekbar was incremented correctly.
	if err := waitForValueFocused(ctx, chromeVoxConn, elementClass, expectedValue); err != nil {
		return errors.Wrap(err, "timed out polling for element incremented")
	}
	if err := checkOutputLog(ctx, chromeVoxConn, expectedOutput, outputFilePath); err != nil {
		return err
	}
	return nil
}

// focusAndCheckElement uses ChromeVox navigation (using Tab), to navigate to the next
// UI element (specified by elementClass), and activates it (using Search + Space).
// Returns an error indicating the success of both actions.
func focusAndCheckElement(ctx context.Context, chromeVoxConn *chrome.Conn, elementClass string, expectedOutput []string, outputFilePath string) error {
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "error with creating EventWriter from keyboard")
	}
	defer ew.Close()

	// Ensure that ChromeVox log is cleared before proceeding.
	if err := chromeVoxConn.Exec(ctx, "LogStore.instance.clearLog()"); err != nil {
		return errors.Wrap(err, "error with clearing ChromeVox Log")
	}
	// Move focus to the next UI element.
	if err := ew.Accel(ctx, "Tab"); err != nil {
		return errors.Wrap(err, "Accel(Tab) returned error")
	}

	if err := accessibility.WaitForChromeVoxStopSpeaking(ctx, chromeVoxConn); err != nil {
		return errors.Wrap(err, "could not check if ChromeVox is speaking")
	}

	// Wait for element to receive focus.
	if err := accessibility.WaitForElementFocused(ctx, chromeVoxConn, elementClass); err != nil {
		return errors.Wrap(err, "timed out polling for element")
	}

	// Activate (check) the currently focused UI element.
	if err := ew.Accel(ctx, "Search+Space"); err != nil {
		return errors.Wrap(err, "Accel(Search + Space) returned error")
	}

	if err := accessibility.WaitForChromeVoxStopSpeaking(ctx, chromeVoxConn); err != nil {
		return errors.Wrap(err, "could not check if ChromeVox is speaking")
	}

	// Poll until the element has been checked.
	if err := waitForElementChecked(ctx, chromeVoxConn, elementClass); err != nil {
		return errors.Wrap(err, "failed to check toggled state")
	}

	// Determine if output matches expected value, and write to file if it does not match.
	if err := checkOutputLog(ctx, chromeVoxConn, expectedOutput, outputFilePath); err != nil {
		return err
	}
	return nil
}

func AccessibilityEvent(ctx context.Context, s *testing.State) {
	const (
		apkName = "ArcAccessibilityTest.apk"

		checkBox     = "android.widget.CheckBox"
		toggleButton = "android.widget.ToggleButton"
		seekBar      = "android.widget.SeekBar"

		toggleButtonOutputFile    = "accessibility_event_diff_toggle_button_output.txt"
		checkBoxOutputFile        = "accessibility_event_diff_checkbox_output.txt"
		seekBarOutputFile         = "accessibility_event_diff_seekbar_output.txt"
		seekBarDiscreteOutputFile = "accessibility_event_diff_seekbar_discrete_output.txt"

		seekBarInitialValue  = 25
		seekBarExpectedValue = 26

		seekBarDiscreteInitialValue  = 3
		seekBarDiscreteExpectedValue = 4
	)
	cr, err := accessibility.NewChrome(ctx)
	if err != nil {
		s.Fatal(err) // NOLINT: arc/ui returns loggable errors
	}
	defer cr.Close(ctx)

	a, err := accessibility.NewARC(ctx, s.OutDir())
	if err != nil {
		s.Fatal(err) // NOLINT: arc/ui returns loggable errors
	}
	defer a.Close()

	if err := accessibility.InstallAndStartSampleApp(ctx, a, s.DataPath(apkName)); err != nil {
		s.Fatal("Setting up ARC environment with accessibility failed: ", err)
	}

	if err := accessibility.EnableSpokenFeedback(ctx, cr, a); err != nil {
		s.Fatal(err) // NOLINT: arc/ui returns loggable errors
	}

	chromeVoxConn, err := accessibility.ChromeVoxExtConn(ctx, cr)
	if err != nil {
		s.Fatal("Creating connection to ChromeVox extension failed: ", err)
	}
	defer chromeVoxConn.Close()

	// Wait for ChromeVox to stop speaking before interacting with it further.
	if err := accessibility.WaitForChromeVoxStopSpeaking(ctx, chromeVoxConn); err != nil {
		s.Fatal("Could not wait for ChromeVox to stop speaking: ", err)
	}

	// Set up event stream logging for accessibility events.
	if err := chromeVoxConn.EvalPromise(ctx, `
		new Promise((resolve, reject) => {
			chrome.automation.getDesktop((desktop) => {
				EventStreamLogger.instance = new EventStreamLogger(desktop);
				EventStreamLogger.instance.notifyEventStreamFilterChangedAll(false);
				EventStreamLogger.instance.notifyEventStreamFilterChanged('focus', true);
				EventStreamLogger.instance.notifyEventStreamFilterChanged('checkedStateChanged', true);
				EventStreamLogger.instance.notifyEventStreamFilterChanged('valueChanged', true);

				resolve();
			});
		})`, nil); err != nil {
		s.Fatal("Enabling event stream logging failed: ", err)
	}

	toggleButtonOutput := []string{
		"EventType = focus",
		"TargetName = OFF",
		"RootName = undefined",
		"DocumentURL = undefined",
		"EventType = checkedStateChanged",
		"TargetName = ON",
		"RootName = undefined",
		"DocumentURL = undefined",
	}
	// Focus to and toggle toggleButton element.
	if err := focusAndCheckElement(ctx, chromeVoxConn, toggleButton, toggleButtonOutput, filepath.Join(s.OutDir(), toggleButtonOutputFile)); err != nil {
		s.Fatal("Failed focusing toggle button: ", err)
	}

	checkBoxOutput := []string{
		"EventType = focus",
		"TargetName = CheckBox",
		"RootName = undefined",
		"DocumentURL = undefined",
		"EventType = checkedStateChanged",
		"TargetName = CheckBox",
		"RootName = undefined",
		"DocumentURL = undefined",
	}
	// Focus to and check checkBox element.
	if err := focusAndCheckElement(ctx, chromeVoxConn, checkBox, checkBoxOutput, filepath.Join(s.OutDir(), checkBoxOutputFile)); err != nil {
		s.Fatal("Failed focusing checkbox: ", err)
	}

	seekBarOutput := []string{
		"EventType = focus",
		"TargetName = seekBar",
		"RootName = AccessibilitySample",
		"DocumentURL = undefined",
		"EventType = valueChanged",
		"TargetName = seekBar",
		"RootName = AccessibilitySample",
		"DocumentURL = undefined",
	}
	// Focus to and increment seekBar element.
	if err := focusAndIncrementElement(ctx, chromeVoxConn, seekBar, seekBarOutput, filepath.Join(s.OutDir(), seekBarOutputFile), seekBarInitialValue, seekBarExpectedValue); err != nil {
		s.Fatal("Failed focusing seekBar: ", err)
	}

	seekBarDiscreteOutput := []string{
		"EventType = focus",
		"TargetName = seekBarDiscrete",
		"RootName = AccessibilitySample",
		"DocumentURL = undefined",
		"EventType = valueChanged",
		"TargetName = seekBarDiscrete",
		"RootName = AccessibilitySample",
		"DocumentURL = undefined",
	}
	// Focus to and increment seekBarDiscrete element.
	if err := focusAndIncrementElement(ctx, chromeVoxConn, seekBar, seekBarDiscreteOutput, filepath.Join(s.OutDir(), seekBarDiscreteOutputFile), seekBarDiscreteInitialValue, seekBarDiscreteExpectedValue); err != nil {
		s.Fatal("Failed focusing seekBarDiscrete: ", err)
	}
}
