// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"time"
	"unicode/utf8"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// Duration of inactivity after which the password input field should be cleared.
// margin of error.
const clearTimeout = 30 * time.Second

func init() {
	testing.AddTest(&testing.Test{
		Func: ClearPasswordAfterInactivity,
		Desc: "Check that that the password input field on the signin screen is cleared after inactivity",
		Contacts: []string{
			"mbid@google.com",
			"cros-lurs@google.com",
			"chromeos-sw-engprod@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Timeout:      2*chrome.LoginTimeout + clearTimeout + time.Minute,
		LacrosStatus: testing.LacrosVariantUnneeded,
	})
}

func ClearPasswordAfterInactivity(ctx context.Context, s *testing.State) {
	cleanUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	// Setup: Create user and save creds.
	creds := func(ctx context.Context) chrome.Creds {
		cr, err := chrome.New(ctx)
		if err != nil {
			s.Fatal("Chrome login failed: ", err)
		}
		defer cr.Close(ctx)
		return cr.Creds()
	}(ctx)

	// chrome.NoLogin() and chrome.KeepState() are needed to show the login screen with a user pod
	// (instead of the OOBE login screen).
	cr, err := chrome.New(ctx,
		chrome.ExtraArgs("--skip-force-online-signin-for-testing"),
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanUpCtx)

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanUpCtx, s.OutDir(), s.HasError, tconn)

	// Wait for the login screen to be ready for password entry.
	readyForPassword := func(st lockscreen.State) bool { return st.ReadyForPassword }
	if _, err := lockscreen.WaitState(ctx, tconn, readyForPassword, 30*time.Second); err != nil {
		s.Fatal("Failed to wait for login screen: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	const partialPassword = "abcd"
	if err := lockscreen.TypePassword(ctx, tconn, creds.User, partialPassword, kb); err != nil {
		s.Fatal("Failed to type password: ", err)
	}

	passwordValue, err := readPasswordValue(ctx, tconn, creds.User)
	if err != nil {
		s.Fatal("Failed to read entered password field: ", err)
	}
	// We have to use RuneCount instead of len because password is concealed and all characters are
	// replaced by the bullet character, which is more than one byte in utf8. We can use `len` for
	// `partialPassword` because it's ASCII.
	if utf8.RuneCountInString(passwordValue) != len(partialPassword) {
		s.Fatal("Failed to verify value of password field: ", passwordValue)
	}

	// Wait until password is cleared. We allow some margin of error.
	const clearTimeoutErrorMargin = 3 * time.Second
	if err := testing.Sleep(ctx, clearTimeout+clearTimeoutErrorMargin); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}
	passwordValue, err = readPasswordValue(ctx, tconn, creds.User)
	if err != nil {
		s.Fatal("Failed to read entered password field: ", err)
	}
	if passwordValue != "" {
		s.Fatal("Password field not cleared after inactivity: ", passwordValue)
	}
}

// readPasswordValue finds the password input field on the lockscreen and reads its value.
func readPasswordValue(ctx context.Context, tconn *chrome.TestConn, user string) (string, error) {
	inputInfo, err := lockscreen.UserPassword(ctx, tconn, user, false /* UsePIN */)
	if err != nil {
		return "", errors.Wrap(err, "failed to find password field")
	}
	return inputInfo.Value, nil
}
