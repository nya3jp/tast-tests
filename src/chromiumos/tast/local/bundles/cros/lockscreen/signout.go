// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lockscreen

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash/ashproc"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/procutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Signout,
		Desc:         "Test signout from the lock screen",
		Contacts:     []string{"rsorokin@google.com", "chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

func Signout(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--force-tablet-mode=clamshell", "--disable-virtual-keyboard"))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test API connection failed: ", err)
	}

	_, err = cr.NewConn(ctx, "chrome://settings")
	if err != nil {
		s.Fatal("Failed to open a tab: ", err)
	}
	_, err = cr.NewConn(ctx, "chrome://os-credits")
	if err != nil {
		s.Fatal("Failed to open a tab: ", err)
	}

	if err := lockscreen.Lock(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}

	oldCrashes, err := crash.GetCrashes(crash.DefaultDirs()...)
	if err != nil {
		s.Fatal("GetCrashes failed: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	oldProc, err := ashproc.Root()
	if err != nil {
		s.Fatal("Failed to get Chrome root PID: ", err)
	}

	ui := uiauto.New(tconn)
	signOutButton := nodewith.Name("Sign out").Role(role.Button)
	buttonFound, err := ui.IsNodeFound(ctx, signOutButton)
	if !buttonFound {
		s.Fatal("Signout button was not found: ", err)
	}

	// We ignore errors here because when we click on "Sign out" button Chrome
	// shuts down and the connection is closed. So we always get an error.
	ui.LeftClick(signOutButton)(ctx)

	// Wait for Chrome restart
	if err := procutil.WaitForTerminated(ctx, oldProc, 30*time.Second); err != nil {
		s.Fatal("Timeout waiting for Chrome to shutdown: ", err)
	}
	if _, err := ashproc.WaitForRoot(ctx, 30*time.Second); err != nil {
		s.Fatal("TImeout waiting for Chrome to restart: ", err)
	}

	newCrashes, err := crash.GetCrashes(crash.DefaultDirs()...)
	if err != nil {
		s.Fatal("GetCrashes failed: ", err)
	}

	if len(oldCrashes) != len(newCrashes) {
		s.Fatal("Chrome crashed during the test")
	}

	// Restart chrome for testing
	if cr, err = chrome.New(ctx, chrome.NoLogin(), chrome.KeepState(), chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey"))); err != nil {
		s.Fatal("Chrome restart for testing failed: ", err)
	}
	tconn, err = cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting signing test API connection failed: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	st, err := lockscreen.GetState(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get login state: ", err)
	}

	if !st.ReadyForPassword {
		s.Fatal("Chrome is not on the login screen: ", err)
	}
}
