// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package smartlock contains tests for the Smart Lock feature in ChromeOS.
package smartlock

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/local/chrome/crossdevice/smartlock"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type settingsTestData struct {
	ctx         context.Context
	loginOpts   []chrome.Option
	noLoginOpts []chrome.Option
	cr          *chrome.Chrome
	tconn       *chrome.TestConn
	kb          *input.KeyboardEventWriter
	username    string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Settings,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests ability to enable/disable Smart lock with Settings",
		Contacts: []string{
			"cclem@google.com",
			"chromeos-sw-engprod@google.com",
			"chromeos-cross-device-eng@google.com",
		},
		Attr:         []string{"group:cross-device"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "crossdeviceOnboardedNoLock",
		Timeout:      10 * time.Minute,
	})
}

// Settings tests changing Smart Lock settings.
func Settings(ctx context.Context, s *testing.State) {
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

	// Logout is done by keyboard shortcut. So setup one to reuse throughout the test.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}

	t := settingsTestData{
		ctx:         ctx,
		loginOpts:   loginOpts,
		noLoginOpts: noLoginOpts,
		cr:          cr,
		tconn:       tconn,
		kb:          kb,
		username:    username,
	}

	defer t.kb.Close()
	defer faillog.DumpUITreeOnError(t.ctx, s.OutDir(), s.HasError, t.tconn)
	defer t.cr.Close(t.ctx)

	// Check that Smart Lock is shown when enabled for Login.
	s.Log("Enabling Smart Lock for login")
	if err := smartlock.EnableSmartLockLogin(t.ctx, t.tconn, t.cr, password); err != nil {
		s.Fatal("Failed to enable smart lock login: ", err)
	}
	if err := goToLoginScreen(&t, s); err != nil {
		s.Fatal("Failed logging out: ", err)
	}
	s.Log("Waiting for the Smart Lock available indicator")
	if err := lockscreen.WaitForSmartUnlockAvailable(t.ctx, t.tconn); err != nil {
		s.Fatal("Failed waiting for Smart Lock icon to appear: ", err)
	}
	s.Log("Signing in with password")
	if err = signIn(&t); err != nil {
		s.Fatal("Failed to sign in: ", err)
	}

	// Check that Smart Lock is not shown when disabled for Login.
	s.Log("Disabling Smart Lock for login")
	if err := smartlock.DisableSmartLockLogin(t.ctx, t.tconn, t.cr); err != nil {
		s.Fatal("Failed to disable smart lock login: ", err)
	}
	if err := goToLoginScreen(&t, s); err != nil {
		s.Fatal("Failed logging out: ", err)
	}
	s.Log("Checking for the Smart Lock available indicator")
	if lockscreen.HasAuthIconView(t.ctx, t.tconn) {
		s.Fatal("Found auth icon; Smart Lock should not be visible")
	}
	s.Log("Signing in with password")
	if err = signIn(&t); err != nil {
		s.Fatal("Failed to sign in: ", err)
	}

	// Check that Smart Lock is not shown when disabled.
	s.Log("Disabling Smart Lock")
	if err := smartlock.ToggleSmartLockEnabled(t.ctx, t.tconn, t.cr, password); err != nil {
		s.Fatal("Failed to toggle off Smart Lock: ", err)
	}
	if err := testing.Sleep(t.ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to sleep on Smart Lock subpage: ", err)
	}
	s.Log("Locking the ChromeOS screen")
	if err := lockscreen.Lock(t.ctx, t.tconn); err != nil {
		s.Fatal("Failed to lock the screen on ChromeOS: ", err)
	}
	s.Log("Checking for the Smart Lock available indicator")
	if lockscreen.HasAuthIconView(t.ctx, t.tconn) {
		s.Fatal("Found auth icon; Smart Lock should not be visible")
	}
	s.Log("Unlocking with password")
	if err = lockscreen.EnterPassword(t.ctx, t.tconn, username, password, t.kb); err != nil {
		s.Fatal("Failed to unlock with password: ", err)
	}

	// Check that Smart Lock is shown when enabled.
	s.Log("Enabling Smart Lock")
	if err := smartlock.ToggleSmartLockEnabled(t.ctx, t.tconn, t.cr, password); err != nil {
		s.Fatal("Failed to toggle off Smart Lock: ", err)
	}
	if err := testing.Sleep(t.ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to sleep on Smart Lock subpage: ", err)
	}
	s.Log("Locking the ChromeOS screen")
	if err := lockscreen.Lock(t.ctx, t.tconn); err != nil {
		s.Fatal("Failed to lock the screen on ChromeOS: ", err)
	}
	s.Log("Waiting for the Smart Lock available indicator")
	if err := lockscreen.WaitForSmartUnlockAvailable(t.ctx, t.tconn); err != nil {
		s.Fatal("Failed waiting for Smart Lock icon to appear: ", err)
	}
}

func goToLoginScreen(t *settingsTestData, s *testing.State) error {
	if err := testing.Sleep(t.ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep before SignOut")
	}

	var err error
	if err = smartlock.SignOut(t.ctx, t.cr, t.kb); err != nil {
		return errors.Wrap(err, "failed to sign out")
	}
	s.Log("Signed out after enabling Smart Lock sign-in")

	// TODO(b/217272610) Remove this second log in once this bug is resolved.
	s.Log("Logging in again with password before Smart Lock will work")
	if err = signIn(t); err != nil {
		return errors.Wrap(err, "failed to sign in")
	}

	if _, err := smartlock.OpenSmartLockSubpage(t.ctx, t.tconn, t.cr); err != nil {
		return errors.Wrap(err, "failed to open Smart lock sub page")
	}
	if err := testing.Sleep(t.ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep on Smart Lock subpage")
	}
	if err := smartlock.SignOut(t.ctx, t.cr, t.kb); err != nil {
		return errors.Wrap(err, "failed to sign out")
	}
	s.Log("Signed out after signing in with password")

	t.cr, err = chrome.New(
		t.ctx,
		t.noLoginOpts...,
	)
	if err != nil {
		return errors.Wrap(err, "failed to start chrome")
	}
	t.tconn, err = t.cr.SigninProfileTestAPIConn(t.ctx)
	if err != nil {
		return errors.Wrap(err, "getting API connection failed")
	}

	return nil
}

func signIn(t *settingsTestData) error {
	var err error
	t.cr, err = chrome.New(
		t.ctx,
		t.loginOpts...,
	)
	if err != nil {
		return errors.Wrap(err, "failed to login to Chrome")
	}

	t.tconn, err = t.cr.TestAPIConn(t.ctx)
	if err != nil {
		return errors.Wrap(err, "creating test API connection failed")
	}

	return nil
}
