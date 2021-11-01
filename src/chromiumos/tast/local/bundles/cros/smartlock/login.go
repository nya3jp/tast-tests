// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package smartlock contains tests for the Smart Lock feature in ChromeOS.
package smartlock

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/local/chrome/crossdevice/smartlock"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Login,
		Desc: "Enables ability to sign-in with Smart lock, logs out signs in again, then signs out and uses Smart Lock to login",
		Contacts: []string{
			"dhaddock@chromium.org",
			"chromeos-sw-engprod@google.com",
			"chromeos-cross-device-eng@google.com",
		},
		Attr:         []string{"group:cross-device"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "crossdeviceOnboardedNoVideoNoLock",
	})
}

// Login tests logging into ChromeOS using Smart Lock feature.
func Login(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*crossdevice.FixtData).Chrome
	tconn := s.FixtValue().(*crossdevice.FixtData).TestConn
	androidDevice := s.FixtValue().(*crossdevice.FixtData).AndroidDevice
	username := s.FixtValue().(*crossdevice.FixtData).Username
	password := s.FixtValue().(*crossdevice.FixtData).Password
	opts := s.FixtValue().(*crossdevice.FixtData).ChromeOptions
	opts = append(opts, chrome.KeepState())

	// Smart Lock requires the Android phone to have a PIN code. Set it here and defer removing it.
	if err := androidDevice.SetPIN(ctx); err != nil {
		s.Fatal("Failed to set lock screen PIN on Android: ", err)
	}
	defer androidDevice.ClearPIN(ctx)

	s.Log("Enabling Smart Lock for login")
	/* if err := settings.DisableSmartLockLogin(cr)(ctx); err != nil {
	                s.Fatal("Failed to enable smart lock login: ", err)
	        }
		if err := testing.Sleep(ctx, 5 * time.Second); err != nil {
	                s.Fatal("Failed to sleep after clicking on the compat-mode button: ", err)
	        }*/
	if err := smartlock.EnableSmartLockLogin(ctx, tconn, cr, password); err != nil {
		s.Fatal("Failed to enable smart lock login: ", err)
	}
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep after clicking on the compat-mode button: ", err)
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()
	cr.Close(ctx)
	if err := signOut(ctx, kb); err != nil {
		s.Fatal("Failed to sign out")
	}

	s.Log("Signed out after enabling Smart Lock sign-in")
	//TODO Check for box telling us to login again
	for i := 0; i < 2; i++ {
		s.Log("Logging in again with password before Smart Lock will work")
		loginOpts := append(opts, chrome.GAIALogin(chrome.Creds{User: username, Pass: password}))
		cr, err = chrome.New(
			ctx,
			loginOpts...,
		)
		if err != nil {
			s.Fatal("Failed to start Chrome 2nd time: ", err)
		}

		tconn, err = cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Creating test API connection failed: ", err)
		}
		if err := ash.WaitForShelf(ctx, tconn, 30*time.Second); err != nil {
			s.Fatal("Shelf did not appear after logging in: ", err)
		}
		_, err = smartlock.OpenSmartLockSubpage(ctx, tconn, cr)
		if err != nil {
			s.Fatal("Failed to open Smart lock sub page: ", err)
		}
		if err := testing.Sleep(ctx, 10*time.Second); err != nil {
			s.Fatal("Failed to sleep after clicking on the compat-mode button: ", err)
		}
		cr.Close(ctx)
		if err := signOut(ctx, kb); err != nil {
			s.Fatal("Failed to sign out")
		}
		s.Log("Signed out after signing in with password")
	}
	nologinOpts := append(opts, chrome.NoLogin())
	nologinOpts = append(nologinOpts, chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))
	cr, err = chrome.New(
		ctx,
		nologinOpts...,
	)
	defer cr.Close(ctx)
	if err != nil {
		s.Fatal("Failed to start chrome: ", err)
	}
	tconn, err = cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := lockscreen.WaitForPasswordField(ctx, tconn, username, 15*time.Second); err != nil {
		s.Fatal("password text field did not appear in the UI: ", err)
	}
	s.Log("Waiting for the Smart Lock ready indicator")
	if err := lockscreen.WaitForSmartUnlockReady(ctx, tconn); err != nil {
		s.Fatal("Failed waiting for Smart Lock icon to turn green: ", err)
	}

	s.Log("Smart Unlock available. Clicking user image to log back in")
	if err := lockscreen.ClickUserImage(ctx, tconn); err != nil {
		s.Fatal("Failed to click user image on the ChromeOS lock screen: ", err)
	}

	// Check for shelf to ensure we are logged back in.
	if err := ash.WaitForShelf(ctx, tconn, 10*time.Second); err != nil {
		s.Fatal("Shelf did not appear after logging in: ", err)
	}
}

func signOut(ctx context.Context, kb *input.KeyboardEventWriter) error {
	// Sign out
	if err := kb.Accel(ctx, "Ctrl+Shift+Q"); err != nil {
		return errors.Wrap(err, "failed to emulate shortcut 1st press")
	}
	if err := kb.Accel(ctx, "Ctrl+Shift+Q"); err != nil {
		return errors.Wrap(err, "failed to emulate shortcut 2nd press")
	}
	return nil
}
