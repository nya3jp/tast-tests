// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/a11y"
	arca11y "chromiumos/tast/local/bundles/cros/arc/a11y"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type axEventTestStep struct {
	keys      string          // a sequence of keys to invoke.
	target    a11y.FindParams // expected params of the event target.
	eventType event.Event     // an expected event type from the focused node.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilityEvent,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks accessibility events in Chrome are as expected with ARC enabled",
		Contacts:     []string{"hirokisato@chromium.org", "dtseng@chromium.org", "arc-framework+tast@google.com"},
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
	watcher, err := a11y.NewRootWatcher(ctx, tconn, step.eventType)
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
			if ok, err := e.Target.Matches(ctx, step.target); err != nil {
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
		{
			"Tab",
			a11y.FindParams{
				Name: "OFF",
				Role: role.ToggleButton,
				Attributes: map[string]interface{}{
					"className": arca11y.ToggleButton,
					"checked":   checked.False,
				},
			},
			event.Focus,
		},
		{
			"Search+Space",
			a11y.FindParams{
				Name: "ON",
				Role: role.ToggleButton,
				Attributes: map[string]interface{}{
					"className": arca11y.ToggleButton,
					"checked":   checked.True,
				},
			},
			event.CheckedStateChanged,
		},
		{
			"Tab",
			a11y.FindParams{
				Name: "CheckBox",
				Role: role.CheckBox,
				Attributes: map[string]interface{}{
					"className": arca11y.CheckBox,
					"checked":   checked.False,
				},
			},
			event.Focus,
		},
		{
			"Search+Space",
			a11y.FindParams{
				Name: "CheckBox",
				Role: role.CheckBox,
				Attributes: map[string]interface{}{
					"className": arca11y.CheckBox,
					"checked":   checked.True,
				},
			},
			event.CheckedStateChanged,
		},
		{
			"Tab",
			a11y.FindParams{
				Name: "CheckBoxWithStateDescription",
				Role: role.CheckBox,
				Attributes: map[string]interface{}{
					"className": arca11y.CheckBox,
					"checked":   checked.False,
				},
			},
			event.Focus,
		},
		{
			"Tab",
			a11y.FindParams{
				Name: "seekBar",
				Role: role.Slider,
				Attributes: map[string]interface{}{
					"className":     arca11y.SeekBar,
					"valueForRange": 25,
				},
			},
			event.Focus,
		},
		{
			"=",
			a11y.FindParams{
				Name: "seekBar",
				Role: role.Slider,
				Attributes: map[string]interface{}{
					"className":     arca11y.SeekBar,
					"valueForRange": 26,
				},
			},
			event.RangeValueChanged,
		},
		{
			"Tab",
			a11y.FindParams{
				Role: role.Slider,
				Attributes: map[string]interface{}{
					"className":     arca11y.SeekBar,
					"valueForRange": 3,
				},
			},
			event.Focus,
		},
		{
			"-",
			a11y.FindParams{
				Role: role.Slider,
				Attributes: map[string]interface{}{
					"className":     arca11y.SeekBar,
					"valueForRange": 2,
				},
			},
			event.RangeValueChanged,
		},
	}

	EditTextActivityTestSteps := []axEventTestStep{
		{
			"Tab",
			a11y.FindParams{
				Name: "contentDescription",
				Role: role.TextField,
				Attributes: map[string]interface{}{
					"className": arca11y.EditText,
				},
			},
			event.Focus,
		},
		{
			"a",
			a11y.FindParams{
				Name: "contentDescription",
				Role: role.TextField,
				Attributes: map[string]interface{}{
					"className": arca11y.EditText,
					"value":     "a",
				},
			},
			event.ValueInTextFieldChanged,
		},
	}

	LiveRegionActivityTestSteps := []axEventTestStep{
		{
			"Tab",
			a11y.FindParams{
				Role: role.Button,
				Attributes: map[string]interface{}{
					"className": arca11y.Button,
					"name": regexp.MustCompile(
						`(CHANGE POLITE LIVE REGION|Change Polite Live Region)`,
					),
				},
			},
			event.Focus,
		},
		{
			"Enter",
			a11y.FindParams{
				Name: "Updated polite text",
				Role: role.StaticText,
				Attributes: map[string]interface{}{
					"className": arca11y.TextView,
				},
			},
			event.LiveRegionChanged,
		}, {
			"Tab",
			a11y.FindParams{
				Role: role.Button,
				Attributes: map[string]interface{}{
					"className": arca11y.Button,
					"name": regexp.MustCompile(
						`(CHANGE ASSERTIVE LIVE REGION|Change Assertive Live Region)`,
					),
				},
			},
			event.Focus,
		},
		{
			"Enter",
			a11y.FindParams{
				Name: "Updated assertive text",
				Role: role.StaticText,
				Attributes: map[string]interface{}{
					"className": arca11y.TextView,
				},
			},
			event.LiveRegionChanged,
		},
	}

	testActivities := []arca11y.TestActivity{
		arca11y.MainActivity, arca11y.EditTextActivity, arca11y.LiveRegionActivity,
	}

	testSteps := map[arca11y.TestActivity][]axEventTestStep{
		arca11y.MainActivity:       MainActivityTestSteps,
		arca11y.EditTextActivity:   EditTextActivityTestSteps,
		arca11y.LiveRegionActivity: LiveRegionActivityTestSteps,
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
