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
	"chromiumos/tast/testing"
)

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

	var t smartlock.SettingsTestData
	var err error
	if t, err = smartlock.GetSettingsTestData(ctx, cr, tconn, username, password, loginOpts, noLoginOpts); err != nil {
		s.Fatal("Failed to GetSettingsTestData: ", err)
	}
	defer smartlock.CleanUpSettingsTestData(&t, s.OutDir(), s.HasError)

	// Check that Smart Lock is shown on the sign-in screen when the Signin
	// with Smart Lock subfeature is enabled.
	s.Log("Enabling Signin with Smart Lock")
	if err := smartlock.EnableSmartLockLogin(t.Ctx, t.Tconn, t.Cr, t.Password); err != nil {
		s.Fatal("Failed to enable Signin with Smart Lock: ", err)
	}
	if err := smartlock.CheckSmartLockVisibility(true /*expectVisible*/, false /*signin*/, &t); err != nil {
		s.Fatal("CheckSmartLockVisibility failed (lock screen, Smart Lock should be visible): ", err)
	}
	if err := smartlock.CheckSmartLockVisibility(true /*expectVisible*/, true /*signin*/, &t); err != nil {
		s.Fatal("CheckSmartLockVisibility failed (signin screen, Smart Lock should be visible): ", err)
	}

	// Check that Smart Lock is not shown when disabled for Login.
	s.Log("Disabling Signin with Smart Lock")
	if err := smartlock.DisableSmartLockLogin(t.Ctx, t.Tconn, t.Cr); err != nil {
		s.Fatal("Failed to disable Signin with Smart Lock: ", err)
	}
	if err := smartlock.CheckSmartLockVisibility(true /*expectVisible*/, false /*signin*/, &t); err != nil {
		s.Fatal("CheckSmartLockVisibility failed (lock screen, Smart Lock should be visible): ", err)
	}
	if err := smartlock.CheckSmartLockVisibility(false /*expectVisible*/, true /*signin*/, &t); err != nil {
		s.Fatal("CheckSmartLockVisibility failed (signin screen, Smart Lock should not be visible): ", err)
	}
}
