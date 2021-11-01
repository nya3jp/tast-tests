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
		Fixture:      "crossdeviceOnboarded",
	})
}

// Login tests logging into ChromeOS using Smart Lock feature.
func Login(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*crossdevice.FixtData).Chrome
	tconn := s.FixtValue().(*crossdevice.FixtData).TestConn
	tLoginConn := s.FixtValue().(*crossdevice.FixtData).LoginConn
	androidDevice := s.FixtValue().(*crossdevice.FixtData).AndroidDevice
	username := s.FixtValue().(*crossdevice.FixtData).Username
	password := s.FixtValue().(*crossdevice.FixtData).Password

	// Smart Lock requires the Android phone to have a PIN code. Set it here and defer removing it.
	if err := androidDevice.SetPIN(ctx); err != nil {
		s.Fatal("Failed to set lock screen PIN on Android: ", err)
	}
	//defer androidDevice.ClearPIN(ctx)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tLoginConn)

	s.Log("Enabling ability for Smart Lock to login")
	if err := smartlock.EnableLogin(ctx, tconn, cr, password); err != nil {
                s.Fatal("Failed to enable login with Smart Lock: ", err)
        }
	if err := testing.Sleep(ctx, 10 * time.Second); err != nil {
		s.Fatal("Failed to sleep after clicking on the compat-mode button: ", err)
	}
	kb, err := input.Keyboard(ctx)
        if err != nil {
                s.Fatal("Failed to get keyboard: ", err)
        }
        defer kb.Close()
	if err := signOut(ctx, kb); err != nil {
		s.Fatal("Failed to sign out")
	}
	s.Log("Signed out after enabling Smart Lock sign-in")
	// Login again with password before Smart Lock will work
	cr, err = waitForLogin(ctx, s, username, true)
	if err != nil {
		s.Fatal("Failed to wait for login screen")
	}
	s.Log("Signing back in with password one more time to enable Smart Lock signin")
	 if err := testing.Sleep(ctx, 10 * time.Second); err != nil {
                s.Fatal("Failed to sleep after clicking on the compat-mode button: ", err)
        }
	if err := kb.Type(ctx, password+"\n"); err != nil {
		s.Fatal("Entering password failed: ", err)
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
	if err := testing.Sleep(ctx, 10 * time.Second); err != nil {
                s.Fatal("Failed to sleep after clicking on the compat-mode button: ", err)
        }
	if err := signOut(ctx, kb); err != nil {
                s.Fatal("Failed to sign out")
        }
        s.Log("Signed out after signing in with password")
	cr, err = waitForLogin(ctx, s, username, false)
        if err != nil {
                s.Fatal("Failed to wait for login screen")
        }
	tconn, err = cr.TestAPIConn(ctx)
        if err != nil {
                s.Fatal("Creating test API connection failed: ", err)
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

func waitForLogin(ctx context.Context, s *testing.State, username string, prompt bool) (*chrome.Chrome, error) {
        // Login again with password before Smart Lock will work
	cr, err := chrome.New(
                ctx,
                chrome.NoLogin(),
                chrome.KeepState(),
                chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
        )
        if err != nil {
		return nil, errors.Wrap(err, "failed to start Chrome")
        }
	tconn, err := cr.SigninProfileTestAPIConn(ctx)
        if err != nil {
		 return nil,  errors.Wrap(err, "getting test API connection failed")
        }
        defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

        if err := lockscreen.WaitForPasswordField(ctx, tconn, username, 15*time.Second); err != nil {
                 return nil, errors.Wrap(err, "password text field did not appear in the UI")
        }
	if prompt {
		if err := lockscreen.WaitForSmartUnlockPasswordPrompt(ctx, tconn); err != nil {
			return nil, errors.Wrap(err, "Smart Lock password prompt did not appear in the UI")
		}
	}
	return cr, nil
}
