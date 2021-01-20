// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"reflect"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/a11y"
	arca11y "chromiumos/tast/local/bundles/cros/arc/a11y"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const axEventFilePrefix = "accessibility_event"

// axEventLog represents a log of accessibility event.
// Defined in https://cs.chromium.org/chromium/src/chrome/browser/resources/chromeos/accessibility/chromevox/background/logging/log_types.js
// TODO(b/159413215): Use automationEvent instead of axEventLog.
type axEventLog struct {
	EventType  ui.EventType `json:"type_"`
	TargetName string       `json:"targetName_"`
	RootName   string       `json:"rootName_"`
	// There is also docUrl property, but it is not used in test.
}

type axEventTestStep struct {
	Key    string        // key events to invoke the event.
	Params ui.FindParams // expected params of focused node after the event.
	Event  axEventLog    // expected event log.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilityEvent,
		Desc:         "Checks accessibility events in Chrome are as expected with ARC enabled",
		Contacts:     []string{"sarakato@chromium.org", "dtseng@chromium.org", "hirokisato@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

// verifyLog gets the current ChromeVox log and checks that it matches with expected log.
// Note that as the initial a11y focus is unstable, checkOnlyLatest=true can be used to check only the latest log.
func verifyLog(ctx context.Context, cvconn *a11y.ChromeVoxConn, expectedLog axEventLog, checkOnlyLatest bool) error {
	var logs []axEventLog
	if err := cvconn.Eval(ctx, "LogStore.instance.getLogsOfType(LogStore.LogType.EVENT)", &logs); err != nil {
		return errors.Wrap(err, "failed to get event logs")
	}

	// Filter out event logs from unrelated windows.
	i := 0
	for _, log := range logs {
		if log.RootName == expectedLog.RootName {
			logs[i] = log
			i++
		}
	}
	logs = logs[:i]
	if checkOnlyLatest && len(logs) > 1 {
		logs = logs[len(logs)-1:]
	}

	if len(logs) != 1 || !reflect.DeepEqual(logs[0], expectedLog) {
		return errors.Errorf("event output is not as expected: got %q; want %q", logs, expectedLog)
	}
	return nil
}

func runTestStep(ctx context.Context, cvconn *a11y.ChromeVoxConn, tconn *chrome.TestConn, ew *input.KeyboardEventWriter, test axEventTestStep, isFirstStep bool) error {
	// Ensure that ChromeVox log is cleared before proceeding.
	if err := cvconn.Eval(ctx, "LogStore.instance.clearLog()", nil); err != nil {
		return errors.Wrap(err, "error with clearing ChromeVox log")
	}

	// Send a key event.
	if err := ew.Accel(ctx, test.Key); err != nil {
		return errors.Wrapf(err, "Accel(%s) returned error", test.Key)
	}

	// Wait for the focused element to match the expected.
	if err := cvconn.WaitForFocusedNode(ctx, tconn, &test.Params, 10*time.Second); err != nil {
		return err
	}

	// Initial action sometimes invokes additional events (like focusing the entire application).
	// Latest logs should only be checked on the first iteration. (b/123397142#comment19)
	// TODO(b/142093176) Find the root cause.
	if err := verifyLog(ctx, cvconn, test.Event, isFirstStep); err != nil {
		return errors.Wrap(err, "failed to verify the log")
	}

	return nil
}

func setupEventStreamLogging(ctx context.Context, cvconn *a11y.ChromeVoxConn, activityName string, axEventTestSteps []axEventTestStep) (func(context.Context, *a11y.ChromeVoxConn) error, error) {
	eventsSeen := make(map[ui.EventType]bool)
	var events []ui.EventType
	for _, test := range axEventTestSteps {
		currentEvent := test.Event.EventType
		if _, ok := eventsSeen[currentEvent]; !ok {
			eventsSeen[currentEvent] = true
			events = append(events, currentEvent)
		}
	}

	if err := cvconn.Call(ctx, nil, `async (events) => {
		  let desktop = await tast.promisify(chrome.automation.getDesktop)();
		  EventStreamLogger.instance = new EventStreamLogger(desktop);
		  for (const event of events) {
		    EventStreamLogger.instance.notifyEventStreamFilterChanged(event, true);
		  }
		}`, events); err != nil {
		return nil, errors.Wrap(err, "enabling event stream logging failed")
	}

	cleanup := func(ctx context.Context, cvconn *a11y.ChromeVoxConn) error {
		return cvconn.Call(ctx, nil, `async (events) => {
		  for (const event of events) {
		    EventStreamLogger.instance.notifyEventStreamFilterChanged(event, false);
		  }
		}`, events)
	}
	return cleanup, nil
}

func AccessibilityEvent(ctx context.Context, s *testing.State) {
	MainActivityTestSteps := []axEventTestStep{
		axEventTestStep{
			Key: "Tab",
			Params: ui.FindParams{
				ClassName: arca11y.ToggleButton,
				Name:      "OFF",
				Role:      ui.RoleTypeToggleButton,
				Attributes: map[string]interface{}{
					"checked": ui.CheckedStateFalse,
				},
			},
			Event: axEventLog{
				EventType:  ui.EventTypeFocus,
				TargetName: "OFF",
			},
		},
		axEventTestStep{
			Key: "Search+Space",
			Params: ui.FindParams{
				ClassName: arca11y.ToggleButton,
				Name:      "ON",
				Role:      ui.RoleTypeToggleButton,
				Attributes: map[string]interface{}{
					"checked": ui.CheckedStateTrue,
				},
			},
			Event: axEventLog{
				EventType:  ui.EventTypeCheckedStateChanged,
				TargetName: "ON",
			},
		},
		axEventTestStep{
			Key: "Tab",
			Params: ui.FindParams{
				ClassName: arca11y.CheckBox,
				Name:      "CheckBox",
				Role:      ui.RoleTypeCheckBox,
				Attributes: map[string]interface{}{
					"checked": ui.CheckedStateFalse,
				},
			},
			Event: axEventLog{
				EventType:  ui.EventTypeFocus,
				TargetName: "CheckBox",
			},
		},
		axEventTestStep{
			Key: "Search+Space",
			Params: ui.FindParams{
				ClassName: arca11y.CheckBox,
				Name:      "CheckBox",
				Role:      ui.RoleTypeCheckBox,
				Attributes: map[string]interface{}{
					"checked": ui.CheckedStateTrue,
				},
			},
			Event: axEventLog{
				EventType:  ui.EventTypeCheckedStateChanged,
				TargetName: "CheckBox",
			},
		},
		axEventTestStep{
			Key: "Tab",
			Params: ui.FindParams{
				ClassName: arca11y.CheckBox,
				Name:      "CheckBoxWithStateDescription",
				Role:      ui.RoleTypeCheckBox,
				Attributes: map[string]interface{}{
					"checked": ui.CheckedStateFalse,
				},
			},
			Event: axEventLog{
				EventType:  ui.EventTypeFocus,
				TargetName: "CheckBoxWithStateDescription",
			},
		},
		axEventTestStep{
			Key: "Tab",
			Params: ui.FindParams{
				ClassName: arca11y.SeekBar,
				Name:      "seekBar",
				Role:      ui.RoleTypeSlider,
				Attributes: map[string]interface{}{
					"valueForRange": 25,
				},
			},
			Event: axEventLog{
				EventType:  ui.EventTypeFocus,
				TargetName: "seekBar",
			},
		},
		axEventTestStep{
			Key: "=",
			Params: ui.FindParams{
				ClassName: arca11y.SeekBar,
				Name:      "seekBar",
				Role:      ui.RoleTypeSlider,
				Attributes: map[string]interface{}{
					"valueForRange": 26,
				},
			},
			Event: axEventLog{
				EventType:  ui.EventTypeRangeValueChanged,
				TargetName: "seekBar",
			},
		},
		axEventTestStep{
			Key: "Tab",
			Params: ui.FindParams{
				ClassName: arca11y.SeekBar,
				Role:      ui.RoleTypeSlider,
				Attributes: map[string]interface{}{
					"valueForRange": 3,
				},
			},
			Event: axEventLog{
				EventType: ui.EventTypeFocus,
			},
		},
		axEventTestStep{
			Key: "-",
			Params: ui.FindParams{
				ClassName: arca11y.SeekBar,
				Role:      ui.RoleTypeSlider,
				Attributes: map[string]interface{}{
					"valueForRange": 2,
				},
			},
			Event: axEventLog{
				EventType: ui.EventTypeRangeValueChanged,
			},
		},
	}
	EditTextActivityTestSteps := []axEventTestStep{
		axEventTestStep{
			Key: "Tab",
			Params: ui.FindParams{
				ClassName: arca11y.EditText,
				Name:      "contentDescription",
				Role:      ui.RoleTypeTextField,
			},
			Event: axEventLog{
				EventType:  ui.EventTypeFocus,
				TargetName: "contentDescription",
			},
		},
		axEventTestStep{
			Key: "a",
			Params: ui.FindParams{
				ClassName: arca11y.EditText,
				Name:      "contentDescription",
				Role:      ui.RoleTypeTextField,
				Attributes: map[string]interface{}{
					"value": "a",
				},
			},
			Event: axEventLog{
				EventType:  ui.EventTypeValueInTextFieldChanged,
				TargetName: "contentDescription",
			},
		},
	}
	testActivities := []arca11y.TestActivity{arca11y.MainActivity, arca11y.EditTextActivity}
	events := make(map[string][]axEventTestStep)
	events[arca11y.MainActivity.Name] = MainActivityTestSteps
	events[arca11y.EditTextActivity.Name] = EditTextActivityTestSteps
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error with creating EventWriter from keyboard: ", err)
	}
	defer ew.Close()

	testFunc := func(ctx context.Context, cvconn *a11y.ChromeVoxConn, tconn *chrome.TestConn, currentActivity arca11y.TestActivity) error {
		testSteps := events[currentActivity.Name]
		cleanup, err := setupEventStreamLogging(ctx, cvconn, currentActivity.Name, testSteps)
		if err != nil {
			return err
		}
		defer func() {
			if err := cleanup(ctx, cvconn); err != nil {
				testing.ContextLog(ctx, "Failed to clean up event stream linsteners: ", err)
			}
		}()

		for i, test := range testSteps {
			test.Event.RootName = currentActivity.Title
			if err := runTestStep(ctx, cvconn, tconn, ew, test, i == 0); err != nil {
				return errors.Wrapf(err, "failed to run a test step %v", test)
			}
		}
		return nil
	}
	arca11y.RunTest(ctx, s, testActivities, testFunc)
}
