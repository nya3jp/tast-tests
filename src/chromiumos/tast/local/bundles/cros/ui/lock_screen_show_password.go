// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/showpassword"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LockScreenShowPassword,
		Desc:         "Test to view password on lockscreen using the Show password button",
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

// LockScreenShowPassword tests to view password on lockscreen using Show password button
func LockScreenShowPassword(ctx context.Context, s *testing.State) {
	const (
		username = "testuser@gmail.com"
		password = "good"
		gaiaID   = "1234"
		PIN      = "1234567890"
	)

	// Login to user account
	cr, err := chrome.New(ctx, chrome.Auth(username, password, gaiaID))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Check the password only field
	// Lock the screen.
	if err := lockscreen.Lock(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
		s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
	}

	// Verify Show password button on password only field
	if err := showpassword.VerifyPasswordField(ctx, tconn, s, username, password, false); err != nil {
		s.Fatal("Failed to verify password only field: ", err)
	}

	if err := lockscreen.SubmitPIN(ctx, tconn); err != nil {
		s.Fatal("Failed to submit PIN: ", err)
	}

	// Set up PIN through a connection to the Settings page with autosubmit disabled.
	if err := showpassword.SetupPIN(ctx, tconn, s, cr, password, PIN, false); err != nil {
		s.Fatal("Failed to set up PIN through a connection to the Settings page: ", err)
	}

	// Lock the screen.
	if err := lockscreen.Lock(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
		s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
	}

	// Verify Show password button on pin or password field using the user password
	if err := showpassword.VerifyPasswordField(ctx, tconn, s, username, password, false); err != nil {
		s.Fatal("Failed to verify pin or password field using the user password: ", err)
	}

	if err := lockscreen.SubmitPIN(ctx, tconn); err != nil {
		s.Fatal("Failed to submit PIN: ", err)
	}

	/* Set up PIN through a connection to the Settings page with autosubmit enabled.
	With auto submit enabled, Show password button appears only on password field.*/

	//Goto settings, enable pin and auto submit
	if err := showpassword.SetupPIN(ctx, tconn, s, cr, password, PIN, true); err != nil {
		s.Fatal("Failed to set up PIN through a connection to the Settings page: ", err)
	}

	// Lock the screen.
	if err := lockscreen.Lock(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
		s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
	}

	//Click the 'Switch to Password' button to view password only field
	s.Log("Clicking Switch to password button")
	if err := lockscreen.SwitchToPassword(ctx, tconn); err != nil {
		s.Fatal("Failed to click 'Switch to password' button: ", err)
	}

	// Verify Show password button on password only field after 'Switch to Password'
	if err := showpassword.VerifyPasswordField(ctx, tconn, s, username, password, false); err != nil {
		s.Fatal("Failed to verify password only field after 'Switch to Password': ", err)
	}
}
