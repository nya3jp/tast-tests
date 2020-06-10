// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"os"
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

const axEventFilePrefix = "accessibility_event"

// axEventLog represents a log of accessibility event.
// Defined in https://cs.chromium.org/chromium/src/chrome/browser/resources/chromeos/accessibility/chromevox/background/logging/log_types.js
type axEventLog struct {
	EventType  string `json:"type_"`
	TargetName string `json:"targetName_"`
	RootName   string `json:"rootName_"`
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
		Contacts:     []string{"sarakato@chromium.org", "dtseng@chromium.org", "hirokisato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
		Data:         []string{"accessibility_event.MainActivity.json", "accessibility_event.EditTextActivity.json"},
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
func verifyLog(ctx context.Context, cvconn *chrome.Conn, expectedLog axEventLog, checkOnlyLatest bool) error {
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

func runTestStep(ctx context.Context, cvconn *chrome.Conn, tconn *chrome.TestConn, ew *input.KeyboardEventWriter, test axEventTestStep, isFirstStep bool) error {
	// Ensure that ChromeVox log is cleared before proceeding.
	if err := cvconn.Eval(ctx, "LogStore.instance.clearLog()", nil); err != nil {
		return errors.Wrap(err, "error with clearing ChromeVox log")
	}

	// Send a key event.
	if err := ew.Accel(ctx, test.Key); err != nil {
		return errors.Wrapf(err, "Accel(%s) returned error", test.Key)
	}

	// Wait for the focused element to match the expected.
	if err := accessibility.WaitForFocusedNode(ctx, cvconn, tconn, &test.Params, 10*time.Second); err != nil {
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

// getEventTestSteps returns a slice of axEventTestStep, which is read from the specific file.
func getEventTestSteps(filepath string) ([]axEventTestStep, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var steps []axEventTestStep
	err = json.NewDecoder(f).Decode(&steps)
	return steps, err
}

func setupEventStreamLogging(ctx context.Context, cvconn *chrome.Conn, activityName string, axEventTestSteps []axEventTestStep) error {
	eventsSeen := make(map[string]bool)
	var events []string
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
		  EventStreamLogger.instance.notifyEventStreamFilterChangedAll(false);
		  for (const event of events) {
		    EventStreamLogger.instance.notifyEventStreamFilterChanged(event, true);
		  }
		}`, events); err != nil {
		return errors.Wrap(err, "enabling event stream logging failed")
	}
	return nil
}

func AccessibilityEvent(ctx context.Context, s *testing.State) {
	testActivities := []accessibility.TestActivity{accessibility.MainActivity, accessibility.EditTextActivity}

	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error with creating EventWriter from keyboard: ", err)
	}
	defer ew.Close()

	testFunc := func(ctx context.Context, cvconn *chrome.Conn, tconn *chrome.TestConn, currentActivity accessibility.TestActivity) error {
		testSteps, err := getEventTestSteps(s.DataPath(axEventFilePrefix + currentActivity.Name + ".json"))
		if err != nil {
			return errors.Wrap(err, "error reading from JSON")
		}
		if err := setupEventStreamLogging(ctx, cvconn, currentActivity.Name, testSteps); err != nil {
			return err
		}
		for i, test := range testSteps {
			test.Event.RootName = currentActivity.Title
			if err := runTestStep(ctx, cvconn, tconn, ew, test, i == 0); err != nil {
				return errors.Wrapf(err, "failed to run a test step %v", test)
			}
		}
		return nil
	}
	accessibility.RunTest(ctx, s, testActivities, testFunc)
}
