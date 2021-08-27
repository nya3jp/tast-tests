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
	"chromiumos/tast/local/kerberos"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: KerberosAccountWebsite,
		Desc: "Checks the behavior of accessing website secured with kerberos using Kerberos account configuration",
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

func KerberosAccountWebsite(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_kerberos_account")

	// Perform cleanup.
	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		s.Fatal("Failed to clean up: ", err)
	}

	kerberosAcc := policy.KerberosAccountsValue{
		Principal: kerberos.KerberosUser,
		Password:  kerberos.KerberosUserPass,
		Krb5conf:  []string{},
	}

	// Update policies
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{
		&policy.KerberosEnabled{Val: true},
		&policy.KerberosAccounts{
			Val: []*policy.KerberosAccountsValue{
				&kerberosAcc,
			},
		},
	}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	_, err = apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/kerberos")
	if err != nil {
		s.Fatal("Could not open Kerberos section in OS settings: ", err)
	}

	ui := uiauto.New(tconn)

	// Open Kerberos tickets section.
	if err := ui.LeftClick(nodewith.Name("Kerberos tickets").Role(role.Link))(ctx); err != nil {
		s.Fatal("Failed to open Kerberos tickets section: ", err)
	}

	// Wait for ticket to appear.
	testing.ContextLog(ctx, "Waiting for Kerberos ticket to appear")
	if err := ui.WithTimeout(60 * time.Second).WaitUntilExists(nodewith.Name(kerberos.KerberosUser).Role(role.StaticText))(ctx); err != nil {
		s.Fatal("Failed to see added Kerberos: ", err)
	}

	// Check that ticket is not expired.
	if err := ui.Exists(nodewith.Name("Expired").Role(role.StaticText))(ctx); err == nil {
		s.Fatal("Kerberos ticket is expired")
	}
}
