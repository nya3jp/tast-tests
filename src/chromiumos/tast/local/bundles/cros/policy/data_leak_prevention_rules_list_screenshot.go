// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DataLeakPreventionRulesListScreenshot,
		Desc: "Test behavior of DataLeakPreventionRulesList policy with screenshot blocked restriction",
		Contacts: []string{
			"vishal38785@gmail.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "fakeDMS",
	})
}

func DataLeakPreventionRulesListScreenshot(ctx context.Context, s *testing.State) {
	fakeDMS := s.FixtValue().(*fakedms.FakeDMS)

	// DLP policy with screenshots blocked restriction.
	policyDLP := []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable Screenshot in confidential content",
				Description: "User should not be able to take screen of confidential content",
				Sources: &policy.DataLeakPreventionRulesListSources{
					Urls: []string{
						"salesforce.com",
						"google.com",
						"company.com",
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListRestrictions{
					{
						Class: "SCREENSHOT",
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

	captureNotAllowed := "Can't capture confidential content"

	for _, param := range []struct {
		name             string // Name
		wantNotification string // Want Notification
		wantAllowed      bool   // Want Allowed
		url              string // Url String
	}{
		{
			name:             "Salesforce",
			wantAllowed:      false,
			wantNotification: captureNotAllowed,
			url:              "https://www.salesforce.com/",
		},
		{
			name:             "Google",
			wantAllowed:      false,
			wantNotification: captureNotAllowed,
			url:              "https://www.google.com/",
		},
		{
			name:             "Company",
			wantAllowed:      false,
			wantNotification: captureNotAllowed,
			url:              "https://www.company.com/",
		},
		{
			name:             "Chromium",
			wantAllowed:      true,
			wantNotification: "Screenshot taken",
			url:              "https://www.chromium.org/",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Please check kScreenshotMinimumIntervalInMS constant in
			// ui/snapshot/screenshot_grabber.cc
			if err := testing.Sleep(ctx, time.Second); err != nil {
				s.Fatal("Failed to sleep: ", err)
			}

			if err := ash.CloseNotifications(ctx, tconn); err != nil {
				s.Fatal("Failed to close notifications: ", err)
			}
			if err := screenshot.RemoveScreenshots(); err != nil {
				s.Fatal("Failed to remove screenshots: ", err)
			}

			if _, err = cr.NewConn(ctx, param.url); err != nil {
				s.Error("Failed to open page: ", err)
			}

			if err := keyboard.Accel(ctx, "Ctrl+F5"); err != nil {
				s.Fatal("Failed to press Ctrl+F5 to take screenshot: ", err)
			}

			if _, err := ash.WaitForNotification(ctx, tconn, 15*time.Second, ash.WaitIDContains("capture_mode_notification"), ash.WaitTitle(param.wantNotification)); err != nil {
				s.Fatalf("Failed to wait notification with title %q: %v", param.wantNotification, err)
			}

			has, err := screenshot.HasScreenshots()
			if err != nil {
				s.Fatal("Failed to check whether screenshot is present")
			}
			if has != param.wantAllowed {
				s.Errorf("Unexpected screenshot allowed: get %t; want %t", has, param.wantAllowed)
			}
		})
	}
}
