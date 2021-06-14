// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DataLeakPreventionRulesListPrinting,
		Desc: "Test behavior of DataLeakPreventionRulesList policy with printing blocked restriction",
		Contacts: []string{
			"vishal38785@gmail.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "fakeDMS",
	})
}

func DataLeakPreventionRulesListPrinting(ctx context.Context, s *testing.State) {
	fakeDMS := s.FixtValue().(*fakedms.FakeDMS)

	// DLP policy with printing blocked restriction.
	policyDLP := []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable Printing in confidential content",
				Description: "User should not be able to print confidential content",
				Sources: &policy.DataLeakPreventionRulesListSources{
					Urls: []string{
						"salesforce.com",
						"google.com",
						"company.com",
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListRestrictions{
					{
						Class: "PRINTING",
						Level: "BLOCK",
					},
				},
			},
		},
	},
	}

	// Update the policy blob.
	pb := fakedms.NewPolicyBlob()
	pb.AddPolicies(policyDLP)
	if err := fakeDMS.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	// Policies are only updated after Chrome startup.
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fakeDMS.URL))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// Connect to Test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	printingNotAllowed := "Printing is blocked"

	for _, param := range []struct {
		name             string
		printingAllowed  bool
		url              string
		wantNotification string
	}{
		{
			name:             "Salesforce",
			printingAllowed:  false,
			url:              "https://www.salesforce.com/",
			wantNotification: printingNotAllowed,
		},
		{
			name:             "Google",
			printingAllowed:  false,
			url:              "https://www.google.com/",
			wantNotification: printingNotAllowed,
		},
		{
			name:             "Company",
			printingAllowed:  false,
			url:              "https://www.company.com/",
			wantNotification: printingNotAllowed,
		},
		{
			name:             "Chromium",
			printingAllowed:  true,
			url:              "https://www.chromium.org/",
			wantNotification: "Printing allowed no notification",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {

			conn, err := cr.NewConn(ctx, param.url)
			if err != nil {
				s.Fatal("Failed to open page: ", err)
			}
			defer conn.Close()

			// Make a call to the test case body.
			printingPossible, err := testPrinting(ctx, tconn, keyboard)
			if err != nil {
				s.Fatal("Failed to run test body: ", err)
			}

			if printingPossible != param.printingAllowed {
				s.Errorf("Unexpected printing restriction; got: %t, want: %t", printingPossible, param.printingAllowed)
			}

			if !param.printingAllowed {
				if _, err := ash.WaitForNotification(ctx, tconn, 3*time.Second, ash.WaitIDContains("print_dlp_blocked"), ash.WaitTitle(param.wantNotification)); err != nil {
					s.Fatalf("Failed to wait notification with title %q: %v", param.wantNotification, err)
				}
			}
		})
	}
}

// testPrinting tests whether printing is possible via hotkey (Ctrl + P).
func testPrinting(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter) (bool, error) {
	// Type the shortcut.
	if err := keyboard.Accel(ctx, "Ctrl+P"); err != nil {
		return false, errors.Wrap(err, "failed to type printing hotkey: ")
	}

	// Check if printing dialog has appeared.
	printWindowExists := true
	findParams := ui.FindParams{Name: "Print", ClassName: "RootView", Role: ui.RoleTypeWindow}
	if err := ui.WaitUntilExists(ctx, tconn, findParams, 10*time.Second); err != nil {
		// If function above failed, it could be either a timeout or an actual error. Check once again.
		printWindowExists, err = ui.Exists(ctx, tconn, findParams)
		// If the dialog does not exist by now, we assume that it will never be displayed.
		if err != nil {
			return false, errors.Wrap(err, "failed to check for printing windows existance: ")
		}
	}

	return printWindowExists, nil
}
