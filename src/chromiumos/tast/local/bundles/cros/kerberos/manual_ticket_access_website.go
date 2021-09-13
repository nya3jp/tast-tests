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
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/kerberos"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ManualTicketAccessWebsite,
		Desc: "Checks the behavior of accessing website secured by Kerberos after adding Kerberos ticket",
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

func ManualTicketAccessWebsite(ctx context.Context, s *testing.State) {
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

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_manual_ticket")

	// Perform cleanup.
	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		s.Fatal("Failed to clean up: ", err)
	}

	// Set Kerberos configuration.
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{
		&policy.KerberosEnabled{Val: true},
		&policy.AuthServerAllowlist{Val: config.ServerAllowlist},
	}); err != nil {
		s.Fatal("Failed to update policies: ", err)
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

	// Check that title is 401 - unauthorized.
	var websiteTitle string
	if err := conn.Eval(ctx, "document.title", &websiteTitle); err != nil {
		s.Error("Failed to get the website title: ", err)
	}
	if strings.Contains(websiteTitle, "401") {
		s.Error("Website title did not contain error 401")
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get a keyboard")
	}
	defer keyboard.Close()

	ui := uiauto.New(tconn)
	_, err = apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/kerberos")
	if err != nil {
		s.Fatal("Could not open kerberos section in OS settings: ", err)
	}

	// Add a Kerberos ticket.
	if err := uiauto.Combine("add Kerberos ticket",
		ui.LeftClick(nodewith.Name("Kerberos tickets").Role(role.Link)),
		ui.LeftClick(nodewith.Name("Add a ticket").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Kerberos username").Role(role.TextField)),
		keyboard.TypeAction(config.KerberosAccount),
		ui.LeftClick(nodewith.Name("Password").Role(role.TextField)),
		keyboard.TypeAction(password),
		ui.LeftClick(nodewith.Name("Add").HasClass("action-button")),
	)(ctx); err != nil {
		s.Fatal("Failed to add Kerberos ticket: ", err)
	}

	// Wait for ticket to appear.
	s.Log(ctx, "Waiting for Kerberos ticket to appear")
	if err := ui.WaitUntilExists(nodewith.Name(config.KerberosAccount).Role(role.StaticText))(ctx); err != nil {
		s.Fatal("Failed to see added Kerberos: ", err)
	}

	// TODO: chromium/1249773 change to get "Active" state once the bug is
	// resolved. UI tree is not refreshed for 1 minute.
	// Check that ticket is not expired.
	if err := ui.Exists(nodewith.Name("Expired").Role(role.StaticText))(ctx); err == nil {
		s.Fatal("Kerberos ticket is expired")
	}

	apps.Close(ctx, tconn, apps.Settings.ID)

	// Wait for OS Setting to close otherwise page does not reload.
	if err := ui.WaitUntilGone(nodewith.Name("Settings - Kerberos tickets").Role(role.Window))(ctx); err != nil {
		s.Fatal("Failed to see added Kerberos: ", err)
	}

	if err := conn.Navigate(ctx, config.WebsiteAddress); err != nil {
		s.Fatalf("Failed to navigate to the server URL %q: %v", config.WebsiteAddress, err)
	}

	s.Log("Wait for website to have non-empty title")
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
