// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcapp

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
	"chromiumos/tast/local/bundles/cros/arcapp/apptest"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilityEvent,
		Desc:         "Checks accessibility events in Chrome are as expected with ARC enabled",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login", "android_p"},
		Data:         []string{"accessibility_sample.apk"},
		Timeout:      4 * time.Minute,
	})
}

// chromeVoxExtConn returns connection to the ChromeVox extension's background page.
// The caller should not close the returned connection; it will be closed
// automatically by conn.Close.
func chromeVoxExtConn(ctx context.Context, c *chrome.Chrome) (*chrome.Conn, error) {
	extURL := "chrome-extension://mndnfokpggljbaajbnioimlmbfngpief/cvox2/background/background.html"
	testing.ContextLog(ctx, "Waiting for extension at ", extURL)
	f := func(t *chrome.Target) bool { return t.URL == extURL }

	extConn, err := c.NewConnForTarget(ctx, f)
	if err != nil {
		extConn.Close()
		return nil, err
	}

	// Ensure that we don't attempt to use the extension before its APIs are
	// available: https://crbug.com/789313
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ready := false
		if err := extConn.Eval(ctx, "ChromeVoxState.instance != null", &ready); err != nil {
			return err
		}
		if !ready {
			return errors.New("no ChromeVox")
		}
		return nil
	}, &testing.PollOptions{Interval: 10 * time.Millisecond}); err != nil {
		return nil, errors.Wrap(err, "ChromeVox unavailable")
	}

	testing.ContextLog(ctx, "Extension is ready")
	return extConn, nil
}

// clearChromeVoxLog will clear the ChromeVox log.
// Returns an error, which indicates the success of clearing log.
func clearChromeVoxLog(ctx context.Context, chromeVoxConn *chrome.Conn) error {
	const script = `
		LogStore.instance.clearLog();
		LogStore.instance.getLogs().toString();
	`
	var clearLogOutput string
	if err := chromeVoxConn.Eval(ctx, script, &clearLogOutput); err != nil {
		return errors.Errorf("Failed to clear log: ", err)
	}
	if clearLogOutput != "" {
		return errors.Errorf("Log was not cleared, got: '%q', want: ''", clearLogOutput)
	}
	return nil
}

// getEventDiff computes difference between two arrays of accessibility events.
// Returns an array containing event diffs.
// The difference is then obtained taking the diff of these two arrays.
func getEventDiff(gotEvents, wantEvents []string) []string {
	eventLength := len(gotEvents)
	if len(gotEvents) < len(wantEvents) {
		eventLength = len(wantEvents)
	}

	var diffs []string
	for i := 0; i < eventLength; i++ {
		// Check if the event is in range.
		wantEvent, gotEvent := "", ""
		if i < len(gotEvents) {
			gotEvent = gotEvents[i]
		}
		if i < len(wantEvents) {
			wantEvent = wantEvents[i]
		}
		if gotEvent != wantEvent {
			diffs = append(diffs, fmt.Sprintf("got: %s, want: %s \n", gotEvent, wantEvent))
		}
	}
	return diffs
}

// isAccessibilityEnabled checks if accessibility is enabled in Android.
// Returns bool indicating whether or not accessibility was enabled.
func isAccessibilityEnabled(ctx context.Context, a *arc.ARC) (bool, error) {
	cmd := a.Command(ctx, "settings", "--user", "0", "get", "secure", "accessibility_enabled")
	res, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return false, err
	}
	return strings.TrimSpace(string(res)) == "1", nil
}

// isElementChecked polls until UI element has been checked, otherwise returns error after 30 seconds.
// Type of UI element is checked in call to pollForFocusElement, before calling this method.
func isElementChecked(ctx context.Context, chromeVoxConn *chrome.Conn, className string) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var checked string
		if err := chromeVoxConn.EvalPromise(ctx, `
			new Promise((resolve, reject) => {
				chrome.automation.getFocus(function(node) {
					console.log(node.className + ", toggled:"+ node.checked);
					if (node.className == "`+className+`") {
						resolve(node.checked);
					}
				});
			})
		`, &checked); err != nil {
			return err
		}
		if checked == "true" {
			return nil
		}
		return errors.Errorf("%s is not checked corrrectly", className)
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Errorf("Failed to check if element is checked: %v", err)
	}
	return nil
}

// pollForFocusElement will poll  until specified UI element has focus, or otherwise, returns error after 30 seconds.
func pollForFocusElement(ctx context.Context, chromeVoxConn *chrome.Conn, focusClassName string) error {
	// Wait for UI element to be correctly focused.
	err := testing.Poll(ctx, func(ctx context.Context) error {
		var currFocusClassName string
		err := chromeVoxConn.EvalPromise(ctx, `
			new Promise((resolve, reject) => {
				chrome.automation.getFocus(function(node) {
					resolve(node.className);
					return;
				});
			})`, &currFocusClassName)
		if err != nil {
			return err
		}
		if strings.TrimSpace(currFocusClassName) == focusClassName {
			return nil
		}
		return errors.Errorf("'%s' does not have focus, '%s' has focus instead", focusClassName, currFocusClassName)
	}, &testing.PollOptions{Timeout: 30 * time.Second})

	if err != nil {
		return errors.Errorf("failed to get current focus: ", err)
	}
	return nil
}

// focusAndCheckElement will use ChromeVox navigation (using Tab), to navigate to the
// specified UI element (specified by ID), and activate the element (using Search + Space).
// error will be returned, indicating the success of the events.
func focusAndCheckElement(ctx context.Context, chromeVoxConn *chrome.Conn, elementClass string, expectedOutput []string, outputFilePath string) error {
	ew, err := input.Keyboard(ctx)
	if err != nil {
		errors.Errorf("Error with creating EventWriter from keyboard: ", err)
	}
	defer ew.Close()

	// Ensure that ChromeVox log is cleared before proceeding.
	if err := clearChromeVoxLog(ctx, chromeVoxConn); err != nil {
		return errors.Errorf("Error with clearing ChromeVox Log: ", err)
	}

	// Focus onto first UI element.
	if err := ew.Accel(ctx, "Tab"); err != nil {
		return errors.Errorf("Accel(Tab) returned error: ", err)
	}

	// Wait for toggle button to receive focus.
	if err := pollForFocusElement(ctx, chromeVoxConn, elementClass); err != nil {
		return errors.Errorf("Timed out polling for element: ", err)
	}

	// Activate the currently focused UI element.
	if err := ew.Accel(ctx, "Search+Space"); err != nil {
		return errors.Errorf("Accel(Search + Space) returned error: ", err)
	}

	// Poll until the toggle button has been checked.
	if err := isElementChecked(ctx, chromeVoxConn, elementClass); err != nil {
		return errors.Errorf("Failed to check toggled state: ", err)
	}

	// Ensure that generated accessibility event log matches expected event log.
	var gotOutput string
	err = chromeVoxConn.Eval(ctx, `
	LogStore.prototype.getLogsOfType = function(logType) {
  var returnLogs = [];
  for (var i = 0; i < LogStore.LOG_LIMIT; i++) {
    var index = (this.startIndex_ + i) % LogStore.LOG_LIMIT;
    if (!this.logs_[index])
      continue;
    if (this.logs_[index].logType == logType)
      returnLogs.push(this.logs_[index]);
  }
  return returnLogs;
};

		LogStore.instance.getLogsOfType(TextLog.LogType.EVENT).toString();
	`, &gotOutput)
	if err != nil {
		return errors.Errorf("Failed to get event log: ", err)
	}

	// Check ChromeVog log output matches with expected log.
	if err != nil {
		return errors.Errorf("Failed reading internal data file: ", err)
	}

	// Determine if output matches expected value, and write to file if it does not match.
	diff := getEventDiff(strings.Split(gotOutput, ","), expectedOutput)
	if len(diff) != 0 {
		err = ioutil.WriteFile(outputFilePath, []byte(strings.Join(diff, "")), 0644)
		if err != nil {
			return errors.Errorf("Failed to write to %q: ", outputFilePath, err)
		}
	}
	return nil
}

func AccessibilityEvent(ctx context.Context, s *testing.State) {
	const (
		// This is a build of an application containing a single activity and basic UI elements.
		packageName  = "org.chromium.arc.testapp.accessibility_sample"
		activityName = "org.chromium.arc.testapp.accessibility_sample.AccessibilityActivity"

		toggleButtonID = "org.chromium.arc.testapp.accessibility_sample:id/toggleButton"
		checkBoxID     = "org.chromium.arc.testapp.accessibility_sample:id/checkBox"

		checkBox     = "android.widget.CheckBox"
		toggleButton = "android.widget.ToggleButton"

		toggleButtonFile = "accessibility_event_diff_output.txt"
		checkBoxFile     = "accessibility_event_diff_output_2.txt"
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

	apptest.RunWithChrome(ctx, s, cr, "accessibility_sample.apk", packageName, activityName, func(a *arc.ARC, d *ui.Device) {
		if err := d.Object(ui.ID(toggleButtonID)).WaitForExists(ctx); err != nil {
			s.Fatal(err)
		}
		if err := d.Object(ui.ID(checkBoxID)).WaitForExists(ctx); err != nil {
			s.Fatal(err)
		}
	})

	conn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer conn.Close()

	if err := conn.EvalPromise(ctx, `
		window.__spoken_feedback_set_complete = false;
		chrome.accessibilityFeatures.spokenFeedback.set({value: true});
		new Promise((resolve, reject) => {
			chrome.accessibilityFeatures.spokenFeedback.get({}, (details) => {
				window.__spoken_feedback_set_complete = details.value;
				resolve();
			});
		})
	`, nil); err != nil {
		s.Fatal("Failed to enable spoken feedback: ", err)
	}

	// Wait until spoken feedback is enabled.
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		res, err := isAccessibilityEnabled(ctx, a)
		if err != nil {
			s.Fatal("Failed to check whether accessibility is enabled in Android: ", err)
		}
		if !res {
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
			chrome.automation.getDesktop(function(desktop) {
				EventStreamLogger.instance = new EventStreamLogger(desktop);
				EventStreamLogger.instance.notifyEventStreamFilterChangedAll(false);
				EventStreamLogger.instance.notifyEventStreamFilterChanged('FOCUS', true);
				EventStreamLogger.instance.notifyEventStreamFilterChanged('CHECKED_STATE_CHANGED', true);
				resolve();
				return;
			});
		})
	`, nil); err != nil {
		s.Fatal("Enabling event stream logging failed: ", err)
	}

	toggleButtonOutput := []string{"EventType = focus",
		"TargetName = OFF",
		"RootName = undefined",
		"DocumentURL = undefined",
		"EventType = checkedStateChanged",
		"TargetName = ON",
		"RootName = undefined",
		"DocumentURL = undefined"}
	// Focus to and toggle toggleButton element.
	if err := focusAndCheckElement(ctx, chromeVoxConn, toggleButton, toggleButtonOutput, filepath.Join(s.OutDir(), toggleButtonFile)); err != nil {
		s.Fatal(err)
	}

	checkBoxOutput := []string{"EventType = focus",
		"TargetName = CheckBox",
		"RootName = undefined",
		"DocumentURL = undefined",
		"EventType = checkedStateChanged",
		"TargetName = CheckBox",
		"RootName = undefined",
		"DocumentURL = undefined"}
	// Focus to and check checkBox element.
	if err := focusAndCheckElement(ctx, chromeVoxConn, checkBox, checkBoxOutput, filepath.Join(s.OutDir(), checkBoxFile)); err != nil {
		s.Fatal(err)
	}
}
