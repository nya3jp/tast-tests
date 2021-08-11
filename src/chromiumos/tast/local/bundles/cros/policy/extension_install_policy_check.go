// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

type extensionInstallPolicyTestTable struct {
	name         string          // name is the subtest name.
	allowInstall bool            // whether the extension should be allowed to be installed or not.
	policies     []policy.Policy // policies is a list of ExtensionInstallAllowlist, ExtensionInstallBlocklist policies.
}

// Google keep chrome extension.
const extensionID = "lpcaedmchfhocbbapmcbpinfpgnhiddi"
const extensionURL = "https://chrome.google.com/webstore/detail/" + extensionID

func init() {
	testing.AddTest(&testing.Test{
		Func: ExtensionInstallPolicyCheck,
		Desc: "Checks the behavior of ExtensionInstallAllowlist, ExtensionInstallBlocklist policies",
		Contacts: []string{
			"swapnilgupta@google.com", //Test Author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
		Timeout:      4 * time.Minute, // There is a longer wait when installing the extension.
		Params: []testing.Param{
			{
				Name: "blocklist_wildcard",
				Val: []extensionInstallPolicyTestTable{
					{
						name:         "allowlist_set",
						allowInstall: true,
						policies: []policy.Policy{
							// Test API extension should be specified in allow list, otherwise it would get disabled automatically.
							&policy.ExtensionInstallAllowlist{Val: []string{extensionID, chrome.TestExtensionID}},
							&policy.ExtensionInstallBlocklist{Val: []string{"*"}},
						},
					},
					{
						name:         "allowlist_set_with_test_api_extension",
						allowInstall: false,
						policies: []policy.Policy{
							&policy.ExtensionInstallAllowlist{Val: []string{chrome.TestExtensionID}},
							&policy.ExtensionInstallBlocklist{Val: []string{"*"}},
						},
					},
				},
			},
			{
				Name: "blocklist_unset",
				Val: []extensionInstallPolicyTestTable{
					{
						name:         "allowlist_set",
						allowInstall: true,
						policies: []policy.Policy{
							&policy.ExtensionInstallAllowlist{Val: []string{extensionID}},
						},
					},
					{
						name:         "allowlist_unset",
						allowInstall: true,
						policies:     []policy.Policy{},
					},
				},
			},
			{
				Name: "blocklist_set",
				Val: []extensionInstallPolicyTestTable{
					{
						name:         "allowlist_set",
						allowInstall: false,
						policies: []policy.Policy{
							&policy.ExtensionInstallAllowlist{Val: []string{extensionID}},
							&policy.ExtensionInstallBlocklist{Val: []string{extensionID}},
						},
					},
					{
						name:         "allowlist_unset",
						allowInstall: false,
						policies: []policy.Policy{
							&policy.ExtensionInstallBlocklist{Val: []string{extensionID}},
						},
					},
				},
			},
		},
	})
}

func ExtensionInstallPolicyCheck(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	tcs, ok := s.Param().([]extensionInstallPolicyTestTable)
	if !ok {
		s.Fatal("Failed to convert test cases to the desired type")
	}

	for _, tc := range tcs {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+tc.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, tc.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Run actual test.
			if allowInstall, err := isInstallationAllowed(ctx, tconn, cr); err != nil {
				s.Fatal("Failed to check if extension can be installed: ", err)
			} else if allowInstall != tc.allowInstall {
				s.Errorf("Unexpected result: got %t; want %t", allowInstall, tc.allowInstall)
			}
		})
	}

}

// isInstallationAllowed verifies whether the extension should be allowed to install or not.
func isInstallationAllowed(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (bool, error) {
	addParam := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "Add to Chrome",
	}
	blockedParam := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "Blocked by admin",
	}

	// Open the Chrome Web Store page of the extension.
	conn, err := cr.NewConn(ctx, extensionURL)
	if err != nil {
		return false, errors.Wrap(err, "failed to connect to chrome")
	}
	defer conn.Close()

	var allowInstall bool

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// For blocked extensions, there should be a blocked button.
		if blocked, err := ui.Exists(ctx, tconn, blockedParam); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to check Blocked by admin button"))
		} else if blocked {
			allowInstall = false
			return nil
		}

		// For allowed extensions, there should be a button to add them.
		if allowed, err := ui.Exists(ctx, tconn, addParam); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to check Add to chrome button"))
		} else if allowed {
			allowInstall = true
			return nil
		}

		return errors.New("failed to determine the outcome")
	}, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil {
		return false, err
	}

	return allowInstall, nil
}
