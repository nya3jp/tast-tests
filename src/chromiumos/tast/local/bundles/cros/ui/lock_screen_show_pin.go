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
		Func:         LockScreenShowPin,
		Desc:         "Test to view/ hide pin on lockscreen using the Show/ Hide password button",
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

// LockScreenShowPin tests viewing pin on lockscreen using Show password button and it goes
// hidden using Hide Password button.
func LockScreenShowPin(ctx context.Context, s *testing.State) {
	const (
		username = "testuser@gmail.com"
		password = "good"
		PIN      = "1234567890"
	)

	// Login to user account.
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

	// Set up PIN through a connection to the Settings page with autosubmit disabled.
	// Open Settings window and set up PIN through a connection to the Settings page.
	settings, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Settings app: ", err)
	}
	// Perform pin set up here.
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

	// Verify Show password and Hide password button on 'pin or password' field using the pin value.
	s.Log("Verify 'pin or password' field")
	if err := showpassword.ShowAndHidePassword(ctx, tconn, s, username, PIN, true); err != nil {
		s.Fatal("Failed to verify 'pin or password' field using the pin value: ", err)
	}
}
