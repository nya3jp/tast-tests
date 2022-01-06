// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

type testParams struct {
	EnablePIN  bool
	Autosubmit bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShowPasswordLoginScreen,
		Desc:         "Test Show/Hide password functionality on Password field and \"PIN or password\" field of login screen",
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Params: []testing.Param{{
			Val: testParams{false, false},
		}, {
			Name: "pin",
			Val:  testParams{true, false},
		}, {
			Name: "switch_to_password",
			Val:  testParams{true, true},
		}},
	})
}

// ShowPasswordLoginScreen tests viewing PIN / Password on login screen using the "Show password" button and that it goes hidden using the "Hide password" button.
func ShowPasswordLoginScreen(ctx context.Context, s *testing.State) {
	const (
		PIN = "1234567890"
	)
	enablePIN := s.Param().(testParams).EnablePIN
	autosubmit := s.Param().(testParams).Autosubmit
	var creds chrome.Creds

	// Log in and log out to create a user pod on the login screen.
	func() {
		cr, err := chrome.New(ctx)
		if err != nil {
			s.Fatal("Chrome login failed: ", err)
		}
		defer cr.Close(ctx)
		creds = cr.Creds()

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
			if err := settings.EnablePINUnlock(cr, creds.Pass, PIN, autosubmit)(ctx); err != nil {
				s.Fatal("Failed to enable PIN unlock: ", err)
			}
		}

		if err := upstart.RestartJob(ctx, "ui"); err != nil {
			s.Fatal("Failed to restart ui: ", err)
		}
	}()

	// chrome.NoLogin() and chrome.KeepState() are needed to show the login screen with a user pod (instead of the OOBE login screen).
	cr, err := chrome.New(ctx,
		chrome.ExtraArgs("--skip-force-online-signin-for-testing"),
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Wait for the login screen to be ready for PIN / Password entry.
	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.ReadyForPassword }, 30*time.Second); err != nil {
		s.Fatalf("Failed waiting for the login screen to be ready for PIN / Password entry: %v, last state: %+v", err, st)
	}

	// Clicking the "Switch to password" button to view the Password field when PIN autosubmit is enabled
	if autosubmit {
		if err := lockscreen.SwitchToPassword(ctx, tconn); err != nil {
			s.Fatal("Failed to click the Switch to password button: ", err)
		}
	}

	// Test the working of "Show password" and "Hide password" button on login screen.
	if enablePIN && !autosubmit {
		if err := showpassword.ShowAndHidePIN(ctx, tconn, creds.User, PIN); err != nil {
			s.Fatal("Failed to Show/Hide PIN on login screen: ", err)
		}
	} else {
		if err := showpassword.ShowAndHidePassword(ctx, tconn, creds.User, creds.Pass); err != nil {
			s.Fatal("Failed to Show/Hide Password on login screen: ", err)
		}
	}
}
