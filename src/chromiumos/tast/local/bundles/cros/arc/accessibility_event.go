// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arc/accessibility"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// textLog represents a log that can be obtained by
// `LogStore.instance.getLogsOfType(TextLog.LogType.EVENT)`
// Defined in https://cs.chromium.org/chromium/src/chrome/browser/resources/chromeos/chromevox/cvox2/background/log_store.js?l=40
// TODO(crbug/987173): Use event object for LogStr instead of dumped string.
type textLog struct {
	LogStr  string `json:"logStr_"`
	LogType string `json:"logType"`
}

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

// verifyLogs gets the current ChromeVox log and checks that it matches with expected log.
func verifyLogs(ctx context.Context, chromeVoxConn *chrome.Conn, expectedLogs []textLog) error {
	var logs []textLog
	if err := chromeVoxConn.Eval(ctx, "LogStore.instance.getLogsOfType(LogStore.LogType.EVENT)", &logs); err != nil {
		return errors.Wrap(err, "failed to get event logs")
	}

	if !reflect.DeepEqual(logs, expectedLogs) {
		return errors.Errorf("event output is not as expected: got %q; want %q", logs, expectedLogs)
	}
	return nil
}

// focusAndIncrementElement uses ChromeVox navigation (using Tab), to navigate to the next
// UI element (specified by elementClass, and is expected to be a seekBar).
// ChromeVox will then interact with the seekBar, by incrementing its value using '='.
// Returns an error indicating the success of both actions.
func focusAndIncrementElement(ctx context.Context, chromeVoxConn *chrome.Conn, elementClass string, expectedLogs []textLog, initialValue, expectedValue int) error {
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
	if err := verifyLogs(ctx, chromeVoxConn, expectedLogs); err != nil {
		return err
	}
	return nil
}

// focusAndCheckElement uses ChromeVox navigation (using Tab), to navigate to the next
// UI element (specified by elementClass), and activates it (using Search + Space).
// Returns an error indicating the success of both actions.
func focusAndCheckElement(ctx context.Context, chromeVoxConn *chrome.Conn, elementClass string, expectedLogs []textLog) error {
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
	if err := verifyLogs(ctx, chromeVoxConn, expectedLogs); err != nil {
		return err
	}
	return nil
}

func AccessibilityEvent(ctx context.Context, s *testing.State) {
	const (
		apkName = "ArcAccessibilityTest.apk"
		appName = "Accessibility Test App"

		checkBox     = "android.widget.CheckBox"
		toggleButton = "android.widget.ToggleButton"
		seekBar      = "android.widget.SeekBar"

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

	// Each event is represented as a single string for logging in
	// https://cs.chromium.org/chromium/src/chrome/browser/resources/chromeos/chromevox/cvox2/background/event_stream_logger.js?&l=57
	expectedTextLog := func(eventType, targetName, documentURL string) textLog {
		return textLog{
			LogStr:  fmt.Sprintf("EventType = %s, TargetName = %s, RootName = %s, DocumentURL = %s", eventType, targetName, appName, documentURL),
			LogType: "event",
		}
	}

	// Focus to and toggle toggleButton element.
	toggleButtonLogs := []textLog{
		expectedTextLog("focus", "OFF", "undefined"),
		expectedTextLog("checkedStateChanged", "ON", "undefined"),
	}
	if err := focusAndCheckElement(ctx, chromeVoxConn, toggleButton, toggleButtonLogs); err != nil {
		s.Fatal("Failed focusing toggle button: ", err)
	}

	// Focus to and check checkBox element.
	checkBoxLogs := []textLog{
		expectedTextLog("focus", "CheckBox", "undefined"),
		expectedTextLog("checkedStateChanged", "CheckBox", "undefined"),
	}
	if err := focusAndCheckElement(ctx, chromeVoxConn, checkBox, checkBoxLogs); err != nil {
		s.Fatal("Failed focusing checkbox: ", err)
	}

	// Focus to and increment seekBar element.
	seekBarLogs := []textLog{
		expectedTextLog("focus", "seekBar", "undefined"),
		expectedTextLog("valueChanged", "seekBar", "undefined"),
	}
	if err := focusAndIncrementElement(ctx, chromeVoxConn, seekBar, seekBarLogs, seekBarInitialValue, seekBarExpectedValue); err != nil {
		s.Fatal("Failed focusing seekBar: ", err)
	}

	// Focus to and increment seekBarDiscrete element.
	seekBarDiscreteLogs := []textLog{
		expectedTextLog("focus", "seekBarDiscrete", "undefined"),
		expectedTextLog("valueChanged", "seekBarDiscrete", "undefined"),
	}
	if err := focusAndIncrementElement(ctx, chromeVoxConn, seekBar, seekBarDiscreteLogs, seekBarDiscreteInitialValue, seekBarDiscreteExpectedValue); err != nil {
		s.Fatal("Failed focusing seekBarDiscrete: ", err)
	}
}
