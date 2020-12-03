// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/showpassword"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LockScreenShowPassword,
		Desc:         "Test to view/ hide password on lockscreen using the Show/ Hide password button",
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

// LockScreenShowPassword tests the functionality of Show password button to view the password and
// Hide password button to hide it on lockscreen.
func LockScreenShowPassword(ctx context.Context, s *testing.State) {
	const (
		username = "testuser@gmail.com"
		password = "good"
		PIN      = "1234567890"
	)

	// Login to the user account.
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

	// Check the password field.
	// Lock the screen.
	if err := lockscreen.Lock(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
		s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
	}

	// Verify Show password and Hide password button on password field.
	s.Log("Check the password field")
	if err := showpassword.ShowAndHidePassword(ctx, tconn, s, username, password, false); err != nil {
		s.Fatal("Failed to verify the password field: ", err)
	}

	if err := lockscreen.SubmitPIN(ctx, tconn); err != nil {
		s.Fatal("Failed to submit PIN: ", err)
	}

	// Set up PIN through a connection to the Settings page with autosubmit disabled.
	if err := setupPIN(ctx, tconn, s, cr, password, PIN, false); err != nil {
		s.Fatal("Failed to set up PIN through a connection to the Settings page: ", err)
	}

	// Lock the screen.
	if err := lockscreen.Lock(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
		s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
	}

	// Verify Show password and Hide password button on 'pin or password' field using the user password.
	s.Log("Verify 'pin or password' field")
	if err := showpassword.ShowAndHidePassword(ctx, tconn, s, username, password, false); err != nil {
		s.Fatal("Failed to verify 'pin or password' field using the user password: ", err)
	}

	if err := lockscreen.SubmitPIN(ctx, tconn); err != nil {
		s.Fatal("Failed to submit PIN: ", err)
	}

	// Set up PIN through a connection to the Settings page with autosubmit enabled.
	// With auto submit enabled, Show password button appears only for the password field.

	// Goto settings, enable pin and auto submit.
	if err := setupPIN(ctx, tconn, s, cr, password, PIN, true); err != nil {
		s.Fatal("Failed to set up PIN through a connection to the Settings page: ", err)
	}

	// Lock the screen.
	if err := lockscreen.Lock(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
		s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
	}

	// Click the 'Switch to Password' button to view the password field.
	s.Log("Clicking Switch to password button")
	if err := lockscreen.SwitchToPassword(ctx, tconn); err != nil {
		s.Fatal("Failed to click 'Switch to password' button: ", err)
	}

	// Verify Show password and Hide password button on password field after clicking 'Switch to Password'button.
	s.Log("Check the password field after 'Switch to Password'")
	if err := showpassword.ShowAndHidePassword(ctx, tconn, s, username, password, false); err != nil {
		s.Fatal("Failed to verify the password field after 'Switch to Password': ", err)
	}
}

// setupPIN is a utility function to launch settings app and perform pin set up.
func setupPIN(ctx context.Context, tconn *chrome.TestConn, s *testing.State, cr *chrome.Chrome, password, PIN string, autosubmit bool) error {

	// Open Settings window and set up PIN through a connection to the Settings page.
	settings, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Settings app: ", err)
	}
	// Perform pin set up here.
	if err := settings.EnablePINUnlock(cr, password, PIN, autosubmit)(ctx); err != nil {
		s.Fatal("Failed to enable PIN unlock: ", err)
	}
	return nil
}
