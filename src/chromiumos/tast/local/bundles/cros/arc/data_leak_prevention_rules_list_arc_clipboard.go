// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DataLeakPreventionRulesListArcClipboard,
		Desc: "Test behavior of DataLeakPreventionRulesList policy with clipboard blocked restriction on ARC",
		Contacts: []string{
			"vishal38785@gmail.com", // Test author
			"chromeos-dlp@google.com",
			"arc-framework+tast@google.com"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "fakeDMS",
		Timeout:      4 * time.Minute,
	})
}

func DataLeakPreventionRulesListArcClipboard(ctx context.Context, s *testing.State) {
	fakeDMS := s.FixtValue().(*fakedms.FakeDMS)

	policyDLP := []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable copy and paste of confidential content from site to ARC",
				Description: "User should not be able to copy and paste confidential content from site to ARC",
				Sources: &policy.DataLeakPreventionRulesListSources{
					Urls: []string{
						"example.com",
					},
				},
				Destinations: &policy.DataLeakPreventionRulesListDestinations{
					Components: []string{
						"ARC",
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListRestrictions{
					{
						Class: "CLIPBOARD",
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
		chrome.ARCEnabled(),
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fakeDMS.URL))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	const (
		// Simple app with text field.
		apk          = "ArcKeyboardTest.apk"
		pkg          = "org.chromium.arc.testapp.keyboard"
		activityName = ".MainActivity"
		// Used to identify text field in the app.
		fieldID = pkg + ":id/text"
	)

	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed to install the app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatalf("Failed to create a new activity %q", activityName)
	}
	defer act.Close()

	for _, param := range []struct {
		name        string
		copyAllowed bool
		url         string
	}{
		{
			name:        "example",
			copyAllowed: false,
			url:         "www.example.com",
		},
		{
			name:        "chromium",
			copyAllowed: true,
			url:         "www.chromium.org",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			conn, err := cr.NewConn(ctx, "https://"+param.url)
			if err != nil {
				s.Error("Failed to open page: ", err)
			}
			defer conn.Close()

			if err := kb.Accel(ctx, "Ctrl+A"); err != nil {
				s.Fatal("Failed to press Ctrl+A to select all content: ", err)
			}

			if err := kb.Accel(ctx, "Ctrl+C"); err != nil {
				s.Fatal("Failed to press Ctrl+C to copy all content: ", err)
			}

			if err := act.Start(ctx, tconn); err != nil {
				s.Fatalf("Failed to start the activity %q due to error: %v", activityName, err)
			}
			defer act.Stop(ctx, tconn)

			// Focus the input field and paste the text.
			if err := d.Object(ui.ID(fieldID)).WaitForExists(ctx, 30*time.Second); err != nil {
				s.Fatal("Failed to find the input field: ", err)
			}
			if err := d.Object(ui.ID(fieldID)).Click(ctx); err != nil {
				s.Fatal("Failed to click the input field: ", err)
			}
			if err := d.Object(ui.ID(fieldID), ui.Focused(true)).WaitForExists(ctx, 30*time.Second); err != nil {
				s.Fatal("Failed to focus on the input field: ", err)
			}
			// Set the initial state of the input.
			if err := d.Object(ui.ID(fieldID), ui.Focused(true)).SetText(ctx, ""); err != nil {
				s.Fatal("Failed to empty field: ", err)
			}
			if err := kb.Accel(ctx, "Ctrl+V"); err != nil {
				s.Fatal("Failed to press Ctrl+V to paste all content: ", err)
			}

			var copiedString string
			copiedString, err = d.Object(ui.ID(fieldID), ui.Focused(true)).GetText(ctx)
			if err != nil {
				s.Fatal("Failed to get input field text: ", err)
			}

			if copiedString != "" && !param.copyAllowed {
				s.Fatalf("String copied: got %q, want none", copiedString)
			}

			if copiedString == "" && param.copyAllowed {
				s.Fatalf("Failed to copy string: got %q, want not empty", copiedString)
			}

			pageTitle := strings.Title(param.name)

			if !strings.Contains(copiedString, pageTitle) && param.copyAllowed {
				s.Fatalf("Failed to check title in copy string: got %q, want containing title: %q", copiedString, pageTitle)
			}
		})
	}
}
