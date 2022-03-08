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
		Func:         SettingsEnableDisable,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests ability to enable/disable Smart Lock with Settings",
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

// SettingsEnableDisable tests enabling/disabling Smart Lock in settings.
func SettingsEnableDisable(ctx context.Context, s *testing.State) {
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

	// Signin with Smart Lock should be disabled by default. Disable it
	// here in case a previous test run left it on.
	if err := smartlock.DisableSmartLockLogin(t.Ctx, t.Tconn, t.Cr); err != nil {
		s.Fatal("Failed to disable smart lock login: ", err)
	}

	// Check that Smart Lock is not shown when disabled.
	s.Log("Disabling Smart Lock")
	if err := smartlock.ToggleSmartLockEnabled(t.Ctx, t.Tconn, t.Cr, t.Password); err != nil {
		s.Fatal("Failed to toggle off Smart Lock: ", err)
	}
	if err := testing.Sleep(t.Ctx, 3*time.Second); err != nil {
		s.Fatal("Failed to sleep on Smart Lock subpage: ", err)
	}
	if err := smartlock.CheckSmartLockVisibility(false /*expectVisible*/, false /*signin*/, &t); err != nil {
		s.Fatal("CheckSmartLockVisibility failed (lock screen, Smart Lock should not be visible): ", err)
	}
	if err := smartlock.CheckSmartLockVisibility(false /*expectVisible*/, true /*signin*/, &t); err != nil {
		s.Fatal("CheckSmartLockVisibility failed (signin screen, Smart Lock should not be visible): ", err)
	}

	// Check that Smart Lock is shown when enabled.
	s.Log("Enabling Smart Lock")
	if err := smartlock.ToggleSmartLockEnabled(t.Ctx, t.Tconn, t.Cr, t.Password); err != nil {
		s.Fatal("Failed to toggle off Smart Lock: ", err)
	}
	if err := testing.Sleep(t.Ctx, 3*time.Second); err != nil {
		s.Fatal("Failed to sleep on Smart Lock subpage: ", err)
	}
	if err := smartlock.CheckSmartLockVisibility(true /*expectVisible*/, false /*signin*/, &t); err != nil {
		s.Fatal("CheckSmartLockVisibility failed (lock screen, Smart Lock should be visible): ", err)
	}
	if err := smartlock.CheckSmartLockVisibility(false /*expectVisible*/, true /*signin*/, &t); err != nil {
		s.Fatal("CheckSmartLockVisibility failed (signin screen, Smart Lock should not be visible): ", err)
	}
}
