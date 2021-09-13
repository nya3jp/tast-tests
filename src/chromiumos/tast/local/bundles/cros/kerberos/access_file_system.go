// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kerberos

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/kerberos"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AccessFileSystem,
		Desc: "Checks the behavior of accessing file system secured with Kerberos after adding Kerberos ticket",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"alexanderhartl@google.com",
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"kerberos.username", "kerberos.password", "kerberos.domain"},
		Fixture:      "fakeDMS",
	})
}

func AccessFileSystem(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)
	username := s.RequiredVar("kerberos.username")
	password := s.RequiredVar("kerberos.password")
	domain := s.RequiredVar("kerberos.domain")
	config := kerberos.ConstructConfig(domain, username)

	pb := fakedms.NewPolicyBlob()
	pb.AddPolicies([]policy.Policy{
		&policy.KerberosEnabled{Val: true},
		&policy.AuthServerAllowlist{Val: config.ServerAllowList},
	})

	if err := fdms.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment(),
	)
	if err != nil {
		s.Fatal("Creating Chrome with deferred login failed: ", err)
	}
	defer cr.Close(ctx)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Get a handle to the input keyboard.
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard handle: ", err)
	}
	defer keyboard.Close()

	ui := uiauto.New(tconn)
	{
		_, err = apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/kerberos")
		if err != nil {
			s.Fatal("Could not open kerberos section in OS settings: ", err)
		}
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
		testing.ContextLog(ctx, "Waiting for Kerberos ticket to appear")
		if err := ui.WithTimeout(30 * time.Second).WaitUntilExists(nodewith.Name(config.KerberosAccount).Role(role.StaticText))(ctx); err != nil {
			s.Fatal("Failed to see added Kerberos: ", err)
		}

		// Check that ticket is not expired.
		if err := ui.Exists(nodewith.Name("Expired").Role(role.StaticText))(ctx); err == nil {
			s.Fatal("Kerberos ticket is expired")
		}
		apps.Close(ctx, tconn, apps.Settings.ID)
	}

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}

	s.Log(ctx, "Mounting SMB share")
	fileShareURLTextBox := nodewith.Name("File share URL").Role(role.TextField)
	if err := uiauto.Combine("Click add SMB file share",
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
	)(ctx); err != nil {
		s.Fatal("Failed to interact with SMB mount: ", err)
	}
}
