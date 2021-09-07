// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"

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
		Func:         RemoveExistingUser,
		Desc:         "Remove user pods from start screen",
		Contacts:     []string{"dkuzmin@google.com", "chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
	})
}

const (
	localStatePath = "/home/chronos/Local State"
	knownUsersList = "KnownUsers"
)

func RemoveExistingUser(ctx context.Context, s *testing.State) {
	// // LocalState is a json like structure, from which we will need only LoggedInUsers field.
	type LocalState struct {
		Emails []string `json:"LoggedInUsers"`
	}
	const (
		user1    = "user1@gmail.com"
		user2    = "user2@gmail.com"
		user3    = "user3@gmail.com"
		password = "password"
	)
	createUser := func(creds chrome.Creds, extra_opts ...chrome.Option) {
		opts := append([]chrome.Option{chrome.FakeLogin(creds)}, extra_opts...)
		cr, err := chrome.New(ctx, opts...)
		if err != nil {
			s.Fatal("Chrome login failed: ", err)
		}
		cr.Close(ctx)
	}
	createUser(chrome.Creds{User: user1, Pass: password})
	createUser(chrome.Creds{User: user2, Pass: password}, chrome.KeepState())
	createUser(chrome.Creds{User: user3, Pass: password}, chrome.KeepState())

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
	// Remove user pod by clicking remove button twice.
	ui := uiauto.New(tLoginConn)
	if err := ui.LeftClick(nodewith.Name("Open remove dialog for " + user3).Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to open remove dialog: ", err)
	}
	if err := ui.LeftClick(nodewith.Name("Remove account").Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to click remove account button first time: ", err)
	}
	if err := ui.LeftClick(nodewith.Name("Remove account").Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to click remove account button second time: ", err)
	}
	// Check that user pod was deleted.
	if err := ui.WaitUntilGone(nodewith.Name(user3).Role(role.Button))(ctx); err != nil {
		s.Fatal("Removed user pod is still reachable: ", err)
	}
	// Check that there is no user3 in known users list.
	localStateFile, err := os.Open(localStatePath)
	if err != nil {
		s.Fatal(err, "failed to open Local State file")
	}
	defer localStateFile.Close()
	var localState LocalState
	b, err := ioutil.ReadAll(localStateFile)
	if err != nil {
		s.Fatal(err, "failed to read Local State file contents")
	}
	if err := json.Unmarshal(b, &localState); err != nil {
		s.Fatal(err, "failed to unmarshal Local State")
	}
	knownEmails := make(map[string]bool)
	for _, email := range localState.Emails {
		knownEmails[email] = true
	}
	if knownEmails[user3] {
		s.Fatal("Removed user is still in known users")
	}

	// Restart.
	cr, err = chrome.New(
		ctx,
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	// Update extension connection.
	tLoginConn, err = cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	ui = uiauto.New(tLoginConn)
	// Check that cryptohome for user3 was deleted.
	path, err := cryptohome.UserPath(ctx, user3)
	if _, err := os.Stat(path); err == nil {
		s.Fatal("Cryptohome directory still exists under ", path)
	} else if !os.IsNotExist(err) {
		s.Fatal("Unexpected error: ", err)
	}
	// Check that there is no user pod.
	if err := ui.LeftClick(nodewith.Name("Open remove dialog for " + user3).Role(role.Button))(ctx); err == nil {
		s.Fatal("Remove button for " + user3 + " still exists")
	}
}
