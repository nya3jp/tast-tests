// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChangePassword,
		Desc: "Checks cryptohome password change flow",
		Contacts: []string{
			"rsorokin@google.com",
			"cros-oac@google.com",
			"chromeos-sw-engprod@google.com",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
		},
		Attr: []string{"group:mainline", "informational"},
		VarDeps: []string{
			"ui.gaiaPoolDefault",
			"ui.signinProfileTestExtensionManifestKey",
		},
		Timeout: chrome.GAIALoginTimeout + 2*chrome.LoginTimeout + time.Minute,
	})
}

func ChangePassword(ctx context.Context, s *testing.State) {
	var gaiaCreds chrome.Creds
	var fakeCreds chrome.Creds
	// We only need this to get the Gaia creds.
	cr, err := chrome.New(
		ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.DeferLogin())
	if err != nil {
		s.Fatal("Failed to get config: ", err)
	}
	gaiaCreds = cr.Creds()
	cr.Close(ctx)

	fakeCreds = gaiaCreds
	// Add something to the password so when user logs in again - password change would be detected.
	fakeCreds.Pass = "fake" + fakeCreds.Pass
	cr, err = chrome.New(
		ctx,
		chrome.FakeLogin(fakeCreds))
	if err != nil {
		s.Fatal("Failed to create a user: ", err)
	}
	cr.Close(ctx)

	// Isolate the step to leverage `defer` pattern.
	func() {
		cr, err = chrome.New(
			ctx,
			chrome.GAIALogin(gaiaCreds),
			chrome.DontWaitForCryptohome(),
			chrome.KeepState(),
			chrome.RemoveNotification(false), // By default it waits for the user session.
			chrome.DontSkipOOBEAfterLogin())
		if err != nil {
			s.Fatal("Chrome login failed: ", err)
		}
		defer cr.Close(ctx)
		oobeConn, err := cr.WaitForOOBEConnection(ctx)
		if err != nil {
			s.Fatal("Failed to wait for OOBE connection: ", err)
		}
		defer oobeConn.Close()

		if err := oobeConn.WaitForExprFailOnErrWithTimeout(ctx, "!document.querySelector('#gaia-password-changed').hidden", 10*time.Second); err != nil {
			s.Fatal("Failed to wait for the gaia password changed screen: ", err)
		}
		if err := oobeConn.Eval(ctx, fmt.Sprintf("document.querySelector('#gaia-password-changed').$.oldPasswordInput.value = '%s'", fakeCreds.Pass), nil); err != nil {
			s.Fatal("Failed to enter old password: ", err)
		}
		if err := oobeConn.Eval(ctx, "document.querySelector('#gaia-password-changed').$.next.click()", nil); err != nil {
			s.Fatal("Failed to click on next button: ", err)
		}
		if err := cr.WaitForOOBEConnectionToBeDismissed(ctx); err != nil {
			s.Fatal("Failed to wait for OOBE to be dismissed: ", err)
		}
	}()

	// Login again with the updated password.
	cr, err = chrome.New(
		ctx,
		chrome.NoLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
		chrome.KeepState(),
	)
	if err != nil {
		s.Fatal("Chrome start failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting signing test API connection failed: ", err)
	}

	if err = lockscreen.WaitForPasswordField(ctx, tconn, gaiaCreds.User, 10*time.Second); err != nil {
		s.Fatal("Fail to wait for password: ", err)
	}

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get virtual keyboard: ", err)
	}
	defer keyboard.Close()
	if err = lockscreen.EnterPassword(ctx, tconn, gaiaCreds.User, gaiaCreds.Pass, keyboard); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.LoggedIn }, chrome.LoginTimeout); err != nil {
		s.Fatalf("Waiting for login failed: %v (last status %+v)", err, st)
	}
}
