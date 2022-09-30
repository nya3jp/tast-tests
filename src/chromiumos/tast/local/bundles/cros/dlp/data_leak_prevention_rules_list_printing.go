// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"time"

	"chromiumos/tast/common/fixture"
	policyBlob "chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/dlp/policy"
	"chromiumos/tast/local/bundles/cros/dlp/restrictionlevel"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

// finder for the print dialog.
var printDialog = nodewith.Name("Print").HasClass("RootView").Role(role.Window)

type printingTestParams struct {
	name        string
	path        string
	restriction restrictionlevel.RestrictionLevel
	browserType browser.Type
}

const (
	dlpPrintingBlockedPath = "/blocked"
	dlpPrintingAllowedPath = "/allowed"
	dlpPrintingWarnPath    = "/warn"
)

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
				path:        dlpPrintingBlockedPath,
				restriction: restrictionlevel.Blocked,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:      "ash_allowed",
			ExtraAttr: []string{"informational"},
			Fixture:   fixture.ChromePolicyLoggedIn,
			Val: printingTestParams{
				name:        "allowed",
				path:        dlpPrintingAllowedPath,
				restriction: restrictionlevel.Allowed,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:      "ash_warn_proceeded",
			ExtraAttr: []string{"informational"},
			Fixture:   fixture.ChromePolicyLoggedIn,
			Val: printingTestParams{
				name:        "warn_proceded",
				path:        dlpPrintingWarnPath,
				restriction: restrictionlevel.WarnProceeded,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:      "ash_warn_cancelled",
			ExtraAttr: []string{"informational"},
			Fixture:   fixture.ChromePolicyLoggedIn,
			Val: printingTestParams{
				name:        "warn_cancelled",
				path:        dlpPrintingWarnPath,
				restriction: restrictionlevel.WarnCancelled,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:              "lacros_blocked",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: printingTestParams{
				name:        "blocked",
				path:        dlpPrintingBlockedPath,
				restriction: restrictionlevel.Blocked,
				browserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_allowed",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: printingTestParams{
				name:        "allowed",
				path:        dlpPrintingAllowedPath,
				restriction: restrictionlevel.Allowed,
				browserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_warn_proceeded",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: printingTestParams{
				name:        "warn_proceeded",
				path:        dlpPrintingWarnPath,
				restriction: restrictionlevel.WarnProceeded,
				browserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_warn_cancelled",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: printingTestParams{
				name:        "warn_cancelled",
				path:        dlpPrintingWarnPath,
				restriction: restrictionlevel.WarnCancelled,
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

	// Setup test HTTP server.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello DLP client you navigated to ", r.URL.Path)
	}))
	defer server.Close()
	s.Log("Created local HTTP server ", server.URL)

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	// construct URL to test and pass to test and policy
	testurl, err := url.Parse(server.URL)
	if err != nil {
		s.Fatal("Failed to parse url: ", err)
	}
	testurl.Path = path.Join(testurl.Path, s.Param().(printingTestParams).path)

	// Update the policy blob.
	pb := policyBlob.NewBlob()
	if s.Param().(printingTestParams).restriction == restrictionlevel.Allowed {
		pb.AddPolicies(policy.PrintingBlockPolicy(testurl.String()))
	} else if s.Param().(printingTestParams).restriction == restrictionlevel.Blocked {
		pb.AddPolicies(policy.PrintingBlockPolicy(testurl.String()))
	} else {
		pb.AddPolicies(policy.PrintingWarnPolicy(testurl.String()))
	}

	// Update policy.
	if err := policyutil.ServeBlobAndRefresh(ctx, fakeDMS, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
	}

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(printingTestParams).browserType)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	conn, err := br.NewConn(ctx, testurl.String())
	if err != nil {
		s.Fatal("Failed to open page: ", err)
	}
	defer conn.Close()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+s.Param().(printingTestParams).name)

	// Type the shortcut.
	if err := keyboard.Accel(ctx, "Ctrl+P"); err != nil {
		s.Fatal("Failed to type printing hotkey: ", err)
	}
	if err := webutil.WaitForQuiescence(ctx, conn, 3*time.Second); err != nil {
		s.Fatalf("Failed to wait for %q to achieve quiescence: %v", testurl, err)
	}

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
