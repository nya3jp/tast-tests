// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RemoveExistingUserPod,
		Desc:         "Remove user pods from start screen",
		Contacts:     []string{"dkuzmin@google.com", "chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
	})
}

func RemoveExistingUserPod(ctx context.Context, s *testing.State) {
	const (
		user1    = "user1@gmail.com"
		user2    = "user2@gmail.com"
		user3    = "user3@gmail.com"
		password = "password"
	)
	func() {
		cr, err := chrome.New(ctx, chrome.FakeLogin(chrome.Creds{User: user1, Pass: password}))
		if err != nil {
			s.Fatal("Chrome login failed: ", err)
		}
		defer cr.Close(ctx)
	}()
	createUser := func(creds chrome.Creds) {
		cr, err := chrome.New(ctx, chrome.FakeLogin(creds), chrome.KeepState())
		if err != nil {
			s.Fatal("Chrome login failed: ", err)
		}
		defer cr.Close(ctx)
	}
	createUser(chrome.Creds{User: user2, Pass: password})
	createUser(chrome.Creds{User: user3, Pass: password})

	// chrome.NoLogin() and chrome.KeepState() are needed to show the login
	// screen with a user pod (instead of the OOBE login screen).
	cr, err := chrome.New(
		ctx,
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tLoginConn)
	defer tLoginConn.Close()

	ui := uiauto.New(tLoginConn)
	if err := ui.LeftClick(nodewith.Name("Open remove dialog for user3@gmail.com").Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to open remove dialog: ", err)
	}
	if err := ui.LeftClick(nodewith.Name("Remove account").Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to click remove account button first time: ", err)
	}
	if err := ui.LeftClick(nodewith.Name("Remove account").Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to click remove account button second time: ", err)
	}

	// Check that cryptohome for user3 was deleted.
	path, err := cryptohome.UserPath(ctx, user3)
	if err != nil {
		s.Fatal("Failed to get user path: ", err)
	}
	if _, err := ioutil.ReadDir(path); err == nil {
		s.Fatal("Cryptohome directory still exists under ", path)
	}
}
