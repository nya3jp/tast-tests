// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kerberos

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
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
		Func: AccountAccessFileSystem,
		Desc: "Checks the behavior of accessing file system secured with kerberos",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"alexanderhartl@google.com",
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"kerberos.user", "kerberos.password, kerberos.domain"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func AccountAccessFileSystem(ctx context.Context, s *testing.State) {
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

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_kerberos_file")

	// Perform cleanup.
	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		s.Fatal("Failed to clean up: ", err)
	}

	kerberosAcc := policy.KerberosAccountsValue{
		Principal: username,
		Password:  password,
		Krb5conf:  []string{},
	}

	// Update policies.
	// TODO: Figure out whether to change it to ServerAndRefresh or to change
	// the fixture to enrolledFakeDMS as Alex suggested.
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
		s.Fatal("Could not open kerberos section in OS settings: ", err)
	}

	ui := uiauto.New(tconn)

	if err := ui.LeftClick(nodewith.Name("Kerberos tickets").Role(role.Link))(ctx); err != nil {
		s.Fatal("Failed to open Kerberos tickets section: ", err)
	}

	testing.ContextLog(ctx, "Waiting for Kerberos ticket to appear")
	if err := ui.WithTimeout(60 * time.Second).WaitUntilExists(nodewith.Name(username).Role(role.StaticText))(ctx); err != nil {
		s.Fatal("Failed to see added Kerberos: ", err)
	}

	// Check that ticket is not expired.
	if err := ui.Exists(nodewith.Name("Expired").Role(role.StaticText))(ctx); err == nil {
		s.Fatal("Kerberos ticket is expired")
	}

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}

	// Get a handle to the input keyboard
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard handle: ", err)
	}
	defer keyboard.Close()

	fileShareURLTextBox := nodewith.Name("File share URL").Role(role.TextField)
	if err := uiauto.Combine("Click add SMB file share",
		files.ClickMoreMenuItem("Services", "SMB file share"),
		ui.WaitForLocation(fileShareURLTextBox),
		ui.LeftClick(fileShareURLTextBox),
		keyboard.TypeAction(config.RemoteFileSystemURI),
		keyboard.TypeAction("Enter"),
		ui.WithInterval(500*time.Millisecond).LeftClickUntil(fileShareURLTextBox, ui.Gone(fileShareURLTextBox)),
	)(ctx); err != nil {
		s.Fatal("Failed to click add SMB share: ", err)
	}

	if err := uiauto.Combine("Wait for SMB to mount",
		files.OpenPath("Files - sysvol", config.Folder),
		files.WaitForFile("test.txt"), // TODO: Is there a file to be checked?
	)(ctx); err != nil {
		s.Fatal("Failed to wait for SMB to mount: ", err)
	}

	// TODO open the file.
}
