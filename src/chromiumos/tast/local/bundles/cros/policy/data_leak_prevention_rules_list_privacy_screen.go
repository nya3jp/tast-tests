// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DataLeakPreventionRulesListPrivacyScreen,
		Desc: "Test behavior of DataLeakPreventionRulesList policy with privacy screen blocked restriction",
		Contacts: []string{
			"vishal38785@gmail.com", // Test author
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "fakeDMS",
	})
}

func DataLeakPreventionRulesListPrivacyScreen(ctx context.Context, s *testing.State) {
	fakeDMS := s.FixtValue().(*fakedms.FakeDMS)

	// DLP policy with privacy screen blocked restriction.
	policyDLP := []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Enable privacy screen for confidential content in restricted source",
				Description: "Privacy screen should be enabled when on restricted site",
				Sources: &policy.DataLeakPreventionRulesListSources{
					Urls: []string{
						"example.com",
						"chromium.org",
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListRestrictions{
					{
						Class: "PRIVACY_SCREEN",
						Level: "BLOCK",
					},
				},
			},
		},
	},
	}

	// policyDLP := []policy.Policy{&policy.PrivacyScreenEnabled{Val: true}}

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

	screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create ScreenRecorder: ", err)
	}
	if screenRecorder == nil {
		s.Fatal("Screen recorder was not found")
	}
	defer uiauto.ScreenRecorderStopSaveRelease(ctx, screenRecorder, filepath.Join(s.OutDir(), "dlpScreenShare.mp4"))
	screenRecorder.Start(ctx, tconn)

	for _, param := range []struct {
		name        string
		wantAllowed bool
		url         string
	}{
		{
			name:        "Example",
			wantAllowed: false,
			url:         "www.example.com",
		},
		{
			name:        "Chromium",
			wantAllowed: false,
			url:         "www.chromium.org",
		},
		{
			name:        "Company",
			wantAllowed: true,
			url:         "www.company.com",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {

			if _, err := cr.NewConn(ctx, "https://"+param.url); err != nil {
				s.Error("Failed to open page: ", err)
			}

			// faillog.DumpUITreeAndScreenshot(ctx, tconn, "ui_tree_"+param.name, "err")

			// ui := uiauto.New(tconn)

			// err := checkPrivacyScreenOnBubble(ctx, ui, param.wantAllowed)

			// if err != nil {
			// 	s.Fatal("Couldn't check for notification: ", err)
			// }

			value, err := privacyScreenValue(ctx)

			if err != nil {
				s.Fatal("Couldn't check value for privacy screen prop: ", err)
			}

			if !param.wantAllowed && !value {
				s.Errorf("Privacy screen prop got value %q expected: ", value)
			}

			if param.wantAllowed && value {
				s.Errorf("Privacy screen enabled got value %q expected: ", value)
			}

			if _, err := cr.NewConn(ctx, "https://www.google.com"); err != nil {
				s.Error("Failed to open page: ", err)
			}

			// faillog.DumpUITreeAndScreenshot(ctx, tconn, "ui_tree_google_"+param.name, "err")

			// bubbleMessage := nodewith.NameContaining("Privacy screen").Onscreen()
			// err = uiauto.Combine("Privacy screen off bubble ",
			// 	ui.WaitUntilExists(bubbleMessage))(ctx)

			// if err != nil {
			// 	s.Fatal("Failed to check for privacy screen off bubble bubble existence: ", err)
			// }

			// if _, err := ash.WaitForNotification(ctx, tconn, 15*time.Second, ash.WaitTitle("Privacy screen is off")); err != nil {
			// 	s.Fatalf("Failed to wait for notification with title %q: %v", "param.wantNotification", err)
			// }

			if err := testing.Sleep(ctx, time.Second); err != nil {
				s.Fatal("Failed to sleep: ", err)
			}

			value, err = privacyScreenValue(ctx)

			if value {
				s.Errorf("Privacy screen prop enabled got value %q expected: ", value)
			}

			// err = checkPrivacyScreenOffBubble(ctx, ui, param.wantAllowed)

			// if err != nil {
			// 	s.Fatal("Couldn't check for notification: ", err)
			// }
		})
	}
}

func checkPrivacyScreenOnBubble(ctx context.Context, ui *uiauto.Context, wantAllowed bool) error {

	// bubbleView := nodewith.ClassName("TrayBubbleView")
	bubbleMessage := nodewith.NameContaining("Privacy screen is on")

	err := uiauto.Combine("Privacy screen on bubble ",
		ui.WaitUntilExists(bubbleMessage))(ctx)

	if err != nil && !wantAllowed {
		return errors.Wrap(err, "failed to check for privacy screen on bubble existence: ")
	}

	if err == nil && wantAllowed {
		return errors.New("Privacy screen on bubble found expected none")
	}

	return nil
}

func checkPrivacyScreenOffBubble(ctx context.Context, ui *uiauto.Context, wantAllowed bool) error {

	// bubbleView := nodewith.ClassName("TrayBubbleView").Role(role.Window)
	bubbleMessage := nodewith.NameContaining("Privacy screen is off").Role(role.Dialog)

	err := uiauto.Combine("Privacy screen off bubble ",
		ui.WaitUntilExists(bubbleMessage))(ctx)

	if err != nil && !wantAllowed {
		return errors.Wrap(err, "failed to check for privacy screen off bubble bubble existence: ")
	}

	if err == nil && wantAllowed {
		return errors.New("Privacy screen off bubble found expected none")
	}

	return nil
}

// privacyScreenValue retrieves value of privacy screen prop.
func privacyScreenValue(ctx context.Context) (bool, error) {

	output, err := testexec.CommandContext(ctx, "modetest", "-c").Output()
	if err != nil {
		return false, err
	}

	outputSlice := strings.Split(string(output), "privacy-screen:")

	if len(outputSlice) == 1 {
		return false, errors.New("failed to find privacy screen prop")
	}

	for _, line := range strings.Split(outputSlice[1], "\n") {
		matches := strings.Contains(line, "value:")
		if matches {
			if found := strings.Contains(line, "1"); found {
				return true, nil
			}

			if found := strings.Contains(line, "0"); found {
				return false, nil
			}

			return false, errors.New("failed to find value for privacy screen prop")

		}
	}

	return false, nil

}
