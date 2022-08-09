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
		Func:         ManualTicketEditConfiguration,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that changing kerberos config works properly",
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

func ManualTicketEditConfiguration(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)
	username := s.RequiredVar("kerberos.username")
	password := s.RequiredVar("kerberos.password")
	domain := s.RequiredVar("kerberos.domain")
	config := kerberos.ConstructConfig(domain, username)

	// Any value is fine, as long as the number of minutes is not 0
	// and hours number is less than 10, which is the default value.
	var ticketLifetimeConfig = "\nticket_lifetime = 3h37m"
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

	// Enter Kerberos credentials + open the config.
	if err := uiauto.Combine("opening Kerberos configuration",
		ui.LeftClick(nodewith.Name("Kerberos tickets").Role(role.Link)),
		ui.LeftClick(nodewith.Name("Add a ticket").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Kerberos username").Role(role.TextField)),
		keyboard.TypeAction(config.KerberosAccount),
		ui.LeftClick(nodewith.Name("Password").Role(role.TextField)),
		keyboard.TypeAction(password),
		ui.LeftClick(nodewith.Name("Advanced").Role(role.Link)),
	)(ctx); err != nil {
		s.Fatal(err, "failed to open configuration menu")
	}

	configFinder := nodewith.Role(role.TextField).State(state.Editable, true).State(state.Multiline, true)

	// Save the default kerberos configuration, even if it's empty.
	node, err := ui.Info(ctx, configFinder)
	if err != nil {
		s.Fatal(err, "Not able to read the default configuration")
	}
	var defaultConfig = node.Value

	if err := uiauto.Combine("type configuration and Cancel",
		ui.LeftClick(configFinder),
		keyboard.TypeAction(ticketLifetimeConfig),
		ui.LeftClick(nodewith.Name("Cancel").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Advanced").Role(role.Link)),
	)(ctx); err != nil {
		s.Fatal(err, "failed to open configuration menu")
	}

	node, err = ui.Info(ctx, configFinder)
	if err != nil {
		s.Fatal(err, "Not able to read the configuration")
	}

	// Check that the configuration has not changed, since Cancel was pressed.
	if node.Value != defaultConfig {
		s.Fatal("The Cancel button didn't delete new configuration")
	}

	// Create a ticket with custom lifetime and check if it was actually added.
	if err := uiauto.Combine("changing lifetime of Kerberos ticket",
		ui.LeftClick(configFinder),
		keyboard.TypeAction(ticketLifetimeConfig),
		ui.LeftClick(nodewith.Name("Save").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Add").HasClass("action-button")),
		ui.WaitUntilExists(nodewith.NameContaining("Valid for 3 hours").Role(role.StaticText)),
	)(ctx); err != nil {
		s.Fatal(err, "failed to create a ticket with custom lifetime")
	}

	// Check that configuration can not be saved if the syntax is wrong.
	if err := uiauto.Combine("opening configuration menu and adding invalid configuration",
		ui.LeftClick(nodewith.HasClass("icon-more-vert more-actions").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Refresh now").Role(role.MenuItem)),
		ui.LeftClick(nodewith.Name("Advanced").Role(role.Link)),
		ui.LeftClick(configFinder),
		keyboard.TypeAction("\n[realms"), // syntax error, no closing bracket for "["
		ui.LeftClick(nodewith.Name("Save").Role(role.Button)),
		ui.WaitUntilExists(nodewith.NameContaining("syntax error").Role(role.StaticText)),
		ui.LeftClick(nodewith.Name("Cancel").Role(role.Button)),
	)(ctx); err != nil {
		s.Fatal(err, "failed to find syntax error")
	}

	if err := uiauto.Combine("adding invalid configuration",
		ui.LeftClick(nodewith.Name("Advanced").Role(role.Link)),
		ui.LeftClick(configFinder),
		keyboard.TypeAction("\nticket_lifetime: 1337h"), // syntax error, should be "=" instead of ":"
		ui.LeftClick(nodewith.Name("Save").Role(role.Button)),
		ui.WaitUntilExists(nodewith.NameContaining("syntax error").Role(role.StaticText)),
		ui.LeftClick(nodewith.Name("Cancel").Role(role.Button)),
	)(ctx); err != nil {
		s.Fatal(err, "failed to find syntax error")
	}

	// Check that configuration can not be saved if some options are blocklisted.
	if err := uiauto.Combine("adding invalid configuration",
		ui.LeftClick(nodewith.Name("Advanced").Role(role.Link)),
		ui.LeftClick(configFinder),
		keyboard.TypeAction("\nallow_weak_crypto = true"), // "allow_weak_crypto = true" is blocklisted
		ui.LeftClick(nodewith.Name("Save").Role(role.Button)),
		ui.WaitUntilExists(nodewith.NameContaining("option not supported").Role(role.StaticText)),
	)(ctx); err != nil {
		s.Fatal(err, "failed to find blocklist error")
	}
}
