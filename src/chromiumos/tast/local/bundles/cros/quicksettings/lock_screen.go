// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quicksettings

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LockScreen,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that the screen can be locked from Quick Settings",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"amehfooz@chromium.org",
			"cros-system-ui-eng@google.com",
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

// LockScreen tests that the screen can be locked from Quick Settings
// and verifies its contents when the screen is locked.
func LockScreen(ctx context.Context, s *testing.State) {
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

	finder := nodewith.Role(role.Window).ClassName("ash/message_center/MessagePopup")

	if err := uiauto.New(tconn).WithTimeout(30 * time.Second).WaitUntilExists(finder)(ctx); err != nil {
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
		restricted, err := quicksettings.PodRestricted(ctx, tconn, setting)
		if err != nil {
			s.Fatalf("Failed to check restricted status of pod setting %v: %v", setting, err)
		}
		if !restricted {
			s.Errorf("Pod setting %v not restricted: %v", setting, err)
		}
	}

	hasBattery := s.Param().(bool)

	// Get the common Quick Settings elements to verify.
	checkNodes, err := quicksettings.CommonElements(ctx, tconn, hasBattery, true /* isLockedScreen */)
	if err != nil {
		s.Fatal("Failed to get the params in LockedScreen: ", err)
	}

	// Loop through all the Quick Settings nodes of locked screen and verify if they exist.
	for node, finder := range checkNodes {
		ui := uiauto.New(tconn)
		if err := ui.WaitUntilExists(finder)(ctx); err != nil {
			s.Fatalf("Failed to wait for %v node to exist: %v", node, err)
		}
		shown, err := ui.IsNodeFound(ctx, finder)
		if err != nil {
			s.Fatalf("Failed to find %v node: %v", node, err)
		}
		if !shown {
			s.Errorf("%v node was not shown in the UI", node)
		}
	}
}
