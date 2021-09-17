// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lockscreen

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/lockscreen/showpassword"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SwitchToPassword,
		Desc:         "Test to view/hide password on lockscreen after clicking the \"Switch to password\" button",
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

// SwitchToPassword tests the functionality of "Show password" button and "Hide password" button on lockscreen when PIN autosubmit is enabled. With autosubmit enabled, "Show password" button appears only for the password field. Clicking "Switch to password" button enables to view the password field.
func SwitchToPassword(ctx context.Context, s *testing.State) {
	const (
		username = "testuser@gmail.com"
		password = "good"
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

	// Set up PIN through a connection to the Settings page and enable autosubmit.
	settings, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Settings app: ", err)
	}

	s.Log("Perform PIN set up and enable autosubmit")
	if err := settings.EnablePINUnlock(cr, password, PIN, true)(ctx); err != nil {
		s.Fatal("Failed to enable PIN unlock: ", err)
	}

	if err := lockscreen.Lock(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
		s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
	}

	// Unlock the screen to ensure subsequent tests aren't affected by the screen remaining locked.
	// TODO(b/187794615): Remove once chrome.go has a way to clean up the lock screen state.
	defer func() {
		if err := lockscreen.UnLock(ctx, tconn); err != nil {
			s.Fatal("Failed to unlock the screen: ", err)
		}
	}()

	s.Log("Clicking \"Switch to password\" button to view the password field")
	if err := lockscreen.SwitchToPassword(ctx, tconn); err != nil {
		s.Fatal("Failed to click \"Switch to password\" button: ", err)
	}

	// Test the functionality of "Show password" and "Hide password" button on lockscreen password field after clicking "Switch to password" button.
	s.Log("Testing \"Show/Hide password\" button on lockscreen password field after clicking \"Switch to password\" button")
	if err := showpassword.ShowAndHidePassword(ctx, tconn, username, password, false); err != nil {
		s.Fatal("Show/Hide password on lockscreen failed after clicking the \"Switch to password\" button: ", err)
	}
}
