// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

type accessibilityTestCase struct {
	name      string          // name is the subtest name.
	policyKey string          // policyKey is the key for the policy value in chrome.accessibilityFeatures map.
	wantValue bool            // wantValue is the expected value of the policy once set.
	policies  []policy.Policy // policies is the policies values.
}

func init() {
	testing.AddTest(&testing.Test{
		Func: AccessibilityPolicies,
		Desc: "Checks set values for the Accessability polices in the chrome.accessibilityFeatures map",
		Contacts: []string{
			"kamilszarek@google.com", // Test author.
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		// autoclick case needs to be disabled.
		Attr:    []string{},
		Fixture: "chromePolicyLoggedIn",
		Params: []testing.Param{
			// TODO(crbug.com/1186655): Find a way to close/avoid the dialog about disabling autoclick.
			{
				Name: "autoclick",
				Val: []accessibilityTestCase{
					{
						name:      "enabled",
						policyKey: "autoclick",
						wantValue: true,
						policies:  []policy.Policy{&policy.AutoclickEnabled{Val: true}},
					},
					{
						name:      "disabled",
						policyKey: "autoclick",
						wantValue: false,
						policies:  []policy.Policy{&policy.AutoclickEnabled{Val: false}},
					},
					{
						name:      "unset",
						policyKey: "autoclick",
						wantValue: false,
						policies:  []policy.Policy{&policy.AutoclickEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:      "caret_highlight",
				ExtraAttr: []string{"group:mainline"},
				Val: []accessibilityTestCase{
					{
						name:      "enabled",
						policyKey: "caretHighlight",
						wantValue: true,
						policies:  []policy.Policy{&policy.CaretHighlightEnabled{Val: true}},
					},
					{
						name:      "disabled",
						policyKey: "caretHighlight",
						wantValue: false,
						policies:  []policy.Policy{&policy.CaretHighlightEnabled{Val: false}},
					},
					{
						name:      "unset",
						policyKey: "caretHighlight",
						wantValue: false,
						policies:  []policy.Policy{&policy.CaretHighlightEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:      "cursor_highlight",
				ExtraAttr: []string{"group:mainline"},
				Val: []accessibilityTestCase{
					{
						name:      "enabled",
						policyKey: "cursorHighlight",
						wantValue: true,
						policies:  []policy.Policy{&policy.CursorHighlightEnabled{Val: true}},
					},
					{
						name:      "disabled",
						policyKey: "cursorHighlight",
						wantValue: false,
						policies:  []policy.Policy{&policy.CursorHighlightEnabled{Val: false}},
					},
					{
						name:      "unset",
						policyKey: "cursorHighlight",
						wantValue: false,
						policies:  []policy.Policy{&policy.CursorHighlightEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:      "docked_magnifier",
				ExtraAttr: []string{"group:mainline"},
				Val: []accessibilityTestCase{
					{
						// Value 1 for this policy is allowed but does not
						// enable docked magnifier. Hence, the value for the
						// 'dockedMagnifier' key is expected to be false.
						name:      "enabled-full-screen",
						policyKey: "dockedMagnifier",
						wantValue: false, // Negative test case as this value applies to screenMagnifier.
						policies:  []policy.Policy{&policy.ScreenMagnifierType{Val: 1}},
					},
					{
						name:      "enabled-docked",
						policyKey: "dockedMagnifier",
						wantValue: true,
						policies:  []policy.Policy{&policy.ScreenMagnifierType{Val: 2}},
					},
					{
						name:      "disabled",
						policyKey: "dockedMagnifier",
						wantValue: false,
						policies:  []policy.Policy{&policy.ScreenMagnifierType{Val: 0}},
					},
					{
						name:      "unset",
						policyKey: "dockedMagnifier",
						wantValue: false,
						policies:  []policy.Policy{&policy.ScreenMagnifierType{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:      "focus_highlight",
				ExtraAttr: []string{"group:mainline"},
				Val: []accessibilityTestCase{
					{
						name:      "enabled",
						policyKey: "focusHighlight",
						wantValue: true,
						policies:  []policy.Policy{&policy.KeyboardFocusHighlightEnabled{Val: true}},
					},
					{
						name:      "disabled",
						policyKey: "focusHighlight",
						wantValue: false,
						policies:  []policy.Policy{&policy.KeyboardFocusHighlightEnabled{Val: false}},
					},
					{
						name:      "unset",
						policyKey: "focusHighlight",
						wantValue: false,
						policies:  []policy.Policy{&policy.KeyboardFocusHighlightEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:      "high_contrast",
				ExtraAttr: []string{"group:mainline"},
				Val: []accessibilityTestCase{
					{
						name:      "enabled",
						policyKey: "highContrast",
						wantValue: true,
						policies:  []policy.Policy{&policy.HighContrastEnabled{Val: true}},
					},
					{
						name:      "disabled",
						policyKey: "highContrast",
						wantValue: false,
						policies:  []policy.Policy{&policy.HighContrastEnabled{Val: false}},
					},
					{
						name:      "unset",
						policyKey: "highContrast",
						wantValue: false,
						policies:  []policy.Policy{&policy.HighContrastEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:      "large_cursor",
				ExtraAttr: []string{"group:mainline"},
				Val: []accessibilityTestCase{
					{
						name:      "enabled",
						policyKey: "largeCursor",
						wantValue: true,
						policies:  []policy.Policy{&policy.LargeCursorEnabled{Val: true}},
					},
					{
						name:      "disabled",
						policyKey: "largeCursor",
						wantValue: false,
						policies:  []policy.Policy{&policy.LargeCursorEnabled{Val: false}},
					},
					{
						name:      "unset",
						policyKey: "largeCursor",
						wantValue: false,
						policies:  []policy.Policy{&policy.LargeCursorEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:      "screen_magnifier",
				ExtraAttr: []string{"group:mainline"},
				Val: []accessibilityTestCase{
					{
						name:      "enabled-full-screen",
						policyKey: "screenMagnifier",
						wantValue: true,
						policies:  []policy.Policy{&policy.ScreenMagnifierType{Val: 1}},
					},
					{
						// Value 2 for this policy is allowed but enables
						// docked magnifier. Hence, the value for the
						// 'screenMagnifier' key is expected to be false.
						name:      "enabled-docker",
						policyKey: "screenMagnifier",
						wantValue: false, // Negative test case as this value applies to dockedMagnifier.
						policies:  []policy.Policy{&policy.ScreenMagnifierType{Val: 2}},
					},
					{
						name:      "disabled",
						policyKey: "screenMagnifier",
						wantValue: false,
						policies:  []policy.Policy{&policy.ScreenMagnifierType{Val: 0}},
					},
					{
						name:      "unset",
						policyKey: "screenMagnifier",
						wantValue: false,
						policies:  []policy.Policy{&policy.ScreenMagnifierType{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:      "select_to_speak",
				ExtraAttr: []string{"group:mainline"},
				Val: []accessibilityTestCase{
					{
						name:      "enabled",
						policyKey: "selectToSpeak",
						wantValue: true,
						policies:  []policy.Policy{&policy.SelectToSpeakEnabled{Val: true}},
					},
					{
						name:      "disabled",
						policyKey: "selectToSpeak",
						wantValue: false,
						policies:  []policy.Policy{&policy.SelectToSpeakEnabled{Val: false}},
					},
					{
						name:      "unset",
						policyKey: "selectToSpeak",
						wantValue: false,
						policies:  []policy.Policy{&policy.SelectToSpeakEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:      "spoken_feedback",
				ExtraAttr: []string{"group:mainline", "informational"},
				Timeout:   3 * time.Minute,
				Val: []accessibilityTestCase{
					{
						name:      "enabled",
						policyKey: "spokenFeedback",
						wantValue: true,
						policies:  []policy.Policy{&policy.SpokenFeedbackEnabled{Val: true}},
					},
					{
						name:      "disabled",
						policyKey: "spokenFeedback",
						wantValue: false,
						policies:  []policy.Policy{&policy.SpokenFeedbackEnabled{Val: false}},
					},
					{
						name:      "unset",
						policyKey: "spokenFeedback",
						wantValue: false,
						policies:  []policy.Policy{&policy.SpokenFeedbackEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:      "sticky_keys",
				ExtraAttr: []string{"group:mainline"},
				Val: []accessibilityTestCase{
					{
						name:      "enabled",
						policyKey: "stickyKeys",
						wantValue: true,
						policies:  []policy.Policy{&policy.StickyKeysEnabled{Val: true}},
					},
					{
						name:      "disabled",
						policyKey: "stickyKeys",
						wantValue: false,
						policies:  []policy.Policy{&policy.StickyKeysEnabled{Val: false}},
					},
					{
						name:      "unset",
						policyKey: "stickyKeys",
						wantValue: false,
						policies:  []policy.Policy{&policy.StickyKeysEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:      "virtual_keyboard",
				ExtraAttr: []string{"group:mainline", "informational"},
				Timeout:   3 * time.Minute,
				Val: []accessibilityTestCase{
					{
						name:      "enabled",
						policyKey: "virtualKeyboard",
						wantValue: true,
						policies:  []policy.Policy{&policy.VirtualKeyboardEnabled{Val: true}},
					},
					{
						name:      "disabled",
						policyKey: "virtualKeyboard",
						wantValue: false,
						policies:  []policy.Policy{&policy.VirtualKeyboardEnabled{Val: false}},
					},
					{
						name:      "unset",
						policyKey: "virtualKeyboard",
						wantValue: false,
						policies:  []policy.Policy{&policy.VirtualKeyboardEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
		},
	})
}

// AccessibilityPolicies checks that accessibility policies have the correct
// value in chrome.accessibilityFeatures.
func AccessibilityPolicies(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	tcs := s.Param().([]accessibilityTestCase)

	for _, tc := range tcs {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to reset Chrome: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, tc.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Run actual test.
			script := fmt.Sprintf(`(async () => {
				let result = await tast.promisify(tast.bind(chrome.accessibilityFeatures['%s'], "get"))({});
				return result.value;
			  })()`, tc.policyKey)

			var policyValue bool
			if err := tconn.Eval(ctx, script, &policyValue); err != nil {
				s.Fatalf("Failed to retrieve %q enabled value: %s", tc.policyKey, err)
			}

			if policyValue != tc.wantValue {
				s.Errorf("Unexpected value of chrome.accessibilityFeatures[%q]: got %t; want %t", tc.policyKey, policyValue, tc.wantValue)
			}
		})
	}
}
