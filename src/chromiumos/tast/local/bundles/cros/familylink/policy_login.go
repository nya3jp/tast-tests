// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package familylink

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/familylink"
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
		Func:         PolicyLogin,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks if Unicorn login with policy setup is working",
		Contacts:     []string{"xiqiruan@chromium.org", "cros-families-eng+test@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Val:     browser.TypeAsh,
			Fixture: "familyLinkUnicornPolicyLogin",
		}, {
			Name:    "lacros",
			Val:     browser.TypeLacros,
			Fixture: "familyLinkUnicornPolicyLoginWithLacros",
		}},
	})
}

func PolicyLogin(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*familylink.FixtData).FakeDMS
	cr := s.FixtValue().(*familylink.FixtData).Chrome
	tconn := s.FixtValue().(*familylink.FixtData).TestConn
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	// The PasswordManagerEnabled policy is arbitrarily chosen just to illustrate
	// that setting policies works for Family Link users.
	policies := []policy.Policy{
		&policy.PasswordManagerEnabled{Val: true},
	}

	pb := policy.NewBlob()
	pb.PolicyUser = s.FixtValue().(*familylink.FixtData).PolicyUser
	pb.AddPolicies(policies)

	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve policies: ", err)
	}

	s.Log("Verifying policies delivered to device")
	if err := policyutil.Verify(ctx, tconn, policies); err != nil {
		s.Fatal("Failed to verify policies: ", err)
	}

	s.Log("Verifying policies enforced by browser")
	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Set up browser.
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to set up browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	// Ensure policy is enforced by browser.
	if err := policyutil.SettingsPage(ctx, cr, br, "passwords").
		SelectNode(ctx, nodewith.
			Name("Offer to save passwords").
			Role(role.ToggleButton)).
		Restriction(restriction.Disabled).
		Checked(checked.True).
		Verify(); err != nil {
		s.Error("Unexpected settings state: ", err)
	}
}
