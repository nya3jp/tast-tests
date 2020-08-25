// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PINUnlock,
		Desc:         "Checks that PIN unlock works for Chrome OS",
		Contacts:     []string{"kyleshima@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Attr:         []string{"group:mainline", "informational"},
	})
}

func PINUnlock(ctx context.Context, s *testing.State) {
	const (
		username = "testuser@gmail.com"
		password = "good"
		pin      = "111111"
	)

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed creating virtual keyboard: ", err)
	}
	defer kb.Close()

	cr, err := chrome.New(ctx, chrome.Auth(username, password, "1234"))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test API connection failed: ", err)
	}

	// Set up PIN through a connection to the Settings page.
	if err := apps.Launch(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatal("Failed to launch Settings app: ", err)
	}
	settingsConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://os-settings/"))
	if err != nil {
		s.Fatal("Failed to get Chrome connection to Settings app: ", err)
	}
	defer settingsConn.Close()

	if err := settingsConn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		s.Fatal("Failed waiting for Settings app document state to be ready: ", err)
	}

	// Wait for chrome.quickUnlockPrivate to be available.
	if err := settingsConn.WaitForExpr(ctx, `typeof(chrome.quickUnlockPrivate) === "object"`); err != nil {
		s.Fatal("Failed waiting for chrome.quickUnlockPrivate to load: ", err)
	}

	// An auth token is required to set up the PIN.
	var token string
	if err := settingsConn.EvalPromise(ctx,
		fmt.Sprintf(`new Promise(function(resolve, reject) {
			chrome.quickUnlockPrivate.getAuthToken('%s', function(authToken) {
			  if (chrome.runtime.lastError === undefined) {
				resolve(authToken['token']);
			  } else {
				reject(chrome.runtime.lastError.message);
			  }
			});
		  })`, password),
		&token); err != nil {
		s.Fatal("Failed to get auth token: ", err)
	}
	s.Log("Auth token: ", token)

	// Set the PIN and enable PIN unlock.
	if err := settingsConn.EvalPromise(ctx,
		fmt.Sprintf(`new Promise(function(resolve, reject) {
			chrome.quickUnlockPrivate.setModes('%s', [chrome.quickUnlockPrivate.QuickUnlockMode.PIN], ['%s'], function(success) {
			  if (chrome.runtime.lastError === undefined) {
				resolve(success);
			  } else {
				reject(chrome.runtime.lastError.message);
			  }
			});
		  })`, token, pin),
		nil); err != nil {
		s.Fatal("Failed to set PIN and enable PIN unlock: ", err)
	}

	// Lock the screen with the keyboard.
	const accel = "Search+L"
	s.Log("Locking screen via ", accel)
	if err := kb.Accel(ctx, accel); err != nil {
		s.Fatalf("Typing %v failed: %v", accel, err)
	}
	s.Log("Waiting for Chrome to report that screen is locked")

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
		s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
	}

	// Find the PIN pad in the UI.
	pinpad, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "LoginPinView"}, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find PIN pad: ", err)
	}
	defer pinpad.Release(ctx)

	// Find the 1 button in the PIN pad and enter the PIN.
	one, err := pinpad.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeButton, Name: "1"}, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find 1 button in the PIN pad: ", err)
	}
	defer one.Release(ctx)

	for i := 0; i < 6; i++ {
		if err := one.LeftClick(ctx); err != nil {
			s.Fatal("Failed to press 1 button in the PIN pad: ", err)
		}
	}

	// Submit the PIN.
	submit, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: "Submit", Role: ui.RoleTypeButton}, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find submit button: ", err)
	}
	defer submit.Release(ctx)

	if err := submit.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click submit button for PIN: ", err)
	}

	s.Log("Waiting for Chrome to report that screen is unlocked")
	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return !st.Locked }, 30*time.Second); err != nil {
		s.Fatalf("Waiting for screen to be unlocked failed: %v (last status %+v)", err, st)
	}
}
