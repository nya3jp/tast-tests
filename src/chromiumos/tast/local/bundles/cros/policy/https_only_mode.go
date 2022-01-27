// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HTTPSOnlyMode,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that the HTTPSOnlyMode policy is properly applied",
		Contacts: []string{
			"jityao@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome", "lacros"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.LacrosPolicyLoggedIn,
	})
}

func HTTPSOnlyMode(ctx context.Context, s *testing.State) {
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
		name            string
		value           *policy.HttpsOnlyMode
		wantRestriction bool // whether the HTTPS only button is enabled
		setChecked      bool // whether to set the HTTPS only button to true
		httpAllowed     bool // whether HTTP connections are allowed
	}{
		{
			name:            "allowed",
			value:           &policy.HttpsOnlyMode{Val: "allowed"},
			wantRestriction: false,
			setChecked:      false,
			httpAllowed:     true,
		},
		{
			name:            "allowed_and_enabled",
			value:           &policy.HttpsOnlyMode{Val: "allowed"},
			wantRestriction: false,
			setChecked:      true,
			httpAllowed:     false,
		},
		{
			name:            "disallowed",
			value:           &policy.HttpsOnlyMode{Val: "disallowed"},
			wantRestriction: true,
			setChecked:      false,
			httpAllowed:     true,
		},
		{
			name:            "unset",
			value:           &policy.HttpsOnlyMode{Stat: policy.StatusUnset},
			wantRestriction: false,
			setChecked:      false,
			httpAllowed:     true,
		},
		{
			name:            "unset_enabled",
			value:           &policy.HttpsOnlyMode{Stat: policy.StatusUnset},
			wantRestriction: false,
			setChecked:      true,
			httpAllowed:     false,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), browser.TypeLacros)
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			conn, err := br.NewConn(ctx, "")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}

			// Open Security settings page.
			if err := conn.Navigate(ctx, "chrome://settings/security"); err != nil {
				s.Fatal("Failed to open Security settings page: ", err)
			}
			defer conn.Close()

			ui := uiauto.New(tconn)

			httpsOnlyButton := nodewith.Name("Always use secure connections").Role(role.ToggleButton)
			if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(httpsOnlyButton)(ctx); err != nil {
				s.Fatal("Failed to find HTTPS only toggle: ", err)
			}

			var info *uiauto.NodeInfo
			if info, err = ui.WithTimeout(5*time.Second).Info(ctx, httpsOnlyButton); err != nil {
				s.Fatal("Failed to get node info for HTTPS only toggle: ", err)
			}

			// Initial state of button should always be unchecked.
			if info.Checked == checked.True {
				b, _ := json.Marshal(info)
				s.Fatalf("Unexpected toggle button checked state, got %s", b)
			}

			isRestricted := info.Restriction == restriction.Disabled
			if param.wantRestriction != isRestricted {
				b, _ := json.Marshal(info)
				s.Fatalf("Unexpected toggle button restricted state, got %s", b)
			}

			if param.setChecked {
				// Toggle HTTPS only mode on.
				if err := ui.WithTimeout(5 * time.Second).DoDefault(httpsOnlyButton)(ctx); err != nil {
					s.Fatal("Could not click on HTTPS only toggle: ", err)
				}
			}

			// Navigate to a HTTP-only site which does not support HTTPS upgrade.
			if err := conn.Navigate(ctx, "http://httpforever.com"); err != nil {
				s.Fatal("Failed to navigate to httpforever.com: ", err)
			}

			continueButton := nodewith.Name("Continue to site").Role(role.Button)

			// Check if Continue to site button exists which indicates that the HTTP connection warning is shown.
			if param.httpAllowed {
				if err := ui.EnsureGoneFor(continueButton, 2*time.Second)(ctx); err != nil {
					s.Error("Continue to site button is visible: ", err)
				}
			} else {
				if err := ui.WaitUntilExists(continueButton)(ctx); err != nil {
					s.Error("Continue to site button not visible: ", err)
				}
			}
		})
	}
}
