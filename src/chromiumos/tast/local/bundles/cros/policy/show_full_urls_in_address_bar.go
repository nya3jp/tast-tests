// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"
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
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShowFullUrlsInAddressBar,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of ShowFullUrlsInAddressBar policy on both Ash and Lacros browser",
		Contacts: []string{
			"mpetrisor@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:    "ash",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.ShowFullUrlsInAddressBar{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func ShowFullUrlsInAddressBar(ctx context.Context, s *testing.State) {
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

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value           *policy.ShowFullUrlsInAddressBar
		expectedFullURL bool
	}{
		{
			name:            "true",
			value:           &policy.ShowFullUrlsInAddressBar{Val: true},
			expectedFullURL: true,
		},
		{
			name:            "false",
			value:           &policy.ShowFullUrlsInAddressBar{Val: false},
			expectedFullURL: false,
		},
		{
			name:            "unset",
			value:           &policy.ShowFullUrlsInAddressBar{Stat: policy.StatusUnset},
			expectedFullURL: false,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			// ServeAndVerify() doesn't work and throws "Failed to update policies" when used.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Run actual test.
			conn, err := br.NewConn(ctx, "https://www.google.com")
			if err != nil {
				s.Fatal("Failed to connect to the browser: ", err)
			}
			defer conn.Close()

			// Create a uiauto.Context with default timeout.
			ui := uiauto.New(tconn)

			// Check address bar text.
			addressBarNode := nodewith.Name("Address and search bar").Role(role.TextField)
			if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(addressBarNode)(ctx); err != nil {
				s.Fatal("Failed to find address bar: ", err)
			}

			var addressbarInfo *uiauto.NodeInfo
			if addressbarInfo, err = ui.WithTimeout(5*time.Second).Info(ctx, addressBarNode); err != nil {
				s.Fatal("Failed to get node info for address bar: ", err)
			}

			addressBarText := addressbarInfo.Value
			isFullURL := strings.HasPrefix(addressBarText, "https://www.")

			if isFullURL != param.expectedFullURL {
				s.Errorf("Unexpected policy behavior: got %t; want %t", isFullURL, param.expectedFullURL)
			}

			// Invoke context menu of the search bar.
			if err := ui.RightClick(addressBarNode)(ctx); err != nil {
				s.Fatal("Failed to right click address bar: ", err)
			}

			// The menu item "Always show full URLs" only shows up when the policy is not set and the
			// device is not a managed ChromeOS device. Therefore we check that it is never in the menu.
			if fullURLItemShown := checkAlwaysShowFullURLItem(ctx, ui); fullURLItemShown {
				s.Error("Unexpected shown menu item")
			}
		})
	}
}

func checkAlwaysShowFullURLItem(ctx context.Context, ui *uiauto.Context) bool {
	menuItemNode := nodewith.Role(role.MenuItem).Name("Always show full URLs")
	if err := ui.EnsureGoneFor(menuItemNode, 5*time.Second)(ctx); err != nil {
		// Found menu item.
		return true
	}
	return false
}
