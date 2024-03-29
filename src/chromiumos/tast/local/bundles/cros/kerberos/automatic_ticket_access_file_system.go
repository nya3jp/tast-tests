// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/kerberos"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AutomaticTicketAccessFileSystem,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks the behavior of accessing file system secured with Kerberos using Kerberos account",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"alexanderhartl@google.com",
			"chromeos-commercial-identity@google.com",
		},
		// TODO(crbug/1380920): Remove when this bug is fixed.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel(
			[]string{"rusty", "steelix", "tentacruel", "lazor", "pompom"}...)),
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"kerberos.username", "kerberos.password", "kerberos.domain"},
		Fixture:      fixture.FakeDMS,
	})
}

func AutomaticTicketAccessFileSystem(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)
	username := s.RequiredVar("kerberos.username")
	password := s.RequiredVar("kerberos.password")
	domain := s.RequiredVar("kerberos.domain")
	config := kerberos.ConstructConfig(domain, username)

	kerberosAcc := policy.KerberosAccountsValue{
		Principal: "${LOGIN_ID}" + "@" + domain,
		Password:  "${PASSWORD}",
		Krb5conf:  []string{config.RealmsConfig},
	}

	pb := policy.NewBlob()
	pb.PolicyUser = username + "@managedchrome.com"
	pb.AddPolicies([]policy.Policy{
		&policy.KerberosEnabled{Val: true},
		&policy.KerberosAccounts{
			Val: []policy.KerberosAccountsValueIf{
				&kerberosAcc,
			},
		},
	})

	if err := fdms.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: username + "@managedchrome.com", Pass: password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment(),
	)
	if err != nil {
		s.Fatal("Creating Chrome with deferred login failed: ", err)
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

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_automatic_ticket")

	_, err = apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/kerberos")
	if err != nil {
		s.Fatal("Could not open kerberos section in OS settings: ", err)
	}

	ui := uiauto.New(tconn)

	if err := ui.LeftClick(nodewith.Name("Kerberos tickets").Role(role.Link))(ctx); err != nil {
		s.Fatal("Failed to open Kerberos tickets section: ", err)
	}

	s.Log("Waiting for Kerberos ticket to appear")
	if err := ui.WaitUntilExists(nodewith.NameStartingWith(username).Role(role.StaticText))(ctx); err != nil {
		s.Fatal("Failed to find Kerberos ticket: ", err)
	}

	// Check that ticket is active.
	if err := ui.Exists(nodewith.Name("Active").Role(role.StaticText))(ctx); err != nil {
		s.Fatal("Kerberos ticket is not active: ", err)
	}

	// Close the OS Settings app so the UI locators won't ghost one another.
	apps.Close(ctx, tconn, apps.Settings.ID)

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}

	// Get a handle to the input keyboard.
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard handle: ", err)
	}
	defer keyboard.Close()

	// Change the keyboard layout to English(US). See crbug.com/1351417.
	// If layout is already English(US), which is true for most of the cases,
	// nothing happens.
	ime.EnglishUS.InstallAndActivate(tconn)(ctx)

	s.Log("Mounting SMB share")
	fileShareURLTextBox := nodewith.Name("File share URL").Role(role.TextField)
	if err := uiauto.Combine("Add SMB file share",
		files.ClickMoreMenuItem("Services", "SMB file share"),
		ui.WaitForLocation(fileShareURLTextBox),
		keyboard.TypeAction(config.RemoteFileSystemURI),
		ui.LeftClick(nodewith.Name("Add").HasClass("action-button")),
		ui.WaitUntilGone(fileShareURLTextBox),
	)(ctx); err != nil {
		s.Fatal("Failed to add SMB share: ", err)
	}

	if err := uiauto.Combine("Wait for SMB to mount and open file",
		files.OpenPath("Files - "+config.Folder, config.Folder),
		files.WaitForFile(config.File),
		files.SelectFile(config.File),
	)(ctx); err != nil {
		s.Fatal("Failed to interact with SMB mount: ", err)
	}
}
