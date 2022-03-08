// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package smartlock contains tests for the Smart Lock feature in Chrome OS.
package smartlock

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/local/chrome/crossdevice/smartlock"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

// TODO(b/227357808) Remove this test when we're ready to remove the signin subfeature of Smart Lock

func init() {
	testing.AddTest(&testing.Test{
		Func:         SettingsSigninEnableDisable,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests ability to enable/disable Sign-in with Smart Lock with Settings",
		Contacts: []string{
			"cclem@google.com",
			"chromeos-sw-engprod@google.com",
			"chromeos-cross-device-eng@google.com",
		},
		Attr:         []string{"group:cross-device"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "crossdeviceOnboardedNoLock",
		Timeout:      8 * time.Minute,
	})
}

// SettingsSigninEnableDisable tests enabling/disabling Signin with Smart Lock in settings.
func SettingsSigninEnableDisable(ctx context.Context, s *testing.State) {
	var err error

	cr := s.FixtValue().(*crossdevice.FixtData).Chrome
	tconn := s.FixtValue().(*crossdevice.FixtData).TestConn
	username := s.FixtValue().(*crossdevice.FixtData).Username
	password := s.FixtValue().(*crossdevice.FixtData).Password

	// This test does a few login/logout cycles. We want all the chrome sessions to have the same setup as the fixture.
	opts := s.FixtValue().(*crossdevice.FixtData).ChromeOptions

	// Ensure all logins do not clear existing users.
	opts = append(opts, chrome.KeepState())
	loginOpts := append(opts, chrome.GAIALogin(chrome.Creds{User: username, Pass: password}))
	noLoginOpts := append(opts, chrome.NoLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))

	defer func() {
		faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
		cr.Close(ctx)
	}()

	// Check that Smart Lock is shown on the sign-in screen when the Signin
	// with Smart Lock subfeature is enabled.
	s.Log("Enabling Signin with Smart Lock")
	if err := smartlock.EnableSmartLockLogin(ctx, tconn, cr, password); err != nil {
		s.Fatal("Failed to enable Signin with Smart Lock: ", err)
	}
	if err = smartlock.CheckSmartLockVisibilityOnLockScreen(ctx, true /*expectVisible*/, tconn, username, password); err != nil {
		s.Fatal("CheckSmartLockVisibilityOnLockScreen(expectVisible=true) failed: ", err)
	}
	if cr, tconn, err = smartlock.CheckSmartLockVisibilityOnSigninScreen(ctx, true /*expectVisible*/, cr, tconn, loginOpts, noLoginOpts); err != nil {
		s.Fatal("CheckSmartLockVisibilityOnSigninScreen(expectVisible=true) failed: ", err)
	}

	// Check that Smart Lock is not shown when disabled for Login.
	s.Log("Disabling Signin with Smart Lock")
	if err := smartlock.DisableSmartLockLogin(ctx, tconn, cr); err != nil {
		s.Fatal("Failed to disable Signin with Smart Lock: ", err)
	}
	// Checking the lock screen before the signin screen here is important.
	// The setting change doesn't seem to take effect when logging out and back in
	// with chrome.New(), so running this first CheckSmartLockVisibility for the
	// lock screen helps to ensure that the next call will succeed for
	// signin. This could be due to b/208931707 or b/208931338.
	if err = smartlock.CheckSmartLockVisibilityOnLockScreen(ctx, true /*expectVisible*/, tconn, username, password); err != nil {
		s.Fatal("CheckSmartLockVisibilityOnLockScreen(expectVisible=true) failed: ", err)
	}
	if cr, tconn, err = smartlock.CheckSmartLockVisibilityOnSigninScreen(ctx, false /*expectVisible*/, cr, tconn, loginOpts, noLoginOpts); err != nil {
		s.Fatal("CheckSmartLockVisibilityOnSigninScreen(expectVisible=false) failed: ", err)
	}
}
