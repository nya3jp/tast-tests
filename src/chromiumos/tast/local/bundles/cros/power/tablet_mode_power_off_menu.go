// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"time"

	pmpb "chromiumos/system_api/power_manager_proto"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

type testType int

const (
	signOutTest testType = iota
	shutDownTest
	closeMenuTest
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabletModePowerOffMenu,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks power",
		Contacts: []string{
			"sophiewen@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      2 * time.Minute,
		Params: []testing.Param{
			{
				Name: "sign_out",
				Val:  signOutTest,
			}, {
				Name: "shut_down",
				Val:  shutDownTest,
			}, {
				Name: "close_menu",
				Val:  closeMenuTest,
			},
		},
	})
}

func TabletModePowerOffMenu(ctx context.Context, s *testing.State) {
	test := s.Param().(testType)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	const (
		username = "testuser@gmail.com"
		password = "password"
	)

	cr, err := chrome.New(ctx, chrome.FakeLogin(chrome.Creds{User: username, Pass: password}))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	pc, err := pointer.NewTouch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to set up the touch context: ", err)
	}
	defer pc.Close()

	ui := uiauto.New(tconn)

	// Long press the power button.
	emitter := &power.PowerManagerEmitter{}
	eventType := pmpb.InputEvent_POWER_BUTTON_DOWN
	if err := emitter.EmitInputEvent(ctx, &pmpb.InputEvent{Type: &eventType}); err != nil {
		s.Fatal("Send POWER_BUTTON_DOWN failed: ", err)
	}
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		s.Fatal("Failed to long press power button: ", err)
	}
	eventType = pmpb.InputEvent_POWER_BUTTON_UP
	if err := emitter.EmitInputEvent(ctx, &pmpb.InputEvent{Type: &eventType}); err != nil {
		s.Fatal("Send POWER_BUTTON_UP failed: ", err)
	}

	switch test {
	case signOutTest:
		// Ensure "Sign out" button signs current user out.
		signOutButton := nodewith.Name("Sign out").HasClass("PowerButtonMenuItemView")
		if err := pc.Click(signOutButton)(ctx); err != nil {
			s.Fatal("Failed to click the sign out button: ", err)
		}

		sm, err := session.NewSessionManager(ctx)
		if err != nil {
			s.Fatal(err, "failed to connect to session manager")
		}

		const state = "stopped"
		sw, err := sm.WatchSessionStateChanged(ctx, state)
		if err != nil {
			s.Fatal(err, "failed to watch for D-Bus signals")
		}
		defer sw.Close(ctx)
	case shutDownTest:
		// Ensure "Shut down" button appears on the power button menu. Do not
		// press the button since shut down would disconnect the device and end
		// the test process here.
		shutdownButton := nodewith.Name("Shut down").HasClass("PowerButtonMenuItemView")
		if err := ui.WaitUntilExists(shutdownButton)(ctx); err != nil {
			s.Fatal("Failed to wait for the shut down button: ", err)
		}
	case closeMenuTest:
		// Ensure tapping outside menu closes menu.
		powerButtonMenu := nodewith.ClassName("PowerButtonMenuView")
		if err := ui.WaitUntilExists(powerButtonMenu)(ctx); err != nil {
			s.Fatal("Failed to find PowerButtonMenuView: ", err)
		}

		powerButtonMenuBounds, err := ui.Location(ctx, powerButtonMenu)
		s.Log("powerButtonMenuBounds: ", powerButtonMenuBounds)
		if err != nil {
			s.Fatal("Failed to get PowerButtonMenu bounds: ", err)
		}

		offset := coords.NewPoint(5, 5)
		point := powerButtonMenuBounds.BottomRight().Add(offset)
		s.Log("point: ", point)
		if err := pc.ClickAt(point)(ctx); err != nil {
			s.Fatal("Failed to press a point outside of the PowerButtonMenu: ", err)
		}

		if err := ui.WithTimeout(45 * time.Second).WaitUntilGone(powerButtonMenu)(ctx); err != nil {
			s.Fatal("PowerButtonMenuScreenView is not dismissed: ", err)
		}
	}
}
