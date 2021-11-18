// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/ctxutil"
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
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Remove user pods from start screen",
		Contacts:     []string{"dkuzmin@google.com", "chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		Timeout: 5*chrome.LoginTimeout + 25*time.Second,
	})
}

const (
	localStatePath = "/home/chronos/Local State"
	knownUsersList = "KnownUsers"
)

func RemoveExistingUser(ctx context.Context, s *testing.State) {
	// LocalState is a json like structure, from which we will need only LoggedInUsers field.
	type LocalState struct {
		Emails []string `json:"LoggedInUsers"`
	}
	const (
		user1    = "user1@gmail.com"
		user2    = "user2@gmail.com"
		user3    = "user3@gmail.com"
		password = "password"
	)
	cleanUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()
	createUser := func(creds chrome.Creds, extraOpts ...chrome.Option) {
		opts := append([]chrome.Option{chrome.FakeLogin(creds)}, extraOpts...)
		cr, err := chrome.New(ctx, opts...)
		if err != nil {
			s.Fatal("Chrome login failed: ", err)
		}
		cr.Close(cleanUpCtx)
	}
	createUser(chrome.Creds{User: user1, Pass: password})
	createUser(chrome.Creds{User: user2, Pass: password}, chrome.KeepState())
	createUser(chrome.Creds{User: user3, Pass: password}, chrome.KeepState())

	func() {
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
		defer cr.Close(cleanUpCtx)
		// Connect to login extension.
		tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
		if err != nil {
			s.Fatal("Creating login test API connection failed: ", err)
		}
		defer faillog.DumpUITreeOnError(cleanUpCtx, s.OutDir(), s.HasError, tLoginConn)
		ui := uiauto.New(tLoginConn)
		// Wait for user pods to be available.
		if err := ui.WaitUntilExists(nodewith.Name(user3).Role(role.Button))(ctx); err != nil {
			s.Fatal("Failed to wait for user pods to be available: ", err)
		}
		// Remove user pod by clicking remove button twice.
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
	}()
	cr, err := chrome.New(
		ctx,
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanUpCtx)
	// Check that there is no user3 in LoggedInUsers list.
	localStateFile, err := os.Open(localStatePath)
	if err != nil {
		s.Fatal("Failed to open Local State file: ", err)
	}
	defer localStateFile.Close()
	var localState LocalState
	b, err := ioutil.ReadAll(localStateFile)
	if err != nil {
		s.Fatal("Failed to read Local State file contents: ", err)
	}
	if err := json.Unmarshal(b, &localState); err != nil {
		s.Fatal("Failed to unmarshal Local State: ", err)
	}
	knownEmails := make(map[string]bool)
	for _, email := range localState.Emails {
		knownEmails[email] = true
	}
	if knownEmails[user3] {
		s.Fatal("Removed user is still in LoggedInUsers list")
	}
	// Check that cryptohome for user3 was deleted.
	path, err := cryptohome.UserPath(ctx, user3)
	if _, err := os.Stat(path); err == nil {
		s.Fatal("Cryptohome directory still exists under ", path)
	} else if !os.IsNotExist(err) {
		s.Fatal("Unexpected error: ", err)
	}
	// Connect to login extension.
	tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanUpCtx, s.OutDir(), s.HasError, tLoginConn)
	ui := uiauto.New(tLoginConn)
	// Wait for user pods to be available.
	if err := ui.WaitUntilExists(nodewith.Name(user1).Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to wait for user pods to be available after reboot: ", err)
	}
	if err := ui.WaitUntilExists(nodewith.Name(user2).Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to wait for user pods to be available after reboot: ", err)
	}
	// Check that there is no user pod for user3.
	if err := ui.Gone(nodewith.Name(user3).Role(role.Button))(ctx); err != nil {
		s.Fatal("Removed user pod for " + user3 + " still exists")
	}
}
