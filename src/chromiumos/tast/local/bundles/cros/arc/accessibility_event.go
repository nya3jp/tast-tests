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
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_both", "chrome"},
		Data:         []string{accessibility.ApkName},
		Timeout:      4 * time.Minute,
	})
}

// verifyLogs gets the current ChromeVox log and checks that it matches with expected log.
// Note that as the initial a11y focus is unstable, checkOnlyLatest=true can be used to check only the latest logs.
func verifyLogs(ctx context.Context, chromeVoxConn *chrome.Conn, expectedLogs []eventLog, checkOnlyLatest bool) error {
	var logs []eventLog
	if err := chromeVoxConn.Eval(ctx, "LogStore.instance.getLogsOfType(LogStore.LogType.EVENT)", &logs); err != nil {
		return errors.Wrap(err, "failed to get event logs")
	}

	if checkOnlyLatest && len(logs) > len(expectedLogs) {
		logs = logs[len(logs)-len(expectedLogs) : len(logs)]
	}

	if !reflect.DeepEqual(logs, expectedLogs) {
		return errors.Errorf("event output is not as expected: got %q; want %q", logs, expectedLogs)
	}
	return nil
}

// focusAndIncrementElement uses ChromeVox navigation (using Tab), to navigate to the next
// UI element (specified by node).
// ChromeVox will then interact with the seekBar, by incrementing its value using '='.
// node is the node that initially receives focus, and expectedNode is the node containing
// the expected value after incrementing node.
// Returns an error indicating the success of both actions.
func focusAndIncrementElement(ctx context.Context, chromeVoxConn *chrome.Conn, node, expectedNode *accessibility.AutomationNode) error {
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "error with creating EventWriter from keyboard")
	}
	defer ew.Close()

	// Move focus to the next UI element.
	if err := ew.Accel(ctx, "Tab"); err != nil {
		return errors.Wrap(err, "Accel(Tab) returned error")
	}

	// Make sure that seekBar is focused with expected initial value.
	if err := accessibility.WaitForFocusedNode(ctx, chromeVoxConn, node); err != nil {
		return errors.Wrap(err, "timed out polling for element")
	}

	// Increment value of seekBar by ChromeVox key combination.
	if err := ew.Accel(ctx, "="); err != nil {
		return errors.Wrap(err, "Accel(=) returned error")
	}

	// Check that seekbar was incremented correctly.
	if err := accessibility.WaitForFocusedNode(ctx, chromeVoxConn, expectedNode); err != nil {
		return errors.Wrap(err, "timed out polling for element incremented")
	}
	return nil
}

// focusAndCheckElement uses ChromeVox navigation (using Tab), to navigate to the next
// UI element (specified by node), and activates it (using Search + Space).
// Returns an error indicating the success of both actions.
func focusAndCheckElement(ctx context.Context, chromeVoxConn *chrome.Conn, node, expectedNode *accessibility.AutomationNode) error {
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "error with creating EventWriter from keyboard")
	}
	defer ew.Close()

	// Move focus to the next UI element.
	if err := ew.Accel(ctx, "Tab"); err != nil {
		return errors.Wrap(err, "Accel(Tab) returned error")
	}

	if err := accessibility.WaitForChromeVoxStopSpeaking(ctx, chromeVoxConn); err != nil {
		return errors.Wrap(err, "could not check if ChromeVox is speaking")
	}

	// Wait for element to receive focus.
	if err := accessibility.WaitForFocusedNode(ctx, chromeVoxConn, node); err != nil {
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
	if err := accessibility.WaitForFocusedNode(ctx, chromeVoxConn, expectedNode); err != nil {
		return errors.Wrap(err, "failed to check toggled state")
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

		for i, test := range []struct {
			action   func() error
			expected []eventLog
		}{
			{
				action: func() error {
					return focusAndCheckElement(ctx, chromeVoxConn,
						&accessibility.AutomationNode{
							ClassName: accessibility.ToggleButton,
							Checked:   "false",
						}, &accessibility.AutomationNode{
							ClassName: accessibility.ToggleButton,
							Checked:   "true",
						})
				},
				expected: []eventLog{
					eventLog{"focus", "OFF", appName},
					eventLog{"checkedStateChanged", "ON", appName},
				},
			}, {
				action: func() error {
					return focusAndCheckElement(ctx, chromeVoxConn,
						&accessibility.AutomationNode{
							ClassName: accessibility.CheckBox,
							Checked:   "false",
						},
						&accessibility.AutomationNode{
							ClassName: accessibility.CheckBox,
							Checked:   "true",
						})
				},
				expected: []eventLog{
					eventLog{"focus", "CheckBox", appName},
					eventLog{"checkedStateChanged", "CheckBox", appName},
				},
			}, {
				action: func() error {
					return focusAndIncrementElement(ctx, chromeVoxConn,
						&accessibility.AutomationNode{
							ClassName:     accessibility.SeekBar,
							ValueForRange: seekBarInitialValue,
						},
						&accessibility.AutomationNode{
							ClassName:     accessibility.SeekBar,
							ValueForRange: seekBarExpectedValue,
						})
				},
				expected: []eventLog{
					eventLog{"focus", "seekBar", appName},
					eventLog{"valueChanged", "seekBar", appName},
				},
			}, {
				action: func() error {
					return focusAndIncrementElement(ctx, chromeVoxConn,
						&accessibility.AutomationNode{
							ClassName:     accessibility.SeekBar,
							ValueForRange: seekBarDiscreteInitialValue,
						},
						&accessibility.AutomationNode{
							ClassName:     accessibility.SeekBar,
							ValueForRange: seekBarDiscreteExpectedValue,
						})
				},
				expected: []eventLog{
					eventLog{"focus", "seekBarDiscrete", appName},
					eventLog{"valueChanged", "seekBarDiscrete", appName},
				},
			},
		} {
			// Ensure that ChromeVox log is cleared before proceeding.
			if err := chromeVoxConn.Exec(ctx, "LogStore.instance.clearLog()"); err != nil {
				s.Fatal("Error with clearing ChromeVox Log: ", err)
			}

			if err := test.action(); err != nil {
				s.Fatal("Failed to run the test: ", err)
			}

			// Initial action sometimes invokes additional events (like focusing the entire application).
			// Latest logs should only be checked on the first iteration. (b/123397142#comment19)
			// TODO(b/142093176) Find the root cause.
			if err := verifyLogs(ctx, chromeVoxConn, test.expected, i == 0); err != nil {
				s.Fatal("Failed to verify the log: ", err)
			}
		}
	})
}
