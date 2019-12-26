// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
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
// Defined in https://cs.chromium.org/chromium/src/chrome/browser/resources/chromeos/accessibility/chromevox/background/logging/log_types.js
type eventLog struct {
	EventType  string `json:"type_"`
	TargetName string `json:"targetName_"`
	RootName   string `json:"rootName_"`
	// eventLog has docUrl, but it will not be used in test.
}

type testStep struct {
	Key   string                       // key events to invoke the event.
	Node  accessibility.AutomationNode // expected focused node after the event.
	Event eventLog                     // expected event log.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilityEvent,
		Desc:         "Checks accessibility events in Chrome are as expected with ARC enabled",
		Contacts:     []string{"sarakato@chromium.org", "dtseng@chromium.org", "hirokisato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_both", "chrome"},
		Data:         []string{accessibility.ApkName},
		Timeout:      4 * time.Minute,
		Pre:          arc.Booted(),
	})
}

// verifyLog gets the current ChromeVox log and checks that it matches with expected log.
// Note that as the initial a11y focus is unstable, checkOnlyLatest=true can be used to check only the latest log.
func verifyLog(ctx context.Context, chromeVoxConn *chrome.Conn, expectedLog eventLog, checkOnlyLatest bool) error {
	var logs []eventLog
	if err := chromeVoxConn.Eval(ctx, "LogStore.instance.getLogsOfType(LogStore.LogType.EVENT)", &logs); err != nil {
		return errors.Wrap(err, "failed to get event logs")
	}

	if checkOnlyLatest && len(logs) > 1 {
		logs = logs[len(logs)-1:]
	}

	if len(logs) != 1 || !reflect.DeepEqual(logs[0], expectedLog) {
		return errors.Errorf("event output is not as expected: got %q; want %q", logs, expectedLog)
	}
	return nil
}

func runTestStep(ctx context.Context, chromeVoxConn *chrome.Conn, ew *input.KeyboardEventWriter, test testStep, isFirstStep bool) error {
	// Ensure that ChromeVox log is cleared before proceeding.
	if err := chromeVoxConn.Exec(ctx, "LogStore.instance.clearLog()"); err != nil {
		return errors.Wrap(err, "error with clearing ChromeVox log")
	}

	// Send a key event.
	if err := ew.Accel(ctx, test.Key); err != nil {
		return errors.Wrapf(err, "Accel(%s) returned error", test.Key)
	}

	// Wait for the focused element to match the expected.
	if err := accessibility.WaitForFocusedNode(ctx, chromeVoxConn, &test.Node); err != nil {
		return errors.Wrapf(err, "timed out polling for focused element, waiting for: %q", test.Node)
	}

	// Initial action sometimes invokes additional events (like focusing the entire application).
	// Latest logs should only be checked on the first iteration. (b/123397142#comment19)
	// TODO(b/142093176) Find the root cause.
	if err := verifyLog(ctx, chromeVoxConn, test.Event, isFirstStep); err != nil {
		return errors.Wrap(err, "failed to verify the log")
	}

	return nil
}

func AccessibilityEvent(ctx context.Context, s *testing.State) {
	const (
		appName = "Accessibility Test App"

		seekBarInitialValue         = 25
		seekBarDiscreteInitialValue = 3
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

		// Validate the initial state. Initially the focus is on the title node.
		titleNode := accessibility.AutomationNode{
			ClassName: accessibility.TextView,
		}
		if err := accessibility.WaitForFocusedNode(ctx, chromeVoxConn, &titleNode); err != nil {
			s.Fatal("Timed out polling for focused element, waiting for the title node: ", err)
		}

		// In ArcAccessibilityHelperService, accessibility events are dropped when the events are dispatched
		// within 200 ms of uptime after window state change. By waiting like this avoids dropping events.
		// TODO(hirokisato, b/xxxxxx) remove this wait.
		if err := accessibility.WaitForChromeVoxStopSpeaking(ctx, chromeVoxConn); err != nil {
			s.Fatal("Failed to wait for finishing ChromeVox speaking: ", err)
		}

		for i, test := range []testStep{
			// Move focus to ToggleButton and toggle it.
			{
				"Tab",
				accessibility.AutomationNode{
					ClassName: accessibility.ToggleButton,
					Checked:   "false",
				},
				eventLog{"focus", "OFF", appName},
			}, {
				"Search+Space",
				accessibility.AutomationNode{
					ClassName: accessibility.ToggleButton,
					Checked:   "true",
				},
				eventLog{"checkedStateChanged", "ON", appName},
			},
			// Move focus to CheckBox and check it.
			{
				"Tab",
				accessibility.AutomationNode{
					ClassName: accessibility.CheckBox,
					Checked:   "false",
				},
				eventLog{"focus", "CheckBox", appName},
			}, {
				"Search+Space",
				accessibility.AutomationNode{
					ClassName: accessibility.CheckBox,
					Checked:   "true",
				},
				eventLog{"checkedStateChanged", "CheckBox", appName},
			},
			// Move focus to SeekBar and increment it.
			{
				"Tab",
				accessibility.AutomationNode{
					ClassName:     accessibility.SeekBar,
					ValueForRange: seekBarInitialValue,
				},
				eventLog{"focus", "seekBar", appName},
			}, {
				"=",
				accessibility.AutomationNode{
					ClassName:     accessibility.SeekBar,
					ValueForRange: seekBarInitialValue + 1,
				},
				eventLog{"valueChanged", "seekBar", appName},
			},
			// Move focus to SeekbarDiscrete and decrement it.
			{
				"Tab",
				accessibility.AutomationNode{
					ClassName:     accessibility.SeekBar,
					ValueForRange: seekBarDiscreteInitialValue,
				},
				eventLog{"focus", "seekBarDiscrete", appName},
			}, {
				"-",
				accessibility.AutomationNode{
					ClassName:     accessibility.SeekBar,
					ValueForRange: seekBarDiscreteInitialValue - 1,
				},
				eventLog{"valueChanged", "seekBarDiscrete", appName},
			},
		} {
			if err := runTestStep(ctx, chromeVoxConn, ew, test, i == 0); err != nil {
				s.Fatalf("Failed to run a test step %q: %v", test, err)
			}
		}
	})
}
