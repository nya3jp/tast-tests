// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

type accessibilityTestTable struct {
	name      string          // name is the subtest name.
	policyKey string          // policyKey is the key for the policy value in chrome.accessibilityFeatures map.
	wantValue bool            // wantValue is the expected value of the policy once set.
	policies  []policy.Policy // policies is the policies values.
}

// smTestTable screen magnifier test table checking screenMagnifier key.
var smTestTable = []accessibilityTestTable{
	{
		name:      "enabled-1",
		policyKey: "screenMagnifier",
		wantValue: true,
		policies:  []policy.Policy{&policy.ScreenMagnifierType{Val: 1}},
	},
	{
		name:      "enabled-2",
		policyKey: "screenMagnifier",
		wantValue: false, // Negative test case as this value applies to dockedMagnifier.
		policies:  []policy.Policy{&policy.ScreenMagnifierType{Val: 2}},
	},
	{
		name:      "disable",
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
}

// dmTestTable screen magnifier test table checking dockedMagnifier key.
var dmTestTable = []accessibilityTestTable{
	{
		name:      "enabled-1",
		policyKey: "dockedMagnifier",
		wantValue: false, // Negative test case as this value applies to screenMagnifier.
		policies:  []policy.Policy{&policy.ScreenMagnifierType{Val: 1}},
	},
	{
		name:      "enabled-2",
		policyKey: "dockedMagnifier",
		wantValue: true,
		policies:  []policy.Policy{&policy.ScreenMagnifierType{Val: 2}},
	},
	{
		name:      "disable",
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
}

// hcTestTable high contrast test table.
var hcTestTable = []accessibilityTestTable{
	{
		name:      "enabled",
		policyKey: "highContrast",
		wantValue: true,
		policies:  []policy.Policy{&policy.HighContrastEnabled{Val: true}},
	},
	{
		name:      "disable",
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
}

// lcTestTable large cursor test table.
var lcTestTable = []accessibilityTestTable{
	{
		name:      "enabled",
		policyKey: "largeCursor",
		wantValue: true,
		policies:  []policy.Policy{&policy.LargeCursorEnabled{Val: true}},
	},
	{
		name:      "disable",
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
}

// vkTestTable virtual keyboard test table.
var vkTestTable = []accessibilityTestTable{
	{
		name:      "enabled",
		policyKey: "virtualKeyboard",
		wantValue: true,
		policies:  []policy.Policy{&policy.VirtualKeyboardEnabled{Val: true}},
	},
	{
		name:      "disable",
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
		// Once this test gets into QC please start https://crbug.com/1179710 .
		Attr: []string{"group:mainline", "informational"},
		// TODO: change to use Fixture: "chromePolicyLoggedIn".
		Pre: pre.User,
		Params: []testing.Param{
			{
				Name: "screen_magnifier",
				Val:  smTestTable,
			},
			{
				Name: "docked_magnifier",
				Val:  dmTestTable,
			},
			{
				Name: "high_contrast",
				Val:  hcTestTable,
			},
			{
				Name: "large_cursor",
				Val:  lcTestTable,
			},
			{
				Name: "virtual_keyboard",
				Val:  vkTestTable,
			},
		},
	})
}

// AccessibilityPolicies checks that accessibility policies have the correct
// value in chrome.accessibilityFeatures.
func AccessibilityPolicies(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Connect to Test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	tcs, ok := s.Param().([]accessibilityTestTable)
	if !ok {
		s.Fatal("Failed to convert test cases to the desired type")
	}

	for _, tc := range tcs {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
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
				s.Errorf("Failed to retrieve %s enabled value: %s", tc.policyKey, err)
			}

			if policyValue != tc.wantValue {
				s.Errorf("Unexpected value of chrome.accessibilityFeatures.%s: got %t; want %t", tc.policyKey, policyValue, tc.wantValue)
			}
		})
	}
}
