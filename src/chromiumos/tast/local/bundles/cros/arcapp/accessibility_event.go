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

// cvoxEextConn returns connection to the ChromeVox extension's background page.
// The caller should not close the returned connection; it will be closed
// automatically by Close.
// |extID| specifies ID of extension, for example: 'mndnfokpggljbaajbnioimlmbfngpief'
// |extPath| specifies path of extension, usually '/_generated_background_page.html'
func cvoxExtConn(ctx context.Context, c *chrome.Chrome) (*chrome.Conn, error) {
	extURL := "chrome-extension://mndnfokpggljbaajbnioimlmbfngpief/cvox2/background/background.html"
	testing.ContextLog(ctx, "Waiting for extension at ", extURL)
	f := func(t *chrome.Target) bool { return t.URL == extURL }

	extConn, err := c.NewConnForTarget(ctx, f)
	if err != nil {
		return nil, err
	}

	// Ensure that we don't attempt to use the extension before its APIs are
	// available: https://crbug.com/789313
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ready := false
		err := extConn.Eval(ctx, "ChromeVoxState.instance != null", &ready)
		if err != nil {
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

// Clears the chromevox log.
// Returns an error, which indicates the success of clearing log.
func clearChromevoxLog(ctx context.Context, chromeVoxConn *chrome.Conn) error {
	var clearLogOutput string
	err := chromeVoxConn.Eval(ctx, `LogStore.instance.clearLog();
					LogStore.instance.getLogs().toString();`, &clearLogOutput)
	if err != nil {
		return errors.Errorf("1 Failed to clear log: ", err)
	}
	if clearLogOutput != "" {
		return errors.Errorf("Log was not cleared, got: '%s', want: ''", clearLogOutput)
	}
	return nil
}

// Computes difference between two strings of accessibility events.
// Returns an array containing event diffs.
// The received events are split into an array, using "," as the demiliter.
// The difference is then obtained taking the diff of these two arrays.
func getEventDiff(got, want string) []string {
	gotEvents, wantEvents := strings.Split(got, ","), strings.Split(want, ",")

	eventLength := len(gotEvents)
	if len(gotEvents) < len(wantEvents) {
		eventLength = len(wantEvents)
	}

	gotEventIndex := 0
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

func isAccessibilityEnabled(ctx context.Context, a *arc.ARC) (bool, error) {
	cmd := a.Command(ctx, "settings", "--user", "0", "get", "secure", "accessibility_enabled")
	res, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return false, err
	}
	if strings.TrimSpace(string(res)) == "1" {
		return true, nil
	}
	return false, nil
}

// previous statement ensures that current node has focus.
// polls until specified ui has been checked, otherwise returns error after 30 seconds.
func isElementChecked(ctx context.Context, chromeVoxConn *chrome.Conn, className string) error {
	err := testing.Poll(ctx, func(ctx context.Context) error {
		var checked string
		err := chromeVoxConn.Eval(ctx, `
			var checked = false;
			chrome.automation.getFocus(function(node) {
				checked = node["checked"];
			});
			checked;
		`, &checked)
		if checked == "true" {
			return nil
		}
		return errors.Errorf("%s is not checked corrrectly: %v", className, err)
	}, &testing.PollOptions{Timeout: 30 * time.Second})
	if err != nil {
		return errors.Errorf("Failed to check if element is checked: %v", err)
	}
	return nil
}

// polls until specified ui element has focus, or otherwise, returns error after 30 seconds.
func pollForFocusElement(ctx context.Context, chromeVoxConn *chrome.Conn, focusClassName string) error {
	// Wait for UI element to be correctly focused.
	err := testing.Poll(ctx, func(ctx context.Context) error {
		var currFocusClassName string
		err := chromeVoxConn.Eval(ctx, `
			var result;
			chrome.automation.getFocus(function(node) {
				result = node["className"];
			});
			result;`, &currFocusClassName)
		if err != nil {
			return err
		}
		if strings.TrimSpace(currFocusClassName) == strings.TrimSpace(focusClassName) {
			return nil
		}
		return errors.Errorf("'%s' does not have focus, '%s' has focus instead", focusClassName, currFocusClassName)
	}, &testing.PollOptions{Timeout: 30 * time.Second})

	if err != nil {
		return errors.Errorf("Failed to get current focus: ", err)
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

		accessibilityEventOut1 = "accessibility_event_diff_output.txt"
		accessibilityEventOut2 = "accessibility_event_diff_output_2.txt"
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

	if err := conn.Exec(ctx, `
		window.__spoken_feedback_set_complete = false;
		chrome.accessibilityFeatures.spokenFeedback.set({value: true});
		chrome.accessibilityFeatures.spokenFeedback.get({}, (details) => {
			window.__spoken_feedback_set_complete = details.value;
		});
	`); err != nil {
		s.Fatal("Failed to enable spoken feedback: ", err)
	}

	// Wait until spoken feedback is enabled.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		res, err := isAccessibilityEnabled(ctx, a)
		if err != nil {
			s.Fatal("Failed to check whether accessibility is enabled in Android: ", err)
		}
		if res {
			return nil
		}
		return errors.New("accessibility not enabled")
	}, &testing.PollOptions{Timeout: 30 * time.Second})

	if err != nil {
		s.Fatal("Failed to ensure accessibility is enabled: ", err)
	}

	chromeVoxConn, err := cvoxExtConn(ctx, cr)
	if err != nil {
		s.Fatal("Creating connection to ChromeVox extension failed: ", err)
	}
	defer chromeVoxConn.Close()

	// Setup event stream logging for accessibility events.
	err = chromeVoxConn.Exec(ctx, `
			chrome.automation.getDesktop(function(desktop) {
			    EventStreamLogger.instance = new EventStreamLogger(desktop);
			    EventStreamLogger.instance.notifyEventStreamFilterChangedAll(false);
			    EventStreamLogger.instance.notifyEventStreamFilterChanged("FOCUS", true);
			    EventStreamLogger.instance.notifyEventStreamFilterChanged("CHECKED_STATE_CHANGED", true);
			});
	`)

	if err != nil {
		s.Fatal("Enabling event stream logging failed: ", err)
	}

	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error with creating EventWriter from keyboard: ", err)
	}
	defer ew.Close()

	// Ensure that ChromeVox log is cleared before proceeding.
	if err := clearChromevoxLog(ctx, chromeVoxConn); err != nil {
		s.Fatal("Error with clearing ChromeVox Log: ", err)
	}

	// Focus onto first UI element.
	if err := ew.Accel(ctx, "Tab"); err != nil {
		s.Fatal("Accel(Tab) returned error: ", err)
	}

	// Wait for toggle button to receive focus.
	if err := pollForFocusElement(ctx, chromeVoxConn, toggleButton); err != nil {
		s.Fatal("Timed out polling for element: ", err)
	}

	// Activate the currently focused UI element.
	if err := ew.Accel(ctx, "Search+Space"); err != nil {
		s.Fatalf("Accel(Search + Space) returned error: %v", err)
	}

	// Poll until the toggle button has been checked.
	if err := isElementChecked(ctx, chromeVoxConn, toggleButton); err != nil {
		s.Fatalf("Failed to check toggled state: ", err)
	}

	// Ensure that generated accessibility event log matches expected event log.
	var gotOutput string
	err = chromeVoxConn.Eval(ctx, `
		LogStore.instance.getLogs().toString();
	`, &gotOutput)
	if err != nil {
		s.Fatal("Failed to get event log: ", err)
	}

	// Check ChromeVog log output matches with expected log.
	wantOutput := "EventType = focus, TargetName = OFF, RootName = undefined, DocumentURL = undefined,EventType = checkedStateChanged, TargetName = ON, RootName = undefined, DocumentURL = undefined"
	if err != nil {
		s.Error("Failed reading internal data file: ", err)
	}

	// Determine if output matches expected value, and write to file if it does not match.
	diff := getEventDiff(gotOutput, string(wantOutput))
	if len(diff) != 0 {
		outputFilePath := filepath.Join(s.OutDir(), accessibilityEventOut1)
		err = ioutil.WriteFile(outputFilePath, []byte(strings.Join(diff, "")), 0644)
		if err != nil {
			s.Fatalf("Failed to write to %s: ", accessibilityEventOut1, err)
		}
		s.Fatal(accessibilityEventOut1)
	}

	// Clear ChromeVox log before proceeding.
	if err := clearChromevoxLog(ctx, chromeVoxConn); err != nil {
		s.Fatal("Error with clearing ChromeVox Log: ", err)
	}

	// Continue to activate second (Checkbox) UI element.
	if err := ew.Accel(ctx, "Tab"); err != nil {
		s.Fatalf("Accel(Tab) returned error: %v", err)
	}

	// Poll until checkBox has focus.
	if err := pollForFocusElement(ctx, chromeVoxConn, checkBox); err != nil {
		s.Fatal("Timed out polling for element: ", err)
	}

	if err := ew.Accel(ctx, "Search+Space"); err != nil {
		s.Fatalf("Accel(Search + Space) returned error: %v", err)
	}

	// Poll until the checkbox has been checked.

	// Poll until the toggle button has been checked.
	if err := isElementChecked(ctx, chromeVoxConn, toggleButton); err != nil {
		s.Fatalf("Failed to check toggled state: ", err)
	}

	var gotOutputCheckboxFocus = ""
	err = chromeVoxConn.Eval(ctx, `
		LogStore.instance.getLogsOfType(TextLog.LogType.EVENT).toString();
	`, &gotOutputCheckboxFocus)
	if err != nil {
		s.Fatal("Failed to get event log: ", err)
	}

	// Check ChromeVog log output matches with expected log.
	wantOutput = "EventType = focus, TargetName = CheckBox, RootName = undefined, DocumentURL = undefined,EventType = checkedStateChanged, TargetName = CheckBox, RootName = undefined, DocumentURL = undefined"
	if err != nil {
		s.Error("Failed reading internal data file: ", err)
	}

	// Write diff to external file.
	diff = getEventDiff(gotOutputCheckboxFocus, wantOutput)

	if len(diff) != 0 {
		outputFilePath := filepath.Join(s.OutDir(), accessibilityEventOut2)
		err = ioutil.WriteFile(outputFilePath, []byte(strings.Join(diff, "")), 0644)
		if err != nil {
			s.Fatalf("Failed to write to %s: ", accessibilityEventOut2, err)
		}
		s.Log(strings.Join(diff, ""))
		s.Fatalf("Wrote diff to: %s", accessibilityEventOut2)
	}
}
