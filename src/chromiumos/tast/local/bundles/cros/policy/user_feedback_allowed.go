// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
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
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of UserFeedbackAllowed policy on both Ash and Lacros browser",
		Contacts: []string{
			"crisguerrero@chromium.org", // Test author
		},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraAttr:         []string{"informational"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.UserFeedbackAllowed{}, pci.VerifiedFunctionalityUI),
		},
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
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
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

			// 10 seconds should be enough time to wait to make sure a node appears
			// or not.
			waitTimeout := 10 * time.Second

			// The popup to send feedback to Google is opened in two ways: 1) Key
			// combination (Alt+Shift+I); 2) From the menu (Chrome Menu > Help >
			// Report an Issue). We test the policy affects both. For 1) we check that
			// the popup appears (or not) when pressing the key combination. For 2) it
			// is enough to check that the option "Report an issue" is available
			// (or not) in the Help menu.

			s.Run(ctx, "key_combination", func(ctx context.Context, s *testing.State) {
				defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name+"_key_combination")

				// Check if the popup appears (or not) when Alt+Shift+I is pressed.
				if err := keyboard.Accel(ctx, "Alt+Shift+I"); err != nil {
					s.Fatal("Failed to press Alt+Shift+I: ", err)
				}

				// Availability of report window popup should match wantReportOption.
				feedbackRoot := nodewith.Name("Send feedback to Google").HasClass("RootView")
				if param.wantReportOption {
					if err := uia.WithTimeout(waitTimeout).WaitUntilExists(feedbackRoot)(ctx); err != nil {
						s.Error("Failed to wait for Feedback window: ", err)
					}
					// Close the feedback window to continue the test in a clean state.
					if err := uia.LeftClick(nodewith.Name("Close").Ancestor(feedbackRoot))(ctx); err != nil {
						s.Fatal("Failed to close Feedback window: ", err)
					}
				} else {
					if err := uia.EnsureGoneFor(feedbackRoot, waitTimeout)(ctx); err != nil {
						s.Error("Failed to make sure no Feedback window popup: ", err)
					}
				}
			})

			s.Run(ctx, "menu_access", func(ctx context.Context, s *testing.State) {
				defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name+"_menu_access")

				// Check if the option "Report an issue" is available (or not) in the
				// Help menu, which is accessible from the browser menu.
				browserAppMenuButtonFinder := nodewith.ClassName("BrowserAppMenuButton").Role(role.PopUpButton)
				if err := uia.LeftClick(browserAppMenuButtonFinder)(ctx); err != nil {
					s.Fatal("Failed to open the browser menu: ", err)
				}

				// Ensure the test works despite the screen size.
				helpMenuItemFinder := nodewith.ClassName("MenuItemView").Name("Help")
				if err := uiauto.Combine("Wait and find the Help option",
					uia.WaitUntilExists(helpMenuItemFinder),
					uia.MakeVisible(helpMenuItemFinder),
				)(ctx); err != nil {
					s.Fatal("Failed to find the Help option in the browser menu: ", err)
				}

				if err := uia.LeftClick(helpMenuItemFinder)(ctx); err != nil {
					s.Fatal("Failed to open the Help option from Chrome browser menu: ", err)
				}

				// Availability of the report option in the menu should match wantReportOption.
				reportAnIssueFinder := nodewith.ClassName("MenuItemView").NameContaining("Alt+Shift+I")
				if param.wantReportOption {
					if err := uia.WithTimeout(waitTimeout).WaitUntilExists(reportAnIssueFinder)(ctx); err != nil {
						s.Error("Failed to find the Report option: ", err)
					}
				} else {
					if err := uia.EnsureGoneFor(reportAnIssueFinder, waitTimeout)(ctx); err != nil {
						s.Error("Failed to make sure no Report option is available: ", err)
					}
				}
			})
		})
	}
}
