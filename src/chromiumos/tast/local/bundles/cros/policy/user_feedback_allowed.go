// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
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

func init() {
	testing.AddTest(&testing.Test{
		Func:         UserFeedbackAllowed,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Behavior of UserFeedbackAllowed policy on both Ash and Lacros browser",
		Contacts: []string{
			"crisguerrero@chromium.org", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
	})
}

func UserFeedbackAllowed(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	uia := uiauto.New(tconn)

	// Get virtual keyboard to test key combination behavior.
	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	for _, param := range []struct {
		name             string                      // subtest name.
		value            *policy.UserFeedbackAllowed // policy value.
		wantReportOption bool                        // expected result.
	}{
		{
			name:             "allow",
			value:            &policy.UserFeedbackAllowed{Val: true},
			wantReportOption: true,
		},
		{
			name:             "deny",
			value:            &policy.UserFeedbackAllowed{Val: false},
			wantReportOption: false,
		},
		{
			name:             "unset",
			value:            &policy.UserFeedbackAllowed{Stat: policy.StatusUnset},
			wantReportOption: true,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			// Open Chrome to run test.
			conn, err := br.NewConn(ctx, "")
			if err != nil {
				s.Fatal("Failed to connect to the browser: ", err)
			}
			defer conn.Close()

			// 2 seconds should be enough time to wait to make sure a node appears or not
			waitTimeout := 2 * time.Second

			// The popup to send feedback to Google is opened in two ways: 1) Key
			// combination (Alt+Shift+I); 2) From the menu (Chrome Menu > Help >
			// Report an Issue). We test the policy affects both. For 1) we check that
			// the popup appears (or not) when pressing the key combination. For 2) it
			// is enough to check that the option "Report an issue" is available
			// (or not) in the Help menu.

			// Test Key combination. Check if the popup appears (or not) when Alt+Shift+I
			// is pressed.

			// Press key combination to Send feedback to Google
			if err := keyboard.Accel(ctx, "Alt+Shift+I"); err != nil {
				s.Fatal("Failed to press Alt+Shift+I: ", err)
			}

			// Availability of report window popup should match wantReportOption.
			feedbackWindow := nodewith.Name("Feedback").Role(role.RootWebArea)
			if param.wantReportOption {
				if err := uia.WithTimeout(waitTimeout).WaitUntilExists(feedbackWindow)(ctx); err != nil {
					s.Fatal("Failed to wait for Feedback window: ", err)
				}
			} else {
				if err := uia.EnsureGoneFor(feedbackWindow, waitTimeout)(ctx); err != nil {
					s.Fatal("Failed to make sure no Feedback window popup: ", err)
				}
			}

			// Test menu access. It is enough to check if the option "Report an issue"
			// is available (or not) in the Help menu.
			browserAppMenuButtonFinder := nodewith.ClassName("BrowserAppMenuButton").Role(role.PopUpButton)
			helpMenuItemFinder := nodewith.ClassName("MenuItemView").Name("Help")
			if err := uiauto.Combine("Open Help option from Chrome browser menu",
				uia.LeftClick(browserAppMenuButtonFinder),
				uia.LeftClick(helpMenuItemFinder),
			)(ctx); err != nil {
				s.Fatal("Failed to open Help option from Chrome browser menu: ", err)
			}

			// Availability of the report option in the menu should match wantReportOption.
			reportAnIssueFinder := nodewith.ClassName("MenuItemView").Name("Report an issue… Alt+Shift+I")
			if param.wantReportOption {
				if err := uia.WithTimeout(waitTimeout).WaitUntilExists(reportAnIssueFinder)(ctx); err != nil {
					s.Fatal("Failed to find the Report option: ", err)
				}
			} else {
				if err := uia.EnsureGoneFor(reportAnIssueFinder, waitTimeout)(ctx); err != nil {
					s.Fatal("Failed to make sure no Report option is available: ", err)
				}
			}
		})
	}
}
