// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/kerberos"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: KerberosWebsite,
		Desc: "Checks the behavior of accessing website secured with kerberos",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"alexanderhartl@google.com",
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func KerberosWebsite(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_kerberos")

	// Perform cleanup.
	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		s.Fatal("Failed to clean up: ", err)
	}

	// Update policies.
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{
		&policy.KerberosEnabled{Val: true},
		&policy.AuthServerAllowlist{Val: kerberos.ServerAllowList},
	}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	conn, err := cr.NewConn(ctx, kerberos.WebsiteAddress)
	if err != nil {
		s.Fatal("Failed to connect to chrome: ", err)
	}
	defer conn.Close()
	// TODO: Accept broken certificate

	_, err = apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/kerberos")
	if err != nil {
		s.Fatal("Could not open kerberos section in OS settings: ", err)
	}
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get a keyboard")
	}
	defer keyboard.Close()

	ui := uiauto.New(tconn)
	if err := uiauto.Combine("add Kerberos ticket",
		ui.LeftClick(nodewith.Name("Kerberos tickets").Role(role.Link)),
		ui.LeftClick(nodewith.Name("Add a ticket").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Kerberos username").Role(role.TextField)),
		keyboard.TypeAction(kerberos.KerberosUser),
		ui.LeftClick(nodewith.Name("Password").Role(role.TextField)),
		keyboard.TypeAction(kerberos.KerberosUserPass),
		ui.LeftClick(nodewith.Name("Add").HasClass("action-button")),
		ui.EnsureGoneFor(nodewith.Name("Add a ticket").Role(role.Button), 30*time.Second), // This does not work well yet causing this test to produce false negative.
	)(ctx); err != nil {
		s.Fatal("Failed to add Kerberos ticket: ", err)
	}

	testing.ContextLog(ctx, "Waiting for Kerberos ticket to appear")
	if err := ui.WithTimeout(60 * time.Second).WaitUntilExists(nodewith.Name(kerberos.KerberosUser).Role(role.StaticText))(ctx); err != nil {
		s.Fatal("Failed to see added Kerberos: ", err)
	}

	// Check that ticket is not expired.
	if err := ui.Exists(nodewith.Name("Expired").Role(role.StaticText))(ctx); err == nil {
		s.Fatal("Kerberos ticket is expired")
	}

	if err := conn.Navigate(ctx, kerberos.WebsiteAddress); err != nil {
		s.Fatalf("Failed to navigate to the server URL %q: %v", kerberos.WebsiteAddress, err)
	}
}
