// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/testexec"
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
		},
		SoftwareDeps: []string{"chrome", "lacros"},
		// Disabled due to <1% pass rate over 30 days. See b/246818601
		//Attr:         []string{"group:mainline", "informational"},
		Fixture: fixture.LacrosPolicyLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.HttpsOnlyMode{}, pci.VerifiedFunctionalityUI),
		},
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

	// Get .local domain alias of localhost.
	out, err := testexec.CommandContext(ctx, "avahi-resolve-address", "127.0.0.1").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("avahi-resolve-address failed: ", err)
	}

	fields := strings.Fields(string(out))
	if len(fields) != 2 {
		s.Fatal("Malformed avahi output: ", string(out))
	}

	// Arbitrary port, does not matter.
	address := fmt.Sprintf("http://%s:%d", fields[1], 12345)

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

			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, browser.TypeLacros)
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

			// Navigate to the HTTP-only site which does not support HTTPS connection upgrade.
			// We cannot use localhost or 127.0.0.1 as the policy does not apply to those hosts.
			if err := conn.Navigate(ctx, address); err != nil {
				s.Fatalf("Failed to navigate to %s: %v", address, err)
			}

			continueButton := nodewith.Name("Continue to site").Role(role.Button)

			// Check if Continue to site button exists which indicates that the HTTP connection warning is shown.
			if param.httpAllowed {
				if err := ui.EnsureGoneFor(continueButton, 10*time.Second)(ctx); err != nil {
					s.Error("Continue to site button is visible: ", err)
				}
			} else {
				if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(continueButton)(ctx); err != nil {
					s.Error("Continue to site button not visible: ", err)
				}
			}
		})
	}
}
