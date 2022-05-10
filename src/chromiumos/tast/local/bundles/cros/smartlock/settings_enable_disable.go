// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package smartlock contains tests for the Smart Lock feature in ChromeOS.
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

	// Signin with Smart Lock should be disabled by default. Disable it
	// here in case a previous test run left it on.
	if err := smartlock.DisableSmartLockLogin(ctx, tconn, cr); err != nil {
		s.Fatal("Failed to disable smart lock login: ", err)
	}

	// Check that Smart Lock is not shown when disabled.
	s.Log("Disabling Smart Lock")
	if err := smartlock.ToggleSmartLockEnabled(ctx, false /*enable*/, tconn, cr, password); err != nil {
		s.Fatal("Failed to toggle off Smart Lock: ", err)
	}
	if err = smartlock.CheckSmartLockVisibilityOnLockScreen(ctx, false /*expectVisible*/, tconn, username, password); err != nil {
		s.Fatal("CheckSmartLockVisibilityOnLockScreen(expectVisible=false) failed: ", err)
	}
	if cr, tconn, err = smartlock.CheckSmartLockVisibilityOnSigninScreen(ctx, false /*expectVisible*/, cr, tconn, loginOpts, noLoginOpts); err != nil {
		s.Fatal("CheckSmartLockVisibilityOnSigninScreen(expectVisible=false) failed: ", err)
	}

	// Check that Smart Lock is shown when enabled.
	s.Log("Enabling Smart Lock")
	if err := smartlock.ToggleSmartLockEnabled(ctx, true /*enable*/, tconn, cr, password); err != nil {
		s.Fatal("Failed to toggle on Smart Lock: ", err)
	}
	if err = smartlock.CheckSmartLockVisibilityOnLockScreen(ctx, true /*expectVisible*/, tconn, username, password); err != nil {
		s.Fatal("CheckSmartLockVisibilityOnLockScreen(expectVisible=true) failed: ", err)
	}
	if cr, tconn, err = smartlock.CheckSmartLockVisibilityOnSigninScreen(ctx, false /*expectVisible*/, cr, tconn, loginOpts, noLoginOpts); err != nil {
		s.Fatal("CheckSmartLockVisibilityOnSigninScreen(expectVisible=false) failed: ", err)
	}
}
