// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: QuickSettingsLockScreen,
		Desc: "Checks that the screen can be locked from Quick Settings",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"amehfooz@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraHardwareDeps: hwdep.D(hwdep.Battery()),
			Val:               true,
		}, {
			Name: "no_battery",
			Val:  false,
		}},
	})
}

// QuickSettingsLockScreen tests that the screen can be locked from Quick Settings
// and verifies its contents when the screen is locked.
func QuickSettingsLockScreen(ctx context.Context, s *testing.State) {
	const (
		username = "testuser@gmail.com"
		password = "pass"

		lockTimeout = 30 * time.Second
	)

	cr, err := chrome.New(ctx, chrome.FakeLogin(chrome.Creds{User: username, Pass: password}))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Take a screenshot to show a notification. Using the virtual keyboard is required since
	// different physical keyboards can require different key combinations to take a screenshot.
	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get virtual keyboard: ", err)
	}
	defer keyboard.Close()

	if err := keyboard.Accel(ctx, "Ctrl+F5"); err != nil {
		s.Fatal("Failed to take a screenshot: ", err)
	}

	params := ui.FindParams{
		Role:      ui.RoleTypeWindow,
		ClassName: "ash/message_center/MessagePopup",
	}

	if err := ui.WaitUntilExists(ctx, tconn, params, 30*time.Second); err != nil {
		s.Fatal("Failed to find notification center: ", err)
	}

	if err := quicksettings.LockScreen(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}
	// Unlock the screen to ensure subsequent tests aren't affected by the screen remaining locked.
	// TODO(crbug/1156812): Remove once chrome.go has a way to clean up the lock screen state.
	defer func() {
		if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, lockTimeout); err != nil {
			s.Fatalf("Waiting for screen to be ready for password failed: %v (last status %+v)", err, st)
		}

		if err := lockscreen.EnterPassword(ctx, tconn, username, password+"\n", keyboard); err != nil {
			s.Fatal("Entering password failed: ", err)
		}

		if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.LoggedIn }, 30*time.Second); err != nil {
			s.Fatalf("Failed waiting to log in: %v, last state: %+v", err, st)
		}
	}()

	// Explicitly show Quick Settings on the lock screen, so it will
	// remain open for the UI verification steps.
	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to show quick settings on the lock screen: ", err)
	}
	defer quicksettings.Hide(ctx, tconn)

	// Check if notifications are hidden.
	if hidden, err := quicksettings.NotificationsHidden(ctx, tconn); err != nil {
		s.Fatal("Failed to check if notifications were hidden: ", err)
	} else if !hidden {
		s.Error("Notifications were not hidden")
	}

	// Get the restricted featured pods.
	restrictedPods, err := quicksettings.RestrictedSettingsPods(ctx)
	if err != nil {
		s.Fatal("Failed to get the restricted pod param: ", err)
	}

	// Verify that the pod icons are restricted on the locked screen.
	for _, setting := range restrictedPods {
		if restricted, err := quicksettings.PodRestricted(ctx, tconn, setting); err != nil {
			s.Fatalf("Failed to check restricted status of pod setting %v: %v", setting, err)
		} else if !restricted {
			s.Errorf("Pod setting %v not restricted: %v", setting, err)
		}
	}

	// Get the common Quick Settings elements to verify.
	checkNodes, err := quicksettings.CommonElementsInQuickSettings(ctx, tconn, s.Param().(bool))
	if err != nil {
		s.Fatal("Failed to get the params in LockedScreen: ", err)
	}

	// Loop through all the Quick Settings nodes of locked screen and verify if they exist.
	for node, params := range checkNodes {
		if shown, err := ui.Exists(ctx, tconn, params); err != nil {
			s.Fatalf("Failed to check existence of %v: %v", node, err)
		} else if !shown {
			s.Errorf("%v was not found in the UI", node)
		}
	}
}
