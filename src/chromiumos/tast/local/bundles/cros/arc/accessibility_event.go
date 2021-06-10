// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/action"
	arca11y "chromiumos/tast/local/bundles/cros/arc/a11y"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type axEventTestStep struct {
	keys  string           // a sequence of keys to invoke.
	focus *nodewith.Finder // expected params of focused node after the event.
	event event.Event      // an expected event type from the focused node.
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

func runTestStep(ctx context.Context, cvconn *a11y.ChromeVoxConn, tconn *chrome.TestConn, ew *input.KeyboardEventWriter, step axEventTestStep, isFirstStep bool) error {
	sendKeyEvent := func() action.Action {
		return func(ctx context.Context) error {
			return ew.Accel(ctx, step.keys)
		}
	}

	waitForFocusedNode := func() action.Action {
		return func(ctx context.Context) error {
			return cvconn.WaitForFocusedNode(ctx, tconn, step.focus, 10*time.Second)
		}
	}

	// 1. Send a key event.
	// 2. Wait for the event to fire on the node.
	// 3. Wait for ChromeVox focus to change to the correct node.
	ui := uiauto.New(tconn)
	if err := ui.IfSuccessThen(ui.WithTimeout(10*time.Second).WaitForEvent(step.focus, step.event, sendKeyEvent()), waitForFocusedNode())(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for event on node")
	}

	return nil
}

func AccessibilityEvent(ctx context.Context, s *testing.State) {
	MainActivityTestSteps := []axEventTestStep{
		axEventTestStep{
			"Tab",
			nodewith.Name("OFF").Role(role.ToggleButton).ClassName(arca11y.ToggleButton).Attribute("checked", "false"),
			event.Focus,
		},
		axEventTestStep{
			"Search+Space",
			nodewith.Name("ON").Role(role.ToggleButton).ClassName(arca11y.ToggleButton).Attribute("checked", "true"),
			event.CheckedStateChanged,
		},
		axEventTestStep{
			"Tab",
			nodewith.Name("CheckBox").Role(role.CheckBox).ClassName(arca11y.CheckBox).Attribute("checked", "false"),
			event.Focus,
		},
		axEventTestStep{
			"Search+Space",
			nodewith.Name("CheckBox").Role(role.CheckBox).ClassName(arca11y.CheckBox).Attribute("checked", "true"),
			event.CheckedStateChanged,
		},
		axEventTestStep{
			"Tab",
			nodewith.Name("CheckBoxWithStateDescription").Role(role.CheckBox).ClassName(arca11y.CheckBox).Attribute("checked", "false"),
			event.Focus,
		},
		axEventTestStep{
			"Tab",
			nodewith.Name("seekBar").Role(role.Slider).ClassName(arca11y.SeekBar).Attribute("valueForRange", 25),
			event.Focus,
		},
		axEventTestStep{
			"=",
			nodewith.Name("seekBar").Role(role.Slider).ClassName(arca11y.SeekBar).Attribute("valueForRange", 26),
			event.RangeValueChanged,
		},
		axEventTestStep{
			"Tab",
			nodewith.Role(role.Slider).ClassName(arca11y.SeekBar).Attribute("valueForRange", 3),
			event.Focus,
		},
		axEventTestStep{
			"-",
			nodewith.Role(role.Slider).ClassName(arca11y.SeekBar).Attribute("valueForRange", 2),
			event.RangeValueChanged,
		},
	}
	EditTextActivityTestSteps := []axEventTestStep{
		axEventTestStep{
			"Tab",
			nodewith.Name("contentDescription").Role(role.TextField).ClassName(arca11y.EditText),
			event.Focus,
		},
		axEventTestStep{
			"a",
			nodewith.Name("contentDescription").Role(role.TextField).ClassName(arca11y.EditText).Attribute("value", "a"),
			event.ValueInTextFieldChanged,
		},
	}
	testActivities := []arca11y.TestActivity{arca11y.MainActivity, arca11y.EditTextActivity}

	testSteps := map[arca11y.TestActivity][]axEventTestStep{
		arca11y.MainActivity:     MainActivityTestSteps,
		arca11y.EditTextActivity: EditTextActivityTestSteps,
	}

	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error with creating EventWriter from keyboard: ", err)
	}
	defer ew.Close()

	testFunc := func(ctx context.Context, cvconn *a11y.ChromeVoxConn, tconn *chrome.TestConn, currentActivity arca11y.TestActivity) error {
		for i, test := range testSteps[currentActivity] {
			if err := runTestStep(ctx, cvconn, tconn, ew, test, i == 0); err != nil {
				return errors.Wrapf(err, "failed to run a test step %+v", test)
			}
		}
		return nil
	}
	arca11y.RunTest(ctx, s, testActivities, testFunc)
}
