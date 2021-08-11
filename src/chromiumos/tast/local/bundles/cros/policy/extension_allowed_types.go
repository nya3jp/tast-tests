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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ExtensionAllowedTypes,
		Desc: "Behavior of ExtensionAllowedTypes policy, checking if a theme can be added to Chrome",
		Contacts: []string{
			"gabormagda@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
		Timeout:      4 * time.Minute, // There is a longer wait when installing the extension.
	})
}

// ExtensionAllowedTypes tests the ExtensionAllowedTypes policy.
func ExtensionAllowedTypes(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// "Just Black" theme identifiers.
	const (
		id  = "aghfnjkcakhmadgdomlmlhhaocbkloab"
		url = "https://chrome.google.com/webstore/detail/" + id
	)

	// When the policy is set, "extension" must be added, so the TestAPI is not disabled.
	for _, param := range []struct {
		name        string
		wantAllowed bool // wantAllowed is the expected value of whether an extension is allowed to be added to Chrome.
		value       *policy.ExtensionAllowedTypes
	}{
		{
			name:        "unset",
			wantAllowed: true,
			value:       &policy.ExtensionAllowedTypes{Stat: policy.StatusUnset},
		},
		{
			name:        "extension_only",
			wantAllowed: false,
			value:       &policy.ExtensionAllowedTypes{Val: []string{"extension"}},
		},
		{
			name:        "extension_and_theme",
			wantAllowed: true,
			value:       &policy.ExtensionAllowedTypes{Val: []string{"extension", "theme"}},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Run actual test.
			if allowed, err := canInstallExtension(ctx, tconn, cr, url); err != nil {
				s.Fatal("Failed to check if extension can be installed: ", err)
			} else if allowed != param.wantAllowed {
				s.Errorf("Unexpected result upon installing the extension: got %t; want %t", allowed, param.wantAllowed)
			}
		})
	}
}

func canInstallExtension(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, url string) (bool, error) {
	addButton := nodewith.Name("Add to Chrome").Role(role.Button).First()
	blockedButton := nodewith.Name("Close").ClassName("MdTextButton")
	undoButton := nodewith.Name("Undo").ClassName("MdTextButton")

	// Open the Chrome Web Store page of the extension.
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return false, errors.Wrap(err, "failed to connect to chrome")
	}
	defer conn.Close()

	uia := uiauto.New(tconn)

	// Install extension.
	if err := uia.LeftClick(addButton)(ctx); err != nil {
		return false, errors.Wrap(err, "failed to click Add to Chrome button")
	}

	installed := false
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if blocked, err := uia.IsNodeFound(ctx, blockedButton); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to check Close button"))
		} else if blocked {
			installed = false
			return nil
		}

		if allowed, err := uia.IsNodeFound(ctx, undoButton); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to check Undo button"))
		} else if allowed {
			installed = true
			return nil
		}

		return errors.New("failed to determine installation outcome")
	}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
		return false, err
	}

	// Remove extension if needed. If it is not removed, the next subtests will fail.
	// A theme can only be removed here, or by installing a new one.
	if installed {
		if err := uia.LeftClick(undoButton)(ctx); err != nil {
			return false, errors.Wrap(err, "failed to click Undo button")
		}

		// Wait until removing is complete.
		if err := uia.WaitUntilExists(addButton)(ctx); err != nil {
			return false, errors.Wrap(err, "failed to wait for Add to Chrome button")
		}
	}

	return installed, nil
}
