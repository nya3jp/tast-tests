// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Offline,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test that user can sign in when device offline ",
		Contacts:     []string{"rsorokin@google.com", "chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Timeout:      2*chrome.LoginTimeout + 25*time.Second,
	})
}

func Offline(ctx context.Context, s *testing.State) {
	var creds chrome.Creds
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	creds = cr.Creds()
	if err = cr.Close(ctx); err != nil {
		s.Fatal("Failed to close Chrome instance: ", err)
	}

	loginAgain := func(ctx context.Context) error {
		cr, err := chrome.New(ctx,
			chrome.ExtraArgs("--skip-force-online-signin-for-testing"),
			chrome.NoLogin(),
			chrome.KeepState(),
			chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))

		if err != nil {
			return errors.Wrap(err, "chrome start failed")
		}
		defer cr.Close(ctx)

		tconn, err := cr.SigninProfileTestAPIConn(ctx)

		if err != nil {
			return errors.Wrap(err, "getting test Signin Profile API connection failed")
		}

		hasErrorVar := true
		hasError := func() bool { return hasErrorVar }
		defer faillog.DumpUITreeOnError(ctx, s.OutDir(), hasError, tconn)

		if err := lockscreen.WaitForPasswordField(ctx, tconn, creds.User, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for the password field")
		}
		keyboard, err := input.VirtualKeyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get virtual keyboard")
		}
		defer keyboard.Close()

		// Enter wrong password
		if err = lockscreen.EnterPassword(ctx, tconn, creds.User, creds.Pass+"fake", keyboard); err != nil {
			return errors.Wrap(err, "failed to enter password")
		}

		if err := lockscreen.WaitForAuthError(ctx, tconn, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for auth error")
		}

		// Enter correct password
		if err = lockscreen.EnterPassword(ctx, tconn, creds.User, creds.Pass, keyboard); err != nil {
			return errors.Wrap(err, "failed to enter password")
		}
		if err := lockscreen.WaitForLoggedIn(ctx, tconn, chrome.LoginTimeout); err != nil {
			s.Fatal("Failed to login: ", err)
		}

		hasErrorVar = false
		return nil
	}

	// Run login in offline mode.
	if err := network.ExecFuncOnChromeOffline(ctx, loginAgain); err != nil {
		s.Fatal("Failed to login offline: ", err)
	}
}
