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

// TODO(b/190773660): Migrate this file from chrome/ui to chrome/uiauto.
type axEventTestStep struct {
	keys      string        // a sequence of keys to invoke.
	focus     ui.FindParams // expected params of focused node after the event.
	eventType ui.EventType  // an expected event type from the focused node.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilityEvent,
		Desc:         "Checks accessibility events in Chrome are as expected with ARC enabled",
		Contacts:     []string{"sarakato@chromium.org", "dtseng@chromium.org", "hirokisato@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBootedWithoutUIAutomator",
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
	watcher, err := ui.NewRootWatcher(ctx, tconn, step.eventType)
	if err != nil {
		return errors.Wrap(err, "failed to create EventWatcher")
	}
	defer watcher.Release(ctx)

	// Send a key event.
	if err := ew.Accel(ctx, step.keys); err != nil {
		return errors.Wrapf(err, "Accel(%s) returned error", step.keys)
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
			if ok, err := e.Target.Matches(ctx, step.focus); err != nil {
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
			ui.EventTypeFocus,
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
			ui.EventTypeCheckedStateChanged,
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
			ui.EventTypeFocus,
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
			ui.EventTypeCheckedStateChanged,
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
			ui.EventTypeFocus,
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
			ui.EventTypeFocus,
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
			ui.EventTypeRangeValueChanged,
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
			ui.EventTypeFocus,
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
			ui.EventTypeRangeValueChanged,
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
			ui.EventTypeFocus,
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
