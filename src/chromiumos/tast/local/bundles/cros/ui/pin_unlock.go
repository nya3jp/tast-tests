// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/chrome/ui/ossettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PINUnlock,
		Desc:         "Checks that PIN unlock and PIN autosubmit works for Chrome OS",
		Contacts:     []string{"chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Val: false,
		}, {
			Name: "autosubmit",
			Val:  true,
		}},
	})
}

// PINUnlock tests if we can unlock the device with a PIN.
func PINUnlock(ctx context.Context, s *testing.State) {
	const (
		username = "testuser@gmail.com"
		password = "good"
		gaiaID   = "1234"
		PIN      = "1234567890"
	)

	autosubmit := s.Param().(bool)

	cr, err := chrome.New(ctx, chrome.Auth(username, password, gaiaID))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Set up PIN through a connection to the Settings page.
	if err := ossettings.Launch(ctx, tconn); err != nil {
		s.Fatal("Failed to launch Settings app: ", err)
	}
	settingsConn, err := ossettings.ChromeConn(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get Chrome connection to Settings app: ", err)
	}
	defer settingsConn.Close()

	if err := ossettings.EnablePINUnlock(ctx, settingsConn, password, PIN, autosubmit); err != nil {
		s.Fatal("Failed to enable PIN unlock: ", err)
	}

	// Lock the screen.
	if err := lockscreen.Lock(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
		s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
	}

	// Enter and submit the PIN to unlock the DUT.
	if err := lockscreen.EnterPIN(ctx, tconn, PIN); err != nil {
		s.Fatal("Failed to enter in PIN: ", err)
	}

	if !autosubmit {
		if err := lockscreen.SubmitPIN(ctx, tconn); err != nil {
			s.Fatal("Failed to submit PIN: ", err)
		}
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return !st.Locked }, 30*time.Second); err != nil {
		s.Fatalf("Waiting for screen to be unlocked failed: %v (last status %+v)", err, st)
	}
}
