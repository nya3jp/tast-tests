// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
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
		Fixture:      "chromePolicyLoggedIn",
	})
}

func DataLeakPreventionRulesListArcClipboard(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fakeDMS := s.FixtValue().(*fixtures.FixtData).FakeDMS

	policyDLP := []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable copy and paste of confidential content from site to ARC",
				Description: "User should not be able to copy and paste confidential content from site to ARC",
				Sources: &policy.DataLeakPreventionRulesListSources{
					Urls: []string{
						"example.com",
						"company.com",
					},
				},
				Destinations: &policy.DataLeakPreventionRulesListDestinations{
					Urls: []string{
						"*",
					},
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

	// Update policy.
	if err := policyutil.ServeBlobAndRefresh(ctx, fakeDMS, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
	}

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

	const (
		apk          = "ArcImagePasteTest.apk"
		pkg          = "org.chromium.arc.testapp.imagepaste"
		activityName = ".MainActivity"
		fieldID      = pkg + ":id/input_field"
	)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed to install the app: ", err)
	}

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
			wantAllowed: true,
			url:         "www.chromium.org",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			if _, err = cr.NewConn(ctx, "https://"+param.url); err != nil {
				s.Error("Failed to open page: ", err)
			}

			if err := kb.Accel(ctx, "Ctrl+A"); err != nil {
				s.Fatal("Failed to press Ctrl+A: ", err)
			}

			if err := kb.Accel(ctx, "Ctrl+C"); err != nil {
				s.Fatal("Failed to press Ctrl+C: ", err)
			}

			act, err := arc.NewActivity(a, pkg, activityName)
			if err != nil {
				s.Fatalf("Failed to create a new activity %q", activityName)
			}
			defer act.Close()

			if err := act.Start(ctx, tconn); err != nil {
				s.Fatalf("Failed to start the activity %q", activityName)
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
			if err := kb.Accel(ctx, "Ctrl+V"); err != nil {
				s.Fatal("Failed to press Ctrl+V: ", err)
			}

			var copiedString string
			copiedString, err = d.Object(ui.ID(fieldID), ui.Focused(true)).GetText(ctx)
			if err != nil {
				s.Fatal("Failed to get input field text: ", err)
			}

			if copiedString != "" && !param.wantAllowed {
				s.Fatalf("String copied: got %q, expected none", copiedString)
			}

			ui := uiauto.New(tconn)

			// Verify Clipboard toast.
			toastError := checkArcClipboardToast(ctx, ui, param.url, param.wantAllowed)
			if toastError != nil {
				s.Fatal("Couldn't check for toast: ", toastError)
			}
		})
	}
}

func checkArcClipboardToast(ctx context.Context, ui *uiauto.Context, url string, wantAllowed bool) error {
	bubbleMessage := nodewith.NameContaining("Sharing from " + url + " to Android apps has been blocked by administrator policy").First()

	err := ui.WaitUntilExists(bubbleMessage)(ctx)

	if err != nil && !wantAllowed {
		return errors.Wrap(err, "failed to check for clipboard toast: ")
	}

	if err == nil && wantAllowed {
		return errors.New("Clipboard toast found expected none")
	}

	return nil
}
