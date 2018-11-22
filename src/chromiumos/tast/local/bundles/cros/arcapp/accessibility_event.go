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

// chromeVoxExtConn returns a connection to the ChromeVox extension's background page.
// If there is an error, the connection will be returned before closing.
// Otherwise the calling function will close the connection.
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
	// available: https://crbug.com/789313.
	if err := extConn.WaitForExpr(ctx, "ChromeVoxState.instance != null"); err != nil {
		extConn.Close()
		return nil, errors.Wrap(err, "ChromeVox unavailable")
	}

	testing.ContextLog(ctx, "Extension is ready")
	return extConn, nil
}

// clearChromeVoxLog clears the ChromeVox log.
// Returns an error, indicating success of clearing the log.
func clearChromeVoxLog(ctx context.Context, chromeVoxConn *chrome.Conn) error {
	const script = `LogStore.instance.clearLog();
		LogStore.instance.getLogs().toString();`
	var clearLogOutput string
	if err := chromeVoxConn.Eval(ctx, script, &clearLogOutput); err != nil {
		return errors.Wrap(err, "failed to clear log")
	}
	if clearLogOutput != "" {
		return errors.Errorf("log was not cleared, got %q, want %q", clearLogOutput, "")
	}
	return nil
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
		wantEvent, gotEvent := "", ""
		if i < len(gotEvents) {
			gotEvent = gotEvents[i]
		}
		if i < len(wantEvents) {
			wantEvent = wantEvents[i]
		}
		if strings.TrimSpace(gotEvent) != wantEvent {
			diffs = append(diffs, fmt.Sprintf("got: %q, want: %q", gotEvent, wantEvent))
		}
	}
	return diffs
}

// isAccessibilityEnabled checks if accessibility is enabled in Android.
// Returns a bool and error indicating whether or not accessibility was enabled, along with corresponding error.
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
func isElementChecked(ctx context.Context, chromeVoxConn *chrome.Conn, className string) error {
	script := fmt.Sprintf(
		`new Promise((resolve, reject) => {
		   chrome.automation.getFocus(function(node) {
		     if (node.className == "%s") {
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

// pollForFocusElement polls until specified UI element has focus.
// Returns error after 30 seconds.
func pollForFocusElement(ctx context.Context, chromeVoxConn *chrome.Conn, focusClassName string) error {
	const script = `new Promise((resolve, reject) => {
		   chrome.automation.getFocus(function(node) {
		     resolve(node.className);
		   });
		})`
	// Wait for UI element to be correctly focused.
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

// focusAndCheckElement uses ChromeVox navigation (using Tab), to navigate to the next
// UI element (specified by elementClass), and activates it (using Search + Space).
// Returns an error indicating the success of both actions.
func focusAndCheckElement(ctx context.Context, chromeVoxConn *chrome.Conn, elementClass string, expectedOutput []string, outputFilePath string) error {
	ew, err := input.Keyboard(ctx)
	if err != nil {
		ew.Close()
		return errors.Wrap(err, "error with creating EventWriter from keyboard")
	}
	defer ew.Close()

	// Ensure that ChromeVox log is cleared before proceeding.
	if err := clearChromeVoxLog(ctx, chromeVoxConn); err != nil {
		return errors.Wrap(err, "error with clearing ChromeVox Log")
	}

	// Move focus to the next UI element.
	if err := ew.Accel(ctx, "Tab"); err != nil {
		return errors.Wrap(err, "Accel(Tab) returned error")
	}

	// Wait for element to receive focus.
	if err := pollForFocusElement(ctx, chromeVoxConn, elementClass); err != nil {
		return errors.Wrap(err, "timed out polling for element")
	}

	// Activate (check)  the currently focused UI element.
	if err := ew.Accel(ctx, "Search+Space"); err != nil {
		return errors.Wrap(err, "Accel(Search + Space) returned error")
	}

	// Poll until the element has been checked.
	if err := isElementChecked(ctx, chromeVoxConn, elementClass); err != nil {
		return errors.Wrap(err, "failed to check toggled state")
	}

	// Ensure that generated accessibility event log matches expected event log.
	var gotOutput string
	if err := chromeVoxConn.Eval(ctx, `
		LogStore.instance.getLogsOfType(TextLog.LogType.EVENT).toString();
	`, &gotOutput); err != nil {
		return errors.Wrap(err, "failed to get event log")
	}

	// Determine if output matches expected value, and write to file if it does not match.
	if diff := getEventDiff(strings.Split(gotOutput, ","), expectedOutput); len(diff) != 0 {
		if err = ioutil.WriteFile(outputFilePath, []byte(strings.Join(diff, "\n")), 0644); err != nil {
			return errors.Errorf("failed to write to %q: %v", outputFilePath, err)
		}
	}
	return nil
}

func AccessibilityEvent(ctx context.Context, s *testing.State) {
	const (
		// This is a build of an application containing a single activity and basic UI elements.
		// The source code is in vendor/google_arc.
		packageName  = "org.chromium.arc.testapp.accessibility_sample"
		activityName = "org.chromium.arc.testapp.accessibility_sample.AccessibilityActivity"

		toggleButtonID = "org.chromium.arc.testapp.accessibility_sample:id/toggleButton"
		checkBoxID     = "org.chromium.arc.testapp.accessibility_sample:id/checkBox"

		checkBox     = "android.widget.CheckBox"
		toggleButton = "android.widget.ToggleButton"

		toggleButtonOutputFile = "accessibility_event_diff_toggle_button_output.txt"
		checkBoxOutputFile     = "accessibility_event_diff_checkbox_output.txt"
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

	if err := conn.EvalPromise(ctx, `
		chrome.accessibilityFeatures.spokenFeedback.set({value: true});
		new Promise((resolve, reject) => {
			chrome.accessibilityFeatures.spokenFeedback.get({}, (details) => {
				if (details.value) {
					resolve();
					return;
				}
			});
		})`, nil); err != nil {
		s.Fatal("Failed to enable spoken feedback: ", err)
	}

	// Wait until spoken feedback is enabled.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if res, err := isAccessibilityEnabled(ctx, a); err != nil {
			s.Fatal("Failed to check whether accessibility is enabled in Android: ", err)
		} else if !res {
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
		"DocumentURL = undefined"}
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
		"DocumentURL = undefined"}
	// Focus to and check checkBox element.
	if err := focusAndCheckElement(ctx, chromeVoxConn, checkBox, checkBoxOutput, filepath.Join(s.OutDir(), checkBoxOutputFile)); err != nil {
		s.Fatal("Failed focusing checkbox: ", err)
	}
}
