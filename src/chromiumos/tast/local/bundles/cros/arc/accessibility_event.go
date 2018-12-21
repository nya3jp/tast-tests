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
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilityEvent,
		Desc:         "Checks accessibility events in Chrome are as expected with ARC enabled",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Data:         []string{"app-debug.apk"},
		Timeout:      4 * time.Minute,
	})
}

// chromeVoxExtConn returns a connection to the ChromeVox extension's background page.
// If the extension is not ready, the connection will be closed before returning.
// Otherwise the calling function will close the connection.
func chromeVoxExtConn(ctx context.Context, c *chrome.Chrome) (*chrome.Conn, error) {
	extURL := "chrome-extension://mndnfokpggljbaajbnioimlmbfngpief/cvox2/background/background.html"
	testing.ContextLog(ctx, "Waiting for extension at ", extURL)
	extConn, err := c.NewConnForTarget(ctx, chrome.MatchTargetURL(extURL))
	if err != nil {
		return nil, err
	}

	// Ensure that we don't attempt to use the extension before its APIs are
	// available: https://crbug.com/789313.
	if err := extConn.WaitForExpr(ctx, "ChromeVoxState.instance"); err != nil {
		extConn.Close()
		return nil, errors.Wrap(err, "ChromeVox unavailable")
	}

	testing.ContextLog(ctx, "Extension is ready")
	return extConn, nil
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

// waValueFocusedd polls until specified UI element with spacified value (expectedValue) has focus.
// Returns error after 30 seconds.
func waitForValueFocused(ctx context.Context, chromeVoxConn *chrome.Conn, className string, expectedValue int) error {
	script := fmt.Sprintf(
		`new Promise((resolve, reject) => {
			chrome.automation.getFocus((node) => {
				if (node.className === '%s') {
					resolve(node.valueForRange);
				}
			});
		})`, className)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var gotValue int
		if err := chromeVoxConn.EvalPromise(ctx, script, &gotValue); err != nil {
			return err
		}
		if gotValue != expectedValue {
			return errors.Errorf("%s is not increment correctly", className)
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to check if element is increment")
	}
	return nil
}

// waitForElementFocused polls until specified UI element (focusClassName) has focus.
// Returns error after 30 seconds.
func waitForElementFocused(ctx context.Context, chromeVoxConn *chrome.Conn, focusClassName string) error {
	const script = `new Promise((resolve, reject) => {
			chrome.automation.getFocus((node) => {
				resolve(node.className);
			});
		})`
	// Wait for focusClassName to receive focus.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var currFocusClassName string
		if err := chromeVoxConn.EvalPromise(ctx, script, &currFocusClassName); err != nil {
			return err
		}
		if strings.TrimSpace(currFocusClassName) != focusClassName {
			return errors.Errorf("%q does not have focus, %q has focus instead", focusClassName, currFocusClassName)
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to get current focus")
	}
	return nil
}

// getSeekBarValue returns the value of the currently focused seekBar.
func getSeekBarValue(ctx context.Context, chromeVoxConn *chrome.Conn, elementClass string) (int, error) {
	var currentValue int
	// Get Value of the focus element
	script := fmt.Sprintf(`new Promise((resolve, reject) => {
			chrome.automation.getFocus((node) => {
				if (node.className === '%s') {
					resolve(node.valueForRange);
				}
			});
		})`, elementClass)
	if err := chromeVoxConn.EvalPromise(ctx, script, &currentValue); err != nil {
		return 0, errors.Wrap(err, "could not get value of focused seekbar")
	}
	return currentValue, nil
}

// checkOutputLog gets the current ChromeVox log and checks that it matches with the expected log.
func checkOutputLog(ctx context.Context, chromeVoxConn *chrome.Conn, expectedOutput []string, outputFilePath string) error {
	var gotOutput string
	if err := chromeVoxConn.Eval(ctx, "LogStore.instance.getLogsOfType(TextLog.LogType.EVENT).toString()", &gotOutput); err != nil {
		return errors.Wrap(err, "failed to get event log")
	}

	// Determine if output matches expected value, and write to file if it does not match.
	if diff := getEventDiff(strings.Split(gotOutput, ","), expectedOutput); len(diff) != 0 {
		if err := ioutil.WriteFile(outputFilePath, []byte(strings.Join(diff, "\n")), 0644); err != nil {
			return errors.Errorf("failed to write to %q: %v", outputFilePath, err)
		}
	}
	return nil
}

// FocusAndIncrementElement uses ChromeVox navigation (using Tab), to navigate to the next
// UI element (specified by elementClass, and is expected to be a seekBar).
// ChromeVox will the interact with the seeekBar, by incrementing its value using '='.
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

	// Wait for element to receive focus.
	if err := waitForValueFocused(ctx, chromeVoxConn, elementClass, initialValue); err != nil {
		return errors.Wrap(err, "timed out polling for element")
	}

	// Check initial value of seekBar.
	currentValue, err := getSeekBarValue(ctx, chromeVoxConn, elementClass)
	if err != nil {
		return errors.Wrap(err, "could not get seekBar value")
	}

	if currentValue != initialValue {
		return errors.Errorf("seekBar value was not as expected, got %d want %d", currentValue, initialValue)
	}

	// Increment value of seekBar.
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

	// Wait for element to receive focus.
	if err := waitForElementFocused(ctx, chromeVoxConn, elementClass); err != nil {
		return errors.Wrap(err, "timed out polling for element")
	}

	// Activate (check) the currently focused UI element.
	if err := ew.Accel(ctx, "Search+Space"); err != nil {
		return errors.Wrap(err, "Accel(Search + Space) returned error")
	}

	// Poll until the element has been checked.
	if err := waitForElementChecked(ctx, chromeVoxConn, elementClass); err != nil {
		return errors.Wrap(err, "failed to check toggled state")
	}

	if err := checkOutputLog(ctx, chromeVoxConn, expectedOutput, outputFilePath); err != nil {
		return err
	}
	return nil
}

func AccessibilityEvent(ctx context.Context, s *testing.State) {
	const (
		// This is a build of an application containing a single activity and basic UI elements.
		// The source code is in vendor/google_arc.
		packageName  = "org.chromium.arc.testapp.accessibility_sample"
		activityName = "org.chromium.arc.testapp.accessibility_sample.AccessibilityActivity"

		toggleButtonID    = "org.chromium.arc.testapp.accessibility_sample:id/toggleButton"
		checkBoxID        = "org.chromium.arc.testapp.accessibility_sample:id/checkBox"
		seekBarID         = "org.chromium.arc.testapp.accessibility_sample:id/seekBar"
		seekBarDiscreteID = "org.chromium.arc.testapp.accessibility_sample:id/seekBarDiscrete"
		seekBarValue      = "org.chromium.arc.testapp.accessibility_sample:id/seekBarValue"

		checkBox     = "android.widget.CheckBox"
		toggleButton = "android.widget.ToggleButton"
		seekBar      = "android.widget.SeekBar"

		seekBarInitialValue  = 25
		seekBarExpectedValue = 26

		seekBarDiscreteInitialValue  = 3
		seekBarDiscreteExpectedValue = 4

		toggleButtonOutputFile    = "accessibility_event_diff_toggle_button_output.txt"
		checkBoxOutputFile        = "accessibility_event_diff_checkbox_output.txt"
		seekBarOutputFile         = "accessibility_event_diff_seekbar_output.txt"
		seekBarDiscreteOutputFile = "accessibility_event_diff_seekbar_discrete_output.txt"
	)

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs([]string{"--force-renderer-accessibility"}))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	// Setup UI Automator.
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	s.Log("Starting app")
	// Install accessibility_sample.apk
	if err := a.Install(ctx, s.DataPath("app-debug.apk")); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	// Run accessibility_sample.apk.
	if err := a.Command(ctx, "am", "start", "-W", packageName+"/"+activityName).Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	if err := d.Object(ui.ID(toggleButtonID)).WaitForExists(ctx); err != nil {
		s.Fatal(err)
	}
	if err := d.Object(ui.ID(checkBoxID)).WaitForExists(ctx); err != nil {
		s.Fatal(err)
	}
	if err := d.Object(ui.ID(seekBarID)).WaitForExists(ctx); err != nil {
		s.Fatal(err)
	}
	if err := d.Object(ui.ID(seekBarDiscreteID)).WaitForExists(ctx); err != nil {
		s.Fatal(err)
	}

	conn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if err := conn.EvalPromise(ctx, `
		new Promise((resolve, reject) => {
			chrome.accessibilityFeatures.spokenFeedback.set({value: true});
			chrome.accessibilityFeatures.spokenFeedback.get({}, (details) => {
				if (details.value) {
					resolve();
				} else {
					reject();
				}
			});
		})`, nil); err != nil {
		s.Fatal("Failed to enable spoken feedback: ", err)
	}

	// Wait until spoken feedback is enabled.
	cmd := a.Command(ctx, "settings", "--user", "0", "get", "secure", "accessibility_enabled")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if res, err := cmd.Output(); err != nil {
			cmd.DumpLog(ctx)
			s.Fatal("Failed to check whether accessibility is enabled in Android: ", err)
		} else if strings.TrimSpace(string(res)) != "1" {
			return errors.New("accessibility not enabled")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to ensure accessibility is enabled: ", err)
	}

	chromeVoxConn, err := chromeVoxExtConn(ctx, cr)
	if err != nil {
		s.Fatal("Creating connection to ChromeVox extension failed: ", err)
	}
	defer chromeVoxConn.Close()

	// Set up event stream logging for accessibility events.
	if err := chromeVoxConn.EvalPromise(ctx, `
		new Promise((resolve, reject) => {
			chrome.automation.getDesktop((desktop) => {
				EventStreamLogger.instance = new EventStreamLogger(desktop);
				EventStreamLogger.instance.notifyEventStreamFilterChangedAll(true);
				EventStreamLogger.instance.notifyEventStreamFilterChanged('focus', true);
				EventStreamLogger.instance.notifyEventStreamFilterChanged('checkedStateChanged', true);
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
	}
	// Focus to and increment seekBar element.
	if err := focusAndIncrementElement(ctx, chromeVoxConn, seekBar, seekBarOutput, filepath.Join(s.OutDir(), toggleButtonOutputFile), seekBarInitialValue, seekBarExpectedValue); err != nil {
		s.Fatal("Failed focusing seekBar: ", err)
	}

	seekBarDiscreteOutput := []string{
		"EventType = focus",
		"TargetName = seekBarDiscrete",
		"RootName = AccessibilitySample",
		"DocumentURL = undefined",
	}
	// Focus to and increment seekBarDiscrete element.
	if err := focusAndIncrementElement(ctx, chromeVoxConn, seekBar, seekBarDiscreteOutput, filepath.Join(s.OutDir(), toggleButtonOutputFile), seekBarDiscreteInitialValue, seekBarDiscreteExpectedValue); err != nil {
		s.Fatal("Failed focusing seekBarDiscrete: ", err)
	}
}
