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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type axEventParams struct {
	target    *ui.FindParams // expected node which the event was targeted. When it's nil, the focused node is used.
	eventType ui.EventType
}

type axEventTestStep struct {
	keys  string        // key events to invoke the event.
	focus ui.FindParams // expected params of focused node after the event.
	event axEventParams // expected event log.
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
	// If the event tareget not specified, assume it's the same as the focused node.
	if step.event.target == nil {
		step.event.target = &step.focus
	}

	watcher, err := ui.NewRootWatcher(ctx, tconn, step.event.eventType)
	if err != nil {
		return errors.Wrap(err, "failed to create EventWatcher")
	}
	defer watcher.Release(ctx)

	// Send a key event.
	if err := ew.Accel(ctx, step.keys); err != nil {
		return errors.Wrapf(err, "Accel(%s) returned error", step.keys)
	}

	// Wait for the focused element to match the expected.
	if err := cvconn.WaitForFocusedNode(ctx, tconn, &step.focus, 10*time.Second); err != nil {
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
			if ok, err := e.Target.Matches(ctx, *step.event.target); err != nil {
				return err
			} else if ok {
				return nil
			}
		}
		return errors.Errorf("expected event didn't occur. got: %+v", events)
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

func AccessibilityEvent(ctx context.Context, s *testing.State) {
	MainActivityTestSteps := []axEventTestStep{
		axEventTestStep{
			"Tab",
			ui.FindParams{
				ClassName: arca11y.ToggleButton,
				Name:      "OFF",
				Role:      ui.RoleTypeToggleButton,
				Attributes: map[string]interface{}{
					"checked": ui.CheckedStateFalse,
				},
			},
			axEventParams{
				eventType: ui.EventTypeFocus,
			},
		},
		axEventTestStep{
			"Search+Space",
			ui.FindParams{
				ClassName: arca11y.ToggleButton,
				Name:      "ON",
				Role:      ui.RoleTypeToggleButton,
				Attributes: map[string]interface{}{
					"checked": ui.CheckedStateTrue,
				},
			},
			axEventParams{
				eventType: ui.EventTypeCheckedStateChanged,
			},
		},
		axEventTestStep{
			"Tab",
			ui.FindParams{
				ClassName: arca11y.CheckBox,
				Name:      "CheckBox",
				Role:      ui.RoleTypeCheckBox,
				Attributes: map[string]interface{}{
					"checked": ui.CheckedStateFalse,
				},
			},
			axEventParams{
				eventType: ui.EventTypeFocus,
			},
		},
		axEventTestStep{
			"Search+Space",
			ui.FindParams{
				ClassName: arca11y.CheckBox,
				Name:      "CheckBox",
				Role:      ui.RoleTypeCheckBox,
				Attributes: map[string]interface{}{
					"checked": ui.CheckedStateTrue,
				},
			},
			axEventParams{
				eventType: ui.EventTypeCheckedStateChanged,
			},
		},
		axEventTestStep{
			"Tab",
			ui.FindParams{
				ClassName: arca11y.CheckBox,
				Name:      "CheckBoxWithStateDescription",
				Role:      ui.RoleTypeCheckBox,
				Attributes: map[string]interface{}{
					"checked": ui.CheckedStateFalse,
				},
			},
			axEventParams{
				eventType: ui.EventTypeFocus,
			},
		},
		axEventTestStep{
			"Tab",
			ui.FindParams{
				ClassName: arca11y.SeekBar,
				Name:      "seekBar",
				Role:      ui.RoleTypeSlider,
				Attributes: map[string]interface{}{
					"valueForRange": 25,
				},
			},
			axEventParams{
				eventType: ui.EventTypeFocus,
			},
		},
		axEventTestStep{
			"=",
			ui.FindParams{
				ClassName: arca11y.SeekBar,
				Name:      "seekBar",
				Role:      ui.RoleTypeSlider,
				Attributes: map[string]interface{}{
					"valueForRange": 26,
				},
			},
			axEventParams{
				eventType: ui.EventTypeRangeValueChanged,
			},
		},
		axEventTestStep{
			"Tab",
			ui.FindParams{
				ClassName: arca11y.SeekBar,
				Role:      ui.RoleTypeSlider,
				Attributes: map[string]interface{}{
					"valueForRange": 3,
				},
			},
			axEventParams{
				eventType: ui.EventTypeFocus,
			},
		},
		axEventTestStep{
			"-",
			ui.FindParams{
				ClassName: arca11y.SeekBar,
				Role:      ui.RoleTypeSlider,
				Attributes: map[string]interface{}{
					"valueForRange": 2,
				},
			},
			axEventParams{
				eventType: ui.EventTypeRangeValueChanged,
			},
		},
	}
	EditTextActivityTestSteps := []axEventTestStep{
		axEventTestStep{
			"Tab",
			ui.FindParams{
				ClassName: arca11y.EditText,
				Name:      "contentDescription",
				Role:      ui.RoleTypeTextField,
			},
			axEventParams{
				eventType: ui.EventTypeFocus,
			},
		},
		axEventTestStep{
			"a",
			ui.FindParams{
				ClassName: arca11y.EditText,
				Name:      "contentDescription",
				Role:      ui.RoleTypeTextField,
				Attributes: map[string]interface{}{
					"value": "a",
				},
			},
			axEventParams{
				eventType: ui.EventTypeValueInTextFieldChanged,
			},
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
