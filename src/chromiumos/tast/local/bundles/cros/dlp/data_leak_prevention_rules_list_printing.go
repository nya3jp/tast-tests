// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DataLeakPreventionRulesListPrinting,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test behavior of DataLeakPreventionRulesList policy with printing blocked restriction",
		Contacts: []string{
			"vishal38785@gmail.com", // Test author
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

// finder for the print dialog.
var printDialog = nodewith.Name("Print").HasClass("RootView").Role(role.Window)

// finder for the warning dialog.
var warningDialog = nodewith.Name("Print confidential content?").Role(role.Window)

// Type of the DLP restriction enforced, including the user's action to the warning dialog (proceed or cancel).
type restrictionLevel int

const (
	allowed restrictionLevel = iota
	blocked
	warnCancelled
	warnProceeded
)

func DataLeakPreventionRulesListPrinting(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fakeDMS := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// DLP policy with printing blocked restriction.
	blockPolicy := []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable printing of confidential content",
				Description: "User should not be able to print confidential content",
				Sources: &policy.DataLeakPreventionRulesListValueSources{
					Urls: []string{
						"example.com",
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListValueRestrictions{
					{
						Class: "PRINTING",
						Level: "BLOCK",
					},
				},
			},
		},
	},
	}

	// DLP policy with printing warn restriction.
	warnPolicy := []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Warn before printing confidential content",
				Description: "User should be warned before printing confidential content",
				Sources: &policy.DataLeakPreventionRulesListValueSources{
					Urls: []string{
						"example.com",
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListValueRestrictions{
					{
						Class: "PRINTING",
						Level: "WARN",
					},
				},
			},
		},
	},
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

	for _, param := range []struct {
		name        string
		url         string
		restriction restrictionLevel
		policyDLP   []policy.Policy
	}{
		{
			name:        "blocked",
			url:         "https://www.example.com/",
			restriction: blocked,
			policyDLP:   blockPolicy,
		},
		{
			name:        "warnAndCancel",
			url:         "https://www.example.com/",
			restriction: warnCancelled,
			policyDLP:   warnPolicy,
		},
		{
			name:        "warnAndProceed",
			url:         "https://www.example.com/",
			restriction: warnProceeded,
			policyDLP:   warnPolicy,
		},
		{
			name:        "chromium",
			url:         "https://www.chromium.org/",
			restriction: allowed,
			policyDLP:   blockPolicy,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Update the policy blob.
			pb := fakedms.NewPolicyBlob()
			pb.AddPolicies(param.policyDLP)

			// Update policy.
			if err := policyutil.ServeBlobAndRefresh(ctx, fakeDMS, cr, pb); err != nil {
				s.Fatal("Failed to serve and refresh: ", err)
			}

			conn, err := cr.NewConn(ctx, param.url)
			if err != nil {
				s.Fatal("Failed to open page: ", err)
			}
			defer conn.Close()

			// Make a call to print page.
			if err := testPrinting(ctx, tconn, keyboard, param.restriction); err != nil {
				s.Fatal("Failed to run test body: ", err)
			}

			// Confirm that the notification only appeared if expected.
			if param.restriction == blocked {
				if _, err := ash.WaitForNotification(ctx, tconn, 15*time.Second, ash.WaitIDContains("print_dlp_blocked"), ash.WaitTitle("Printing is blocked")); err != nil {
					s.Error("Failed to wait for notification with title 'Printing is blocked': ", err)
				}
			}
		})
	}
}

// testPrinting tests whether printing is possible via hotkey (Ctrl + P).
func testPrinting(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter, restriction restrictionLevel) error {
	// Type the shortcut.
	if err := keyboard.Accel(ctx, "Ctrl+P"); err != nil {
		return errors.Wrap(err, "failed to type printing hotkey")
	}

	if restriction == warnProceeded {
		// Hit Enter, which is equivalent to clicking on the "Print anyway" button.
		if err := keyboard.Accel(ctx, "Enter"); err != nil {
			return errors.Wrap(err, "failed to hit Enter")
		}
	}

	if restriction == warnCancelled {
		// Hit Esc, which is equivalent to clicking on the "Cancel" button.
		if err := keyboard.Accel(ctx, "Esc"); err != nil {
			return errors.Wrap(err, "failed to hit Esc")
		}
	}

	// Check that the printing dialog appears if and only if printing the page is allowed.
	ui := uiauto.New(tconn)

	if restriction == allowed || restriction == warnProceeded {
		if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(printDialog)(ctx); err != nil {
			return errors.Wrap(err, "failed to find the printing window")
		}
	} else {
		if err := ui.EnsureGoneFor(printDialog, 5*time.Second)(ctx); err != nil {
			return errors.Wrap(err, "should not show the printing window")
		}
	}

	return nil
}
