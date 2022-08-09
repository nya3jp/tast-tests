// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kerberos

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/kerberos"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ManualTicketRememberPassword,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks if the remember password feature is working properly",
		Contacts: []string{
			"slutskii@google.com",
			"chromeos-commercial-identity@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"kerberos.username", "kerberos.password", "kerberos.domain"},
		Fixture:      fixture.FakeDMS,
	})
}

func ManualTicketRememberPassword(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)
	username := s.RequiredVar("kerberos.username")
	password := s.RequiredVar("kerberos.password")
	domain := s.RequiredVar("kerberos.domain")
	config := kerberos.ConstructConfig(domain, username)

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment())
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	defer func(ctx context.Context) {
		// Use cr as a reference to close the last started Chrome instance.
		if err := cr.Close(ctx); err != nil {
			s.Error("Failed to close Chrome connection: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_manual_ticket")

	// Set Kerberos configuration.
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{
		&policy.KerberosEnabled{Val: true},
		&policy.AuthServerAllowlist{Val: config.ServerAllowlist},
	}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get a keyboard")
	}
	defer keyboard.Close()

	// Change the keyboard layout to English(US). See crbug.com/1351417.
	// If layout is already English(US), which is true for most of the cases,
	// nothing happens.
	ime.EnglishUS.InstallAndActivate(tconn)(ctx)

	ui := uiauto.New(tconn)

	if _, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/kerberos"); err != nil {
		s.Fatal(err, "could not open kerberos section in OS settings")
	}

	// Add a Kerberos ticket.
	if err := uiauto.Combine("add Kerberos ticket",
		ui.LeftClick(nodewith.Name("Kerberos tickets").Role(role.Link)),
		ui.LeftClick(nodewith.Name("Add a ticket").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Kerberos username").Role(role.TextField)),
		keyboard.TypeAction(config.KerberosAccount),
		ui.LeftClick(nodewith.Name("Password").Role(role.TextField)),
		keyboard.TypeAction(password),
		ui.LeftClick(nodewith.Name("Remember password").Role(role.CheckBox)),
		ui.LeftClick(nodewith.Name("Advanced").Role(role.Link)),
		ui.LeftClick(nodewith.Role(role.TextField).State(state.Editable, true).State(state.Multiline, true)),
		keyboard.TypeAction(config.RealmsConfig),
		ui.LeftClick(nodewith.Name("Save").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Add").HasClass("action-button")),
	)(ctx); err != nil {
		s.Fatal(err, "failed to add Kerberos ticket")
	}

	// Trying to find an active ticket.
	if err := kerberos.CheckForTicket(ctx, ui, config); err != nil {
		s.Fatal(err, "failed to find active ticket")
	}

	// Refresh the Kerberos ticket using "remember password" feature.
	if err := uiauto.Combine("refresh Kerberos ticket",
		ui.LeftClick(nodewith.HasClass("icon-more-vert more-actions").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Refresh now").Role(role.MenuItem)),
		ui.WaitUntilExists(nodewith.Name("Remember password").Role(role.CheckBox).Attribute("checked", "true")),
		ui.LeftClick(nodewith.Name("Refresh").HasClass("action-button")),
	)(ctx); err != nil {
		s.Fatal(err, "failed to refresh ticket")
	}

	// Trying to find an active ticket.
	if err := kerberos.CheckForTicket(ctx, ui, config); err != nil {
		s.Fatal(err, "failed to find active ticket")
	}

}
