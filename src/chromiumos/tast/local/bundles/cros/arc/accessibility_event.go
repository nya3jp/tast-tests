// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/a11y"
	arca11y "chromiumos/tast/local/bundles/cros/arc/a11y"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type axEventTestStep struct {
	keys      string           // a sequence of keys to invoke.
	focus     *nodewith.Finder // expected params of focused node after the ui.EventType
	eventType ui.EventType     // an expected event type from the focused node.
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
	uia := uiauto.New(tconn)
	testing.ContextLog(ctx, "Running test step: ", step.keys)
	testing.ContextLog(ctx, "Expecting node: ", step.focus.Pretty())
	watcher, err := ui.NewRootWatcher(ctx, tconn, step.eventType)
	if err != nil {
		return errors.Wrap(err, "failed to create EventWatcher")
	}
	defer watcher.Release(ctx)

	// Send a key event.
	if err := ew.Accel(ctx, step.keys); err != nil {
		return errors.Wrapf(err, "Accel(%s) returned error", step.keys)
	}

	// Wait for the focused element to match the expected.
	if err := cvconn.WaitForFocusedNode(ctx, tconn, step.focus, 10*time.Second); err != nil {
		return err
	}

	return testing.Poll(ctx, func(ctx context.Context) error {
		events, err := watcher.WaitForEvent(ctx, 10*time.Second)
		if err != nil {
			return err
		}
		defer events.Release(ctx)

		for _, e := range events {
			if e.Target == nil {
				continue
			}

			// Convert ui.Node into uiauto.NodeInfo so that the target node can be
			// matched against step.focus.
			target, err := nodeToNodeInfo(ctx, tconn, e.Target)
			if err != nil {
				return errors.Wrap(err, "failed to convert Node into NodeInfo")
			}

			err = uia.Matches(ctx, target, step.focus)
			if err != nil {
				return errors.Wrap(err, "nodes did not match")
			}

			return nil
		}
		return errors.Errorf("expected event didn't occur. got: %+v", events)
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

func nodeToNodeInfo(ctx context.Context, tconn *chrome.TestConn, node *ui.Node) (*uiauto.NodeInfo, error) {
	uia := uiauto.New(tconn)
	// 1. Create a finder with all properties in Node.
	finder := nodewith.Name(node.Name)
	// 2. Convert the finder into a NodeInfo
	return uia.Info(ctx, finder)
}

/*
// Manually compares a ui.Node and uiauto.NodeInfo, since there is no existing
// way to compare them. This is a temporary method, which should be removed
// once this file gets converted from chrome/ui to chrome/uiauto.
func compareNodeAndNodeInfo(ctx context.Context, expected *uiauto.NodeInfo, actual *ui.Node) error {
	if expected.Name != actual.Name {
		return errors.Errorf("Expected name: %s, but got: %s", expected.Name, actual.Name)
	}

	if string(expected.Role) != string(actual.Role) {
		return errors.Errorf("Expected role: %s, but got: %s", expected.Role, actual.Role)
	}

	if expected.Value != actual.Value {
		return errors.Errorf("Expected value: %s, but got: %s", expected.Value, actual.Value)
	}

	// Convert the two maps into map[string]bool for comparison.
	newExpected := make(map[string]bool)
	for key, value := range expected.State {
		newExpected[string(key)] = value
	}

	newActual := make(map[string]bool)
	for key, value := range actual.State {
		newActual[string(key)] = value
	}

	ok := reflect.DeepEqual(newExpected, newActual)
	if !ok {
		return errors.Errorf("Expected state: %s, but got: %s", newExpected, newActual)
	}

	if string(expected.Checked) != string(actual.Checked) {
		return errors.Errorf("Expected checked: %s, but got: %s", expected.Checked, actual.Checked)
	}

	if expected.ClassName != actual.ClassName {
		return errors.Errorf("Expected class name: %s, but got: %s", expected.ClassName, actual.ClassName)
	}

	if expected.Location != actual.Location {
		return errors.Errorf("Expected location: %s, but got: %s", expected.Location, actual.Location)
	}

	if string(expected.Restriction) != string(actual.Restriction) {
		return errors.Errorf("Expected restriction: %s, but got: %s", expected.Restriction, actual.Restriction)
	}

	ok = reflect.DeepEqual(expected.HTMLAttributes, actual.HTMLAttributes)
	if !ok {
		return errors.Errorf("Expected HTML attributes: %s, but got: %s", expected.HTMLAttributes, actual.HTMLAttributes)
	}

	return nil
}
*/

func AccessibilityEvent(ctx context.Context, s *testing.State) {
	MainActivityTestSteps := []axEventTestStep{
		axEventTestStep{
			"Tab",
			nodewith.Name("OFF").Role(role.ToggleButton).ClassName(arca11y.ToggleButton).Attribute("checked", "false"),
			ui.EventTypeFocus,
		},
		axEventTestStep{
			"Search+Space",
			nodewith.Name("ON").Role(role.ToggleButton).ClassName(arca11y.ToggleButton).Attribute("checked", "true"),
			ui.EventTypeCheckedStateChanged,
		},
		axEventTestStep{
			"Tab",
			nodewith.Name("CheckBox").Role(role.CheckBox).ClassName(arca11y.CheckBox).Attribute("checked", "false"),
			ui.EventTypeFocus,
		},
		axEventTestStep{
			"Search+Space",
			nodewith.Name("CheckBox").Role(role.CheckBox).ClassName(arca11y.CheckBox).Attribute("checked", "true"),
			ui.EventTypeCheckedStateChanged,
		},
		axEventTestStep{
			"Tab",
			nodewith.Name("CheckBoxWithStateDescription").Role(role.CheckBox).ClassName(arca11y.CheckBox).Attribute("checked", "false"),
			ui.EventTypeFocus,
		},
		axEventTestStep{
			"Tab",
			nodewith.Name("seekBar").Role(role.Slider).ClassName(arca11y.SeekBar).Attribute("valueForRange", 25),
			ui.EventTypeFocus,
		},
		axEventTestStep{
			"=",
			nodewith.Name("seekBar").Role(role.Slider).ClassName(arca11y.SeekBar).Attribute("valueForRange", 26),
			ui.EventTypeRangeValueChanged,
		},
		axEventTestStep{
			"Tab",
			nodewith.Role(role.Slider).ClassName(arca11y.SeekBar).Attribute("valueForRange", 3),
			ui.EventTypeFocus,
		},
		axEventTestStep{
			"-",
			nodewith.Role(role.Slider).ClassName(arca11y.SeekBar).Attribute("valueForRange", 2),
			ui.EventTypeRangeValueChanged,
		},
	}
	EditTextActivityTestSteps := []axEventTestStep{
		axEventTestStep{
			"Tab",
			nodewith.Name("contentDescription").Role(role.TextField).ClassName(arca11y.EditText),
			ui.EventTypeFocus,
		},
		axEventTestStep{
			"a",
			nodewith.Name("contentDescription").Role(role.TextField).ClassName(arca11y.EditText).Attribute("value", "a"),
			ui.EventTypeValueInTextFieldChanged,
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
