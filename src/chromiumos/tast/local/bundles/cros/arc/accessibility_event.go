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
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/accessibility"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// eventLog represents a log of accessibility event.
// Defined in https://cs.chromium.org/chromium/src/chrome/browser/resources/chromeos/chromevox/cvox2/background/log_types.js
type eventLog struct {
	EventType  string `json:"type_"`
	TargetName string `json:"targetName_"`
	RootName   string `json:"rootName_"`
	// eventLog has docUrl, but it will not be used in test.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilityEvent,
		Desc:         "Checks accessibility events in Chrome are as expected with ARC enabled",
		Contacts:     []string{"sarakato@chromium.org", "dtseng@chromium.org", "hirokisato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_both", "chrome"},
		Data:         []string{accessibility.ApkName},
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
				} else {
					reject();
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
func verifyLogs(ctx context.Context, chromeVoxConn *chrome.Conn, expectedLogs []eventLog) error {
	var logs []eventLog
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
func focusAndIncrementElement(ctx context.Context, chromeVoxConn *chrome.Conn, ew *input.KeyboardEventWriter, elementClass string, expectedLogs []eventLog, initialValue, expectedValue int) error {
	if err := moveToNext(ctx, chromeVoxConn, ew); err != nil {
		return err
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
func focusAndCheckElement(ctx context.Context, chromeVoxConn *chrome.Conn, ew *input.KeyboardEventWriter, elementClass string, expectedLogs []eventLog) error {
	if err := moveToNext(ctx, chromeVoxConn, ew); err != nil {
		return err
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

		seekBarInitialValue  = 25
		seekBarExpectedValue = 26

		seekBarDiscreteInitialValue  = 3
		seekBarDiscreteExpectedValue = 4
	)

	accessibility.RunTest(ctx, s, func(a *arc.ARC, chromeVoxConn *chrome.Conn, ew *input.KeyboardEventWriter) {
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

		expectedEventLog := func(eventType, targetName string) eventLog {
			return eventLog{
				EventType:  eventType,
				TargetName: targetName,
				RootName:   appName,
			}
		}

		// Focus to and toggle toggleButton element.
		toggleButtonLogs := []eventLog{
			expectedEventLog("focus", "OFF"),
			expectedEventLog("checkedStateChanged", "ON"),
		}
		if err := focusAndCheckElement(ctx, chromeVoxConn, ew, accessibility.ToggleButton, toggleButtonLogs); err != nil {
			s.Fatal("Failed focusing toggle button: ", err)
		}

		// Focus to and check checkBox element.
		checkBoxLogs := []eventLog{
			expectedEventLog("focus", "CheckBox"),
			expectedEventLog("checkedStateChanged", "CheckBox"),
		}
		if err := focusAndCheckElement(ctx, chromeVoxConn, ew, accessibility.CheckBox, checkBoxLogs); err != nil {
			s.Fatal("Failed focusing checkbox: ", err)
		}

		// Focus to and increment seekBar element.
		seekBarLogs := []eventLog{
			expectedEventLog("focus", "seekBar"),
			expectedEventLog("valueChanged", "seekBar"),
		}
		if err := focusAndIncrementElement(ctx, chromeVoxConn, ew, accessibility.SeekBar, seekBarLogs, seekBarInitialValue, seekBarExpectedValue); err != nil {
			s.Fatal("Failed focusing seekBar: ", err)
		}

		// Focus to and increment seekBarDiscrete element.
		seekBarDiscreteLogs := []eventLog{
			expectedEventLog("focus", "seekBarDiscrete"),
			expectedEventLog("valueChanged", "seekBarDiscrete"),
		}
		if err := focusAndIncrementElement(ctx, chromeVoxConn, ew, accessibility.SeekBar, seekBarDiscreteLogs, seekBarDiscreteInitialValue, seekBarDiscreteExpectedValue); err != nil {
			s.Fatal("Failed focusing seekBarDiscrete: ", err)
		}
	})
}
