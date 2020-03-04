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
	"chromiumos/tast/local/chrome/ui"
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
	Key    string        // key events to invoke the event.
	Params ui.FindParams // expected params of focused node after the event.
	Event  eventLog      // expected event log.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilityEvent,
		Desc:         "Checks accessibility events in Chrome are as expected with ARC enabled",
		Contacts:     []string{"sarakato@chromium.org", "dtseng@chromium.org", "hirokisato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{accessibility.ApkName},
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
	})
}

// verifyLog gets the current ChromeVox log and checks that it matches with expected log.
// Note that as the initial a11y focus is unstable, checkOnlyLatest=true can be used to check only the latest log.
func verifyLog(ctx context.Context, cvConn *chrome.Conn, expectedLog eventLog, checkOnlyLatest bool) error {
	var logs []eventLog
	if err := cvConn.Eval(ctx, "LogStore.instance.getLogsOfType(LogStore.LogType.EVENT)", &logs); err != nil {
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

func runTestStep(ctx context.Context, cvConn *chrome.Conn, tconn *chrome.TestConn, ew *input.KeyboardEventWriter, test testStep, isFirstStep bool) error {
	// Ensure that ChromeVox log is cleared before proceeding.
	if err := cvConn.Exec(ctx, "LogStore.instance.clearLog()"); err != nil {
		return errors.Wrap(err, "error with clearing ChromeVox log")
	}

	// Send a key event.
	if err := ew.Accel(ctx, test.Key); err != nil {
		return errors.Wrapf(err, "Accel(%s) returned error", test.Key)
	}

	// Wait for the focused element to match the expected.
	if err := accessibility.WaitForFocusedNode(ctx, cvConn, tconn, &test.Params); err != nil {
		return err
	}

	// Initial action sometimes invokes additional events (like focusing the entire application).
	// Latest logs should only be checked on the first iteration. (b/123397142#comment19)
	// TODO(b/142093176) Find the root cause.
	if err := verifyLog(ctx, cvConn, test.Event, isFirstStep); err != nil {
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

	accessibility.RunTest(ctx, s, func(ctx context.Context, a *arc.ARC, cvConn *chrome.Conn, tconn *chrome.TestConn, ew *input.KeyboardEventWriter) error {
		// Set up event stream logging for accessibility events.
		if err := cvConn.EvalPromise(ctx, `
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
			return errors.Wrap(err, "enabling event stream logging failed")
		}

		for i, test := range []testStep{
			// Move focus to ToggleButton and toggle it.
			{
				"Tab",
				ui.FindParams{
					ClassName: accessibility.ToggleButton,
					Name:      "OFF",
					Role:      ui.RoleTypeToggleButton,
					Attributes: map[string]interface{}{
						"checked": "false",
						"tooltip": "button tooltip",
					},
				},
				eventLog{"focus", "OFF", appName},
			}, {
				"Search+Space",
				ui.FindParams{
					ClassName: accessibility.ToggleButton,
					Name:      "ON",
					Role:      ui.RoleTypeToggleButton,
					Attributes: map[string]interface{}{
						"checked": "true",
						"tooltip": "button tooltip",
					},
				},
				eventLog{"checkedStateChanged", "ON", appName},
			},
			// Move focus to CheckBox and check it.
			{
				"Tab",
				ui.FindParams{
					ClassName: accessibility.CheckBox,
					Name:      "CheckBox",
					Role:      ui.RoleTypeCheckBox,
					Attributes: map[string]interface{}{
						"checked": "false",
						"tooltip": "checkbox tooltip",
					},
				},
				eventLog{"focus", "CheckBox", appName},
			}, {
				"Search+Space",
				ui.FindParams{
					ClassName: accessibility.CheckBox,
					Name:      "CheckBox",
					Role:      ui.RoleTypeCheckBox,
					Attributes: map[string]interface{}{
						"checked": "true",
						"tooltip": "checkbox tooltip",
					},
				},
				eventLog{"checkedStateChanged", "CheckBox", appName},
			},
			// Move focus to SeekBar and increment it.
			{
				"Tab",
				ui.FindParams{
					ClassName: accessibility.SeekBar,
					Name:      "seekBar",
					Role:      ui.RoleTypeSlider,
					Attributes: map[string]interface{}{
						"valueForRange": seekBarInitialValue,
					},
				},
				eventLog{"focus", "seekBar", appName},
			}, {
				"=",
				ui.FindParams{
					ClassName: accessibility.SeekBar,
					Name:      "seekBar",
					Role:      ui.RoleTypeSlider,
					Attributes: map[string]interface{}{
						"valueForRange": seekBarInitialValue + 1,
					},
				},
				eventLog{"valueChanged", "seekBar", appName},
			},
			// Move focus to SeekbarDiscrete and decrement it.
			{
				"Tab",
				ui.FindParams{
					ClassName: accessibility.SeekBar,
					Name:      "seekBarDiscrete",
					Role:      ui.RoleTypeSlider,
					Attributes: map[string]interface{}{
						"valueForRange": seekBarDiscreteInitialValue,
					},
				},
				eventLog{"focus", "seekBarDiscrete", appName},
			}, {
				"-",
				ui.FindParams{
					ClassName: accessibility.SeekBar,
					Name:      "seekBarDiscrete",
					Role:      ui.RoleTypeSlider,
					Attributes: map[string]interface{}{
						"valueForRange": seekBarDiscreteInitialValue - 1,
					},
				},
				eventLog{"valueChanged", "seekBarDiscrete", appName},
			},
		} {
			if err := runTestStep(ctx, cvConn, tconn, ew, test, i == 0); err != nil {
				return errors.Wrapf(err, "failed to run a test step %v", test)
			}
		}
		return nil
	})
}
