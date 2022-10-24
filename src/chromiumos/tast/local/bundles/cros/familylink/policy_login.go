// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package familylink

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PolicyLogin,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks if Unicorn login with policy setup is working",
		Contacts:     []string{"xiqiruan@chromium.org", "cros-families-eng+test@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Fixture: "familyLinkUnicornPolicyLogin",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "familyLinkUnicornPolicyLoginWithLacros",
			Val:               browser.TypeLacros,
		}},
	})
}

func PolicyLogin(ctx context.Context, s *testing.State) {
	// Reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	// tconn := s.FixtValue().(familylink.HasTestConn).TestConn()
	s.Logf("fdms : %#v", fdms)

	// Perform cleanup.
	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		s.Fatal("Failed to clean up: ", err)
	}

	// The ForceGoogleSafeSearch policy is arbitrarily chosen just to illustrate
	// that setting policies works for Family Link users.
	const safeSearchExpected = true
	policies := []policy.Policy{
		&policy.ForceGoogleSafeSearch{Val: safeSearchExpected},
	}
	// Update policies.
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, policies); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	// DELETE: DEBUG: Open the chrome://policy page to see the policies are
	// displayed in a primary browser whether ash-chrome or lacros-chrome.
	conn, _, closeBrowser, err := browserfixt.SetUpWithURL(ctx, cr, s.Param().(browser.Type), "chrome://policy/")
	if err != nil {
		s.Fatal("Failed to set up browser: ", err)
	}
	defer closeBrowser(cleanupCtx)
	defer conn.Close()
	s.Log("DEBUG: Giving a minute to manually check the policy page before exit")
	testing.Sleep(ctx, 1*time.Minute)
}
