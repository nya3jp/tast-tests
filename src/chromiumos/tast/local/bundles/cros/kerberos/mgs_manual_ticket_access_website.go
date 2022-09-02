// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kerberos

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/kerberos"
	"chromiumos/tast/local/mgs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MgsManualTicketAccessWebsite,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks if Kerberos is working properly in MGS",
		Contacts: []string{
			"slutskii@google.com",
			"chromeos-commercial-identity@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"kerberos.username", "kerberos.password", "kerberos.domain"},
		Fixture:      fixture.FakeDMSEnrolled,
	})
}

func MgsManualTicketAccessWebsite(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)
	username := s.RequiredVar("kerberos.username")
	password := s.RequiredVar("kerberos.password")
	domain := s.RequiredVar("kerberos.domain")
	config := kerberos.ConstructConfig(domain, username)

	mgs, cr, err := mgs.New(
		ctx,
		fdms,
		mgs.DefaultAccount(),
		mgs.AutoLaunch(mgs.MgsAccountID),
		mgs.AddPublicAccountPolicies(mgs.MgsAccountID, []policy.Policy{&policy.KerberosEnabled{Val: true},
			&policy.AuthServerAllowlist{Val: config.ServerAllowlist}}),
	)

	defer func(ctx context.Context) {
		// Use mgs as a reference to close the last started MGS instance.
		if err := mgs.Close(ctx); err != nil {
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

	conn, err := cr.NewConn(ctx, config.WebsiteAddress)
	if err != nil {
		s.Fatal("Failed to connect to chrome: ", err)
	}
	defer conn.Close()

	// The website does not have a valid certificate. We accept the warning and
	// proceed to the content.
	if err := kerberos.ClickAdvancedAndProceed(ctx, conn); err != nil {
		s.Fatal("Could not accept the certificate warning: ", err)
	}

	// Check that title is 401 - unauthorized.
	var websiteTitle string
	if err := conn.Eval(ctx, "document.title", &websiteTitle); err != nil {
		s.Fatal("Failed to get the website title: ", err)
	}
	if websiteTitle == "" {
		s.Fatal("Website title is empty")
	}
	if !strings.Contains(websiteTitle, "401") {
		s.Fatal("Website title does not contain 401")
	}

	// Access keyboard.
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

	// Add a Kerberos ticket.
	if err := kerberos.AddTicket(ctx, cr, tconn, ui, keyboard, config, password); err != nil {
		s.Fatal("Failed to add Kerberos ticket: ", err)
	}
	// Refresh the website.
	if err := conn.Navigate(ctx, config.WebsiteAddress); err != nil {
		s.Fatalf("Failed to navigate to the server URL %q: %v", config.WebsiteAddress, err)
	}
	// Wait for the website to load.
	if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		s.Fatal("Failed waiting for URL to load: ", err)
	}

	s.Log("Getting the website's title")
	if err := conn.Eval(ctx, "document.title", &websiteTitle); err != nil {
		s.Fatal("Failed to get the website title: ", err)
	}
	if websiteTitle == "" {
		s.Fatal("Website title is empty")
	}
	if strings.Contains(websiteTitle, "401") {
		s.Error("Website title contains 401")
	}
	if !strings.Contains(websiteTitle, "KerberosTest") {
		s.Fatal("Website title was not KerberosTest but ", websiteTitle)
	}
}
