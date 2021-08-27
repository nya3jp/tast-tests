// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kerberos

import (
	"context"
	"strings"
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
		Func: AccountAccessWebsite,
		Desc: "Checks the behavior of accessing website secured with kerberos using Kerberos account configuration",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"alexanderhartl@google.com",
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"kerberos.username", "kerberos.password", "kerberos.domain"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func AccountAccessWebsite(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS
	username := s.RequiredVar("kerberos.username")
	password := s.RequiredVar("kerberos.password")
	domain := s.RequiredVar("kerberos.domain")
	config := kerberos.ConstructConfig(domain, username)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Perform cleanup.
	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		s.Fatal("Failed to clean up: ", err)
	}

	kerberosAcc := policy.KerberosAccountsValue{
		Principal: config.KerberosAccount,
		Password:  password,
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
	// Fetching ticket using Kerberos account diplays domain in capital letters.
	if err := ui.WithTimeout(30 * time.Second).WaitUntilExists(nodewith.NameStartingWith(username).Role(role.StaticText))(ctx); err != nil {
		s.Fatal("Failed to see added Kerberos: ", err)
	}

	// Check that ticket is active.
	if err := ui.Exists(nodewith.Name("Active").Role(role.StaticText))(ctx); err != nil {
		s.Fatal("Kerberos ticket was not in Active state")
	}

	conn, err := cr.NewConn(ctx, config.WebsiteAddress)
	if err != nil {
		s.Fatal("Failed to connect to chrome: ", err)
	}
	defer conn.Close()

	var websiteTitle string
	if err := conn.Eval(ctx, "document.title", &websiteTitle); err != nil {
		s.Error("Failed to get the website title: ", err)
	}
	if !strings.Contains(websiteTitle, "KerberosTest") {
		s.Error("Website title was not KerberosTest but ", websiteTitle)
	}
}
