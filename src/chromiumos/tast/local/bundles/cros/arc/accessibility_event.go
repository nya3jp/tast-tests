// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"reflect"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arc/accessibility"
	"chromiumos/tast/local/chrome"
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
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_both", "chrome"},
		Data:         []string{"ArcAccessibilityTest.apk"},
		Timeout:      4 * time.Minute,
	})
}

// verifyLogs gets the current ChromeVox log and checks that it matches with expected log.
func verifyLogs(ctx context.Context, chromeVoxConn *chrome.Conn, expectedLogs []eventLog) error {
	var logs []eventLog
	if err := chromeVoxConn.Eval(ctx, "LogStore.instance.getLogsOfType(LogStore.LogType.EVENT)", &logs); err != nil {
		return errors.Wrap(err, "failed to get event logs")
	}

	if !reflect.DeepEqual(logs, expectedLogs) {
		return errors.Errorf("event output is not as expected: got %q; want %q", logs, expectedLogs)
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
	cr, err := accessibility.NewChrome(ctx)
	if err != nil {
		s.Fatal(err) // NOLINT: arc/ui returns loggable errors
	}
	defer cr.Close(ctx)

	a, err := accessibility.NewARC(ctx, s.OutDir())
	if err != nil {
		s.Fatal(err) // NOLINT: arc/ui returns loggable errors
	}
	defer a.Close()

	if err := accessibility.InstallAndStartSampleApp(ctx, a, s.DataPath(apkName)); err != nil {
		s.Fatal("Setting up ARC environment with accessibility failed: ", err)
	}

	if err := accessibility.EnableSpokenFeedback(ctx, cr, a); err != nil {
		s.Fatal(err) // NOLINT: arc/ui returns loggable errors
	}

	chromeVoxConn, err := accessibility.ChromeVoxExtConn(ctx, cr)
	if err != nil {
		s.Fatal("Creating connection to ChromeVox extension failed: ", err)
	}
	defer chromeVoxConn.Close()

	if err := accessibility.WaitForChromeVoxReady(ctx, chromeVoxConn); err != nil {
		s.Fatal("Could not wait for ChromeVox to be ready: ", err)
	}

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

	expectedEventLog := func(eventType, targetName string) eventLog {
		return eventLog{
			EventType:  eventType,
			TargetName: targetName,
			RootName:   appName,
		}
	}

	// Each test case focus to elementClass, and interact with it (using search+space).
	// The result of the interaction (checked or value changed), will be determined by
	// elementClass.
	for _, test := range []struct {
		node         *accessibility.AutomationNode
		expectedNode *accessibility.AutomationNode
		wantLogs     []eventLog
	}{
		{
			node: &accessibility.AutomationNode{
				ClassName: toggleButton,
				Tooltip:   "button tooltip",
				Checked:   "false",
			},
			expectedNode: &accessibility.AutomationNode{
				ClassName: toggleButton,
				Tooltip:   "button tooltip",
				Checked:   "true",
			},
			wantLogs: []eventLog{
				expectedEventLog("focus", "OFF"),
				expectedEventLog("checkedStateChanged", "ON"),
			},
		},
		{
			node: &accessibility.AutomationNode{
				ClassName: checkBox,
				Tooltip:   "checkbox tooltip",
				Checked:   "false",
			},
			expectedNode: &accessibility.AutomationNode{
				ClassName: checkBox,
				Tooltip:   "checkbox tooltip",
				Checked:   "true",
			},
			wantLogs: []eventLog{
				expectedEventLog("focus", "CheckBox"),
				expectedEventLog("checkedStateChanged", "CheckBox"),
			},
		},
		{
			node: &accessibility.AutomationNode{
				ClassName:     seekBar,
				ValueForRange: seekBarInitialValue,
			},
			expectedNode: &accessibility.AutomationNode{
				ClassName:     seekBar,
				ValueForRange: seekBarExpectedValue,
			},
			wantLogs: []eventLog{
				expectedEventLog("focus", "seekBar"),
				expectedEventLog("valueChanged", "seekBar"),
			},
		},
		{
			node: &accessibility.AutomationNode{
				ClassName:     seekBar,
				ValueForRange: seekBarDiscreteInitialValue,
			},
			expectedNode: &accessibility.AutomationNode{
				ClassName:     seekBar,
				ValueForRange: seekBarDiscreteExpectedValue,
			},
			wantLogs: []eventLog{
				expectedEventLog("focus", "seekBarDiscrete"),
				expectedEventLog("valueChanged", "seekBarDiscrete"),
			},
		},
	} {
		if err := accessibility.SendKeystroke(ctx, chromeVoxConn, []string{"Tab", "Search+Space"}, func() error {
			if err := accessibility.WaitForFocusedNode(ctx, chromeVoxConn, test.node); err != nil {
				return err
			}
			return nil
		}); err != nil {
			s.Fatal("timed out polling for element", err)
		}
	}
}
