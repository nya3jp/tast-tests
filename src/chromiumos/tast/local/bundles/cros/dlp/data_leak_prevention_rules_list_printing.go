// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/dlp/restrictionlevel"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

// finder for the print dialog.
var printDialog = nodewith.Name("Print").HasClass("RootView").Role(role.Window)

type printingTestParams struct {
	name        string
	url         string
	restriction restrictionlevel.RestrictionLevel
	policyDLP   []policy.Policy
	browserType browser.Type
}

// DLP policy with printing blocked restriction.
var blockPolicy = []policy.Policy{&policy.DataLeakPreventionRulesList{
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
var warnPolicy = []policy.Policy{&policy.DataLeakPreventionRulesList{
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

func init() {
	testing.AddTest(&testing.Test{
		Func:         DataLeakPreventionRulesListPrinting,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test behavior of DataLeakPreventionRulesList policy with printing restrictions",
		Contacts: []string{
			"ayaelattar@google.com",
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Name:    "ash_blocked",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: printingTestParams{
				name:        "blocked",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.Blocked,
				policyDLP:   blockPolicy,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:      "ash_allowed",
			ExtraAttr: []string{"informational"},
			Fixture:   fixture.ChromePolicyLoggedIn,
			Val: printingTestParams{
				name:        "allowed",
				url:         "https://www.chromium.com/",
				restriction: restrictionlevel.Allowed,
				policyDLP:   blockPolicy,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:      "ash_warn_proceeded",
			ExtraAttr: []string{"informational"},
			Fixture:   fixture.ChromePolicyLoggedIn,
			Val: printingTestParams{
				name:        "warn_proceded",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.WarnProceeded,
				policyDLP:   warnPolicy,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:      "ash_warn_cancelled",
			ExtraAttr: []string{"informational"},
			Fixture:   fixture.ChromePolicyLoggedIn,
			Val: printingTestParams{
				name:        "warn_cancelled",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.WarnCancelled,
				policyDLP:   warnPolicy,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:              "lacros_blocked",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: printingTestParams{
				name:        "blocked",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.Blocked,
				policyDLP:   blockPolicy,
				browserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_allowed",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: printingTestParams{
				name:        "allowed",
				url:         "https://www.chromium.com/",
				restriction: restrictionlevel.Allowed,
				policyDLP:   blockPolicy,
				browserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_warn_proceeded",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: printingTestParams{
				name:        "warn_proceeded",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.WarnProceeded,
				policyDLP:   warnPolicy,
				browserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_warn_cancelled",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: printingTestParams{
				name:        "warn_cancelled",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.WarnCancelled,
				policyDLP:   warnPolicy,
				browserType: browser.TypeLacros,
			},
		}},
	})
}

func DataLeakPreventionRulesListPrinting(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fakeDMS := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

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

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+s.Param().(printingTestParams).name)

	// Update the policy blob.
	pb := policy.NewBlob()
	pb.AddPolicies(s.Param().(printingTestParams).policyDLP)

	// Update policy.
	if err := policyutil.ServeBlobAndRefresh(ctx, fakeDMS, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
	}

	br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), s.Param().(printingTestParams).browserType)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	conn, err := br.NewConn(ctx, s.Param().(printingTestParams).url)
	if err != nil {
		s.Fatal("Failed to open page: ", err)
	}
	defer conn.Close()

	// Make a call to print page.
	if err := testPrinting(ctx, tconn, keyboard, s.Param().(printingTestParams).restriction); err != nil {
		s.Fatal("Failed to run test body: ", err)
	}

	// Confirm that the notification only appeared if expected.
	if s.Param().(printingTestParams).restriction == restrictionlevel.Blocked {
		if _, err := ash.WaitForNotification(ctx, tconn, 15*time.Second, ash.WaitIDContains("print_dlp_blocked"), ash.WaitTitle("Printing is blocked")); err != nil {
			s.Error("Failed to wait for notification with title 'Printing is blocked': ", err)
		}
	}
}

// testPrinting tests whether printing is possible via hotkey (Ctrl + P).
func testPrinting(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter, restriction restrictionlevel.RestrictionLevel) error {
	// Type the shortcut.
	if err := keyboard.Accel(ctx, "Ctrl+P"); err != nil {
		return errors.Wrap(err, "failed to type printing hotkey")
	}

	if restriction == restrictionlevel.WarnProceeded {
		// Hit Enter, which is equivalent to clicking on the "Print anyway" button.
		if err := keyboard.Accel(ctx, "Enter"); err != nil {
			return errors.Wrap(err, "failed to hit Enter")
		}
	}

	if restriction == restrictionlevel.WarnCancelled {
		// Hit Esc, which is equivalent to clicking on the "Cancel" button.
		if err := keyboard.Accel(ctx, "Esc"); err != nil {
			return errors.Wrap(err, "failed to hit Esc")
		}
	}

	// Check that the printing dialog appears if and only if printing the page is allowed.
	ui := uiauto.New(tconn)

	if restriction == restrictionlevel.Allowed || restriction == restrictionlevel.WarnProceeded {
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
