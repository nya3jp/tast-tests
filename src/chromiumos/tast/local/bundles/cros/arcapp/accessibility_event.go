// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcapp

import (
	"context"
	"fmt"
	"io/ioutil"
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
		SoftwareDeps: []string{"android", "chrome_login"},
		Data:         []string{"accessibility_sample.apk"},
		Timeout:      4 * time.Minute,
	})
}

// ExtConn returns connection to a specified extension's background page.
// The caller should not close the returned connection; it will be closed
// automatically by Close.
// |extId| specifies ID of extension, for example: 'mndnfokpggljbaajbnioimlmbfngpief'
// |extPath| specifies path of extension, usually '/_generated_background_page.html'
func extConn(ctx context.Context, c *chrome.Chrome, extID string, extPath string) (*chrome.Conn, error) {
	if extID == "" || extPath == "" {
		return nil, errors.Errorf("extension ID or extension path is empty")
	}
	if extPath[0] != '/' {
		return nil, errors.Errorf("extension path does not begin with '/'")
	}
	extURL := "chrome-extension://" + extID + extPath
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
		if err := extConn.Eval(ctx, "cvox != null", &ready); err != nil {
			return err
		} else if !ready {
			return errors.New("no ChromeVox")
		}
		return nil
	}, &testing.PollOptions{Interval: 10 * time.Millisecond}); err != nil {
		return nil, errors.Wrap(err, "ChromeVox unavailable")
	}

	testing.ContextLog(ctx, "Extension is ready")
	return extConn, nil
}

// Computes difference between two strings of accessibility events.
// Returns an array containing line-by-line (event) diffs.
func getEventDiff(got, want string) []string {
	gotEvents, wantEvents := strings.Split(got, ","), strings.Split(want, ",")
	eventLength := len(gotEvents)
	var diffs []string
	if len(gotEvents) < len(wantEvents) {
		eventLength = len(wantEvents)
	}

	for i := 0; i < eventLength; i++ {
		// Check if the event is in range
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

func AccessibilityEvent(ctx context.Context, s *testing.State) {
	const (
		// This is a build of an application containing a single activity and basic UI elements..
		apk = "accessibility_sample.apk"
		pkg = "com.example.sarakato.accessibilitysample"
		cls = "com.example.sarakato.accessibilitysample.MainActivity"

		toggleButtonID = "com.example.sarakato.accessibilitysample:id/toggleButton"
		checkBoxID     = "com.example.sarakato.accessibilitysample:id/checkBox"
		cVoxExtID      = "mndnfokpggljbaajbnioimlmbfngpief"
		cVoxExtPath    = "/cvox2/background/background.html"

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

	apptest.RunWithChrome(ctx, s, cr, apk, pkg, cls, func(a *arc.ARC, d *ui.Device) {
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

	err = conn.Exec(ctx, `
		window.__spoken_feedback_set_complete = false;
		chrome.accessibilityFeatures.spokenFeedback.set({value: true});
		chrome.accessibilityFeatures.spokenFeedback.get({}, (details) => {
			window.__spoken_feedback_set_complete = value;
		});
	`)
	if err != nil {
		s.Fatal("Failed to enable spoken feedback: ", err)
	}

	chromeVoxConn, err := extConn(ctx, cr, cVoxExtID, cVoxExtPath)
	if err != nil {
		s.Fatal("Creating connection to ChromeVox extension failed: ", err)
	}
	defer chromeVoxConn.Close()

	// Setup event stream logging for accessibility events.
	err = chromeVoxConn.Exec(ctx, `
		chrome.automation.getDesktop(function(desktop) {
		    EventStreamLogger.instance = new EventStreamLogger(desktop);
		    EventStreamLogger.instance.notifyEventStreamFilterChangedAll(false);
		    EventStreamLogger.instance.notifyEventStreamFilterChange("FOCUS", true);
		    EventStreamLogger.instance.notifyEventStreamFilterChange("CHECKED_STATE_CHANGED", true);
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
	var clearLogOutput string
	err = chromeVoxConn.Eval(ctx, `
		LogStore.instance.clearLog();
		LogStore.instance.getLogs().toString();
	`, &clearLogOutput)
	if err != nil {
		s.Fatal("Failed to clear log: ", err)
	}
	if clearLogOutput != "" {
		s.Fatalf("Log was not cleared, got: '%s', want: ''", clearLogOutput)
	}

	// Focus onto first UI element.
	if err := ew.Accel(ctx, "Tab"); err != nil {
		s.Fatalf("Accel(Tab) returned error: %v", err)
	}

	// Wait for UI element to be correctly focused.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		var className string
		_ = chromeVoxConn.EvalPromise(ctx, `
		new Promise ((resolve, reject) => {
			chrome.automation.getFocus(function(node) {
				console.log(node); // check the property of the node, and makes sure it has focus.
				resolve (node["className"]);
			});
		})
		`, &className)
		if className == "android.widget.ToggleButton" {
			return nil
		}
		return errors.New("android.widget.ToggleButton does not have have focus")
	}, &testing.PollOptions{Timeout: 30 * time.Second})

	if err != nil {
		s.Fatalf("Failed to get current focus: ", err)
	}

	// Activate the currently focused UI element.
	if err := ew.Accel(ctx, "Search+Space"); err != nil {
		s.Fatalf("Accel(Search + Space) returned error: %v", err)
	}

	// Poll until the toggle button has been checked.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		var checked string
		_ = chromeVoxConn.EvalPromise(ctx, `
		new Promise ((resolve, reject) => {
			chrome.automation.getFocus(function(node) {
				resolve (node["checked"]);
			});
		})
		`, &checked)
		if checked == "true" {
			return nil
		}
		return errors.New("Timed out waiting for android.widget.ToggleButton to be checked")
	}, &testing.PollOptions{Timeout: 30 * time.Second})

	if err != nil {
		s.Fatalf("Failed to check toggled state: ", err)
	}
	// Ensure that generated accessibility event log matches expected event log.
	var gotOutput string
	err = chromeVoxConn.Eval(ctx, `
		LogStore.instance.getLogsOfType(TextLog.LogType.EVENT).toString();
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
		err = ioutil.WriteFile(accessibilityEventOut1, []byte(strings.Join(diff, "")), 0644)
		if err != nil {
			s.Fatalf("Failed to write to %s: ", accessibilityEventOut1, err)
		}
		s.Fatal(accessibilityEventOut1)
	}
	gotOutput = ""

	// Clear ChromeVox log before proceeding.
	err = chromeVoxConn.Exec(ctx, `
		LogStore.instance.clearLog();
	`)
	if err != nil {
		s.Fatal("Failed to clear log: ", err)
	}
	if clearLogOutput != "" {
		s.Fatalf("Log was not cleared, got: '%s', want: ''", clearLogOutput)
	}

	// Continue to activate second (Checkbox) UI element.
	if err := ew.Accel(ctx, "Tab"); err != nil {
		s.Fatalf("Accel(Tab) returned error: %v", err)
	}
	// Poll until chexk box has focus.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		var className string
		_ = chromeVoxConn.EvalPromise(ctx, `
		new Promise ((resolve, reject) => {
			chrome.automation.getFocus(function(node) {
				resolve (node["className"]);
			});
		})
		`, &className)
		if className == "android.widget.CheckBox" {
			return nil
		}
		return errors.New("focus not set correctly on node")
	}, &testing.PollOptions{Timeout: 30 * time.Second})

	if err := ew.Accel(ctx, "Search+Space"); err != nil {
		s.Fatalf("Accel(Search + Space) returned error: %v", err)
	}

	// Poll until the checkbox has been checked.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		var checked string
		_ = chromeVoxConn.EvalPromise(ctx, `
		new Promise ((resolve, reject) => {
			chrome.automation.getFocus(function(node) {
				resolve (node["checked"]);
			});
		})
		`, &checked)
		s.Log("checked value is:", checked)
		if checked == "true" {
			return nil
		}
		return errors.New("android.widget.Checkbox is not checked corrrectly")
	}, &testing.PollOptions{Timeout: 30 * time.Second})
	if err != nil {
		s.Fatal("Failed to check checkBox")
	}
	err = chromeVoxConn.Eval(ctx, `
		LogStore.instance.getLogsOfType(TextLog.LogType.EVENT).toString();
	`, &gotOutput)
	if err != nil {
		s.Fatal("Failed to get event log: ", err)
	}

	// Check ChromeVog log output matches with expected log.
	wantOutput = "EventType = focus, TargetName = CheckBox, RootName = undefined, DocumentURL = undefined,EventType = checkedStateChanged, TargetName = CheckBox, RootName = undefined, DocumentURL = undefined"
	if err != nil {
		s.Error("Failed reading internal data file: ", err)
	}

	// Write diff to external file.
	diff = getEventDiff(gotOutput, wantOutput)

	if len(diff) != 0 {
		err = ioutil.WriteFile(accessibilityEventOut2, []byte(strings.Join(diff, "")), 0644)
		if err != nil {
			s.Fatalf("Failed to write to %s: ", accessibilityEventOut2, err)
		}
		s.Log(strings.Join(diff, ""))
		s.Fatalf("Wrote diff to: %s", accessibilityEventOut2)
	}
}
