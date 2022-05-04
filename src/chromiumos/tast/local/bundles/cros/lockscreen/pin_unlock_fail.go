// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lockscreen

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PinUnlockFail,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that password field is shown after incorrect PIN was entered multiple times",
		Contacts:     []string{"sherrilin@google.com", "chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func PinUnlockFail(ctx context.Context, s *testing.State) {
	const (
		username = "username@gmail.com"
		password = "password"
		PIN      = "1234567890"
	)

	cr, err := chrome.New(ctx, chrome.FakeLogin(chrome.Creds{User: username, Pass: password}))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Set up PIN through a connection to the Settings page.
	settings, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Settings app: ", err)
	}
	if err := settings.EnablePINUnlock(cr, password, PIN, false)(ctx); err != nil {
		s.Fatal("Failed to enable PIN unlock: ", err)
	}

	// Lock the screen.
	if err := lockscreen.Lock(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}
	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
		s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
	}
	// Unlock the screen to ensure subsequent tests aren't affected by the screen remaining locked.
	// TODO(b/187794615): Remove once chrome.go has a way to clean up the lock screen state.
	defer func() {
		if err := lockscreen.Unlock(ctx, tconn); err != nil {
			s.Fatal("Failed to unlock the screen: ", err)
		}
	}()

	// Enter the wrong PIN to trigger password field
	if err := triggerPasswordField(ctx, tconn, username); err != nil {
		s.Fatal("Failed to trigger password field: ", err)
	}
}

func triggerPasswordField(ctx context.Context, tconn *chrome.TestConn, username string) error {
	const (
		wrongPin     = "0123456789"
		maxNumberTry = 10
	)

	count := 1
	ui := uiauto.New(tconn)
	for count < maxNumberTry && lockscreen.HasPinPad(ctx, tconn) {
		// Enter and submit the PIN to unlock the DUT.
		if err := lockscreen.EnterPIN(ctx, tconn, wrongPin); err != nil {
			return errors.Wrap(err, "failed to enter PIN")
		}
		if err := lockscreen.SubmitPIN(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to submit PIN")
		}

		// Wait to see the Auth error
		authError := lockscreen.AuthErrorFinder
		if count > 1 {
			authError = lockscreen.ConsecutiveAuthErrorFinder
		}
		if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(authError)(ctx); err != nil {
			return errors.Wrap(err, "failed to see the Auth error")
		}
		count++
	}
	if count == maxNumberTry && lockscreen.HasPinPad(ctx, tconn) {
		return errors.New("failed to see the pin pad hidden")
	}
	if err := lockscreen.WaitForPasswordField(ctx, tconn, username, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for password field")
	}

	return nil
}
