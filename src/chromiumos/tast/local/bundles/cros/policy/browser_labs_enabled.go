// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
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
		Func:         BrowserLabsEnabled,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of BrowserLabsEnabled policy,checking the existence of the experimental features icon in the toolbar after setting the policy",
		Contacts: []string{
			"samicolon@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedInFeatureChromeLabs,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraAttr:         []string{"informational"},
			Fixture:           fixture.LacrosPolicyLoggedInFeatureChromeLabs,
			Val:               browser.TypeLacros,
		}},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.BrowserLabsEnabled{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func BrowserLabsEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve 10 seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Create a uiauto.Context with default timeout.
	ui := uiauto.New(tconn)

	for _, param := range []struct {
		name            string
		iconShouldExist bool
		policy          *policy.BrowserLabsEnabled
	}{
		{
			name:            "unset",
			iconShouldExist: true,
			policy:          &policy.BrowserLabsEnabled{Stat: policy.StatusUnset},
		},
		{
			name:            "allow",
			iconShouldExist: true,
			policy:          &policy.BrowserLabsEnabled{Val: true},
		},
		{
			name:            "deny",
			iconShouldExist: false,
			policy:          &policy.BrowserLabsEnabled{Val: false},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
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
			conn, err := br.NewConn(ctx, "chrome://version")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			PopUpButton := nodewith.ClassName("ChromeLabsButton").Role(role.PopUpButton).First()
			if err = ui.WaitUntilExists(PopUpButton)(ctx); err != nil {
				if !strings.Contains(err.Error(), nodewith.ErrNotFound) {
					s.Fatal("Failed to wait for the chrome labs icon: ", err)
				}
				if param.iconShouldExist {
					s.Error("Chrome labs icon not found: ", err)
				}
			} else if !param.iconShouldExist {
				s.Error("Unexpected Chrome labs icon found: ", err)
			}
		})
	}
}
