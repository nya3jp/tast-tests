// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kerberos

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/kerberos"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ManualTicketAccessWebsite,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks the behavior of accessing website secured by Kerberos after adding Kerberos ticket",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"alexanderhartl@google.com",
			"chromeos-commercial-identity@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"kerberos.username", "kerberos.password", "kerberos.domain"},
		Fixture:      fixture.FakeDMS,
	})
}

func ManualTicketAccessWebsite(ctx context.Context, s *testing.State) {
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

	conn, err := cr.NewConn(ctx, config.WebsiteAddress)
	if err != nil {
		s.Fatal("Failed to connect to chrome: ", err)
	}
	defer conn.Close()

	// The website does not have a valid certificate. We accept the warning and
	// proceed to the content.
	clickAdvance := fmt.Sprintf("document.getElementById(%q).click()", "details-button")
	clickProceed := fmt.Sprintf("document.getElementById(%q).click()", "proceed-link")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conn.Eval(ctx, clickAdvance, nil); err != nil {
			return errors.Wrap(err, "failed to click Advance button")
		}

		if err := conn.Eval(ctx, clickProceed, nil); err != nil {
			return errors.Wrap(err, "failed to click Proceed button")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  5 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		s.Fatal("Could not accept the certificate warning: ", err)
	}

	// Check that title is 401 - unauthorized.
	var websiteTitle string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conn.Eval(ctx, "document.title", &websiteTitle); err != nil {
			return errors.Wrap(err, "failed to get the website title")
		}
		if websiteTitle == "" {
			return errors.New("website title is empty")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  5 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		s.Fatal("Couldn't get non-empty website title: ", err)
	}

	if !strings.Contains(websiteTitle, "401") {
		s.Fatal("Website title did not contain error 401")
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

	// Add a Kerberos ticket.
	if err := kerberos.AddTicket(ctx, cr, tconn, ui, keyboard, config, password); err != nil {
		s.Fatal("Failed to add Kerberos ticket: ", err)
	}

	s.Log("Wait for website to have non-empty title")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conn.Navigate(ctx, config.WebsiteAddress); err != nil {
			s.Fatalf("Failed to navigate to the server URL %q: %v", config.WebsiteAddress, err)
		}

		if err := conn.Eval(ctx, "document.title", &websiteTitle); err != nil {
			return errors.Wrap(err, "failed to get the website title")
		}
		if websiteTitle == "" || strings.Contains(websiteTitle, "401") {
			return errors.New("website title is empty")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  5 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil && websiteTitle == "" {
		s.Fatal("Couldn't get non-empty website title: ", err)
	}

	if !strings.Contains(websiteTitle, "KerberosTest") {
		s.Error("Website title was not KerberosTest but ", websiteTitle)
	}
}
