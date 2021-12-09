// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lockscreen

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/lockscreen/showpassword"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/testing"
)

type testParameters struct {
	EnablePIN  bool
	Autosubmit bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShowPassword,
		Desc:         "Test Show/Hide password functionality on lockscreen Password field and \"PIN or password\" field",
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Val: testParameters{false, false},
		}, {
			Name: "pin",
			Val:  testParameters{true, false},
		}, {
			Name: "switch_to_password",
			Val:  testParameters{true, true},
		}},
	})
}

// ShowPassword tests viewing PIN / Password on lockscreen using the "Show password" button and that it goes hidden using the "Hide password" button.
func ShowPassword(ctx context.Context, s *testing.State) {
	const (
		username = "testuser@gmail.com"
		password = "good"
		PIN      = "1234567890"
	)
	enablePIN := s.Param().(testParameters).EnablePIN
	autosubmit := s.Param().(testParameters).Autosubmit

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

	if enablePIN {
		// Set up PIN through a connection to the Settings page.
		settings, err := ossettings.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch Settings app: ", err)
		}
		s.Log("Performing PIN set up")
		if err := settings.EnablePINUnlock(cr, password, PIN, autosubmit)(ctx); err != nil {
			s.Fatal("Failed to enable PIN unlock: ", err)
		}
	}

	if err := lockscreen.Lock(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
		s.Fatalf("Waiting for the screen to be locked failed: %v (last status %+v)", err, st)
	}

	// Unlock the screen to ensure subsequent tests aren't affected by the screen remaining locked.
	// TODO(b/187794615): Remove once chrome.go has a way to clean up the lock screen state.
	defer func() {
		if err := lockscreen.Unlock(ctx, tconn); err != nil {
			s.Fatal("Failed to unlock the screen: ", err)
		}
	}()

	// Clicking the "Switch to password" button to view the Password field when PIN autosubmit is enabled
	if autosubmit {
		if err := lockscreen.SwitchToPassword(ctx, tconn); err != nil {
			s.Fatal("Failed to click the Switch to password button: ", err)
		}
	}

	// Test the working of "Show password" and "Hide password" button on lockscreen.
	if enablePIN && !autosubmit {
		if err := showpassword.ShowAndHidePIN(ctx, tconn, username, PIN); err != nil {
			s.Fatal("Failed to Show/Hide PIN on lockscreen: ", err)
		}
	} else {
		if err := showpassword.ShowAndHidePassword(ctx, tconn, username, password); err != nil {
			s.Fatal("Failed to Show/Hide Password on lockscreen: ", err)
		}
	}
}
