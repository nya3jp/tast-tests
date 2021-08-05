// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
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
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func DataLeakPreventionRulesListScreenshot(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fakeDMS := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// DLP policy with screenshots blocked restriction.
	policyDLP := []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable Screenshot in confidential content",
				Description: "User should not be able to take screen of confidential content",
				Sources: &policy.DataLeakPreventionRulesListSources{
					Urls: []string{
						"example.com",
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

	// Update policy.
	if err := policyutil.ServeBlobAndRefresh(ctx, fakeDMS, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
	}

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

	// Notification strings.
	const captureNotAllowed = "Can't capture confidential content"
	const captureAllowed = "Screenshot taken"

	for _, param := range []struct {
		name             string
		wantNotification string
		wantAllowed      bool
		url              string
	}{
		{
			name:             "example",
			wantAllowed:      false,
			wantNotification: captureNotAllowed,
			url:              "https://www.example.com/",
		},
		{
			name:             "chromium",
			wantAllowed:      true,
			wantNotification: captureAllowed,
			url:              "https://www.chromium.org/",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			if err := ash.CloseNotifications(ctx, tconn); err != nil {
				s.Fatal("Failed to close notifications: ", err)
			}
			if err := screenshot.RemoveScreenshots(); err != nil {
				s.Fatal("Failed to remove screenshots: ", err)
			}

			conn, err := cr.NewConn(ctx, param.url)
			if err != nil {
				s.Error("Failed to open page: ", err)
			}
			defer conn.Close()

			if err := keyboard.Accel(ctx, "Ctrl+F5"); err != nil {
				s.Fatal("Failed to press Ctrl+F5 to take screenshot: ", err)
			}

			if _, err := ash.WaitForNotification(ctx, tconn, 15*time.Second, ash.WaitIDContains("capture_mode_notification"), ash.WaitTitle(param.wantNotification)); err != nil {
				s.Fatalf("Failed to wait for notification with title %q: %v", param.wantNotification, err)
			}

			has, err := screenshot.HasScreenshots()
			if err != nil {
				s.Fatal("Failed to check whether screenshot is present: ", err)
			}
			if has != param.wantAllowed {
				s.Errorf("Unexpected screenshot allowed: get %v; want %v", has, param.wantAllowed)
			}
		})
	}
}
