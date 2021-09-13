// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kerberos

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
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
		Func: AutomaticTicketAccessWebsite,
		Desc: "Checks the behavior of accessing website secured with kerberos using KerberosAccount policy",
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

func AutomaticTicketAccessWebsite(ctx context.Context, s *testing.State) {
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

	// Update policies.
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{
		&policy.KerberosEnabled{Val: true},
		&policy.AuthServerAllowlist{Val: config.ServerAllowlist},
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
	s.Log(ctx, "Waiting for Kerberos ticket to appear")
	// Fetching ticket using KerberosAccount policy displays domain in capital
	// letters. Hence using NameStartingWith(name) rather Name(name@domain).
	if err := ui.WaitUntilExists(nodewith.NameStartingWith(username).Role(role.StaticText))(ctx); err != nil {
		s.Fatal("Failed to find Kerberos: ", err)
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

	// The website does not have a valid certificate. We accept the warning and
	// proceed to the content.
	clickAdvance := fmt.Sprintf("document.getElementById(%q).click()", "details-button")
	if err := conn.Eval(ctx, clickAdvance, nil); err != nil {
		s.Fatal("Failed to click Advance button: ", err)
	}

	clickProceed := fmt.Sprintf("document.getElementById(%q).click()", "proceed-link")
	if err := conn.Eval(ctx, clickProceed, nil); err != nil {
		s.Fatal("Failed to click Advance button: ", err)
	}

	//Required as after last click we need to wait for completion. Otherwise
	// test check the title of already loaded page.
	if conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		s.Error("Failed waiting for URL to load: ", err)
	}

	s.Log("Wait for website to have non-empty title")
	var websiteTitle string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conn.Eval(ctx, "document.title", &websiteTitle); err != nil {
			return errors.Wrap(err, "failed to get the website title")
		}
		if websiteTitle == "" {
			return errors.New("website title is still empty")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  5 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		s.Error("Couldn't get non-empty website title: ", err)
	}

	if !strings.Contains(websiteTitle, "KerberosTest") {
		s.Error("Website title was not KerberosTest but ", websiteTitle)
	}
}
