// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SecondaryGoogleAccountSigninAllowed,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test behavior of SecondaryGoogleAccountSigninAllowed policy: check if Add account button is restricted based on the value of the policy", // TODO(chromium:1128915): Add test cases for signin screen.
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:commercial_limited"},
		Fixture:      fixture.FakeDMS,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.SecondaryGoogleAccountSigninAllowed{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func SecondaryGoogleAccountSigninAllowed(ctx context.Context, s *testing.State) {
	fakeDMS := s.FixtValue().(*fakedms.FakeDMS)

	for _, param := range []struct {
		name           string
		wantRestricted restriction.Restriction                     // wantRestricted is the expected restriction state of the "Add Google Account" button.
		policy         *policy.SecondaryGoogleAccountSigninAllowed // policy is the policy we test.
	}{
		{
			name:           "unset",
			wantRestricted: restriction.None,
			policy:         &policy.SecondaryGoogleAccountSigninAllowed{Stat: policy.StatusUnset},
		},
		{
			name:           "not_allowed",
			wantRestricted: restriction.Disabled,
			policy:         &policy.SecondaryGoogleAccountSigninAllowed{Val: false},
		},
		{
			name:           "allowed",
			wantRestricted: restriction.None,
			policy:         &policy.SecondaryGoogleAccountSigninAllowed{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Update the policy blob.
			pb := policy.NewBlob()
			pb.AddPolicies([]policy.Policy{param.policy})
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

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Open people settings page.
			conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/osPeople")
			if err != nil {
				s.Fatal("Failed to open OS settings accounts page: ", err)
			}
			defer conn.Close()

			// Connect to Test API to use it with the UI library.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			ui := uiauto.New(tconn)

			// Find and click the Google accounts button.
			accountButton := nodewith.Name("Google Accounts").Role(role.Button)
			if err = ui.WaitUntilExists(accountButton)(ctx); err != nil {
				s.Fatal("Google Accounts button not found: ", err)
			}
			if err := ui.LeftClick(accountButton)(ctx); err != nil {
				s.Fatal("Failed to click Google Accounts button: ", err)
			}

			addAccountButton := nodewith.Name("Add Google Account").Role(role.Button)
			viewAccountButton := nodewith.Name("View accounts").Role(role.Button)
			// We might get a dialog box where we have to click a button before we get to the actual settings we need.
			if err := testing.Poll(ctx, func(ctx context.Context) error {

				// If the Add Google Account Button already exists we can continue.
				if err = ui.Exists(addAccountButton)(ctx); err == nil {
					return nil
				}

				// Check if we have the dialog and if so click the View account button to continue.
				if err = ui.Exists(viewAccountButton)(ctx); err != nil {
					return errors.New("Add Google Account and View accounts button not found")
				}
				if err := ui.LeftClick(viewAccountButton)(ctx); err != nil {
					return testing.PollBreak(errors.Wrap(err, "failed to click View acounts button"))
				}

				return nil

			}, nil); err != nil {
				s.Fatal("Could not find Add Google Account button: ", err)
			}

			// Get the node info for the Add Google Account button.
			nodeInfo, err := ui.Info(ctx, addAccountButton)
			if err != nil {
				s.Fatal("Could not get info for the Add Google Account button: ", err)
			}

			if nodeInfo.Restriction != param.wantRestricted {
				s.Errorf("Unexpected button restriction in the settings: got %s; want %s", nodeInfo.Restriction, param.wantRestricted)
			}
		})
	}
}
