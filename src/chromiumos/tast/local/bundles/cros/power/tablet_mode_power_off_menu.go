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
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
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
		Vars:         []string{"ui.signinProfileTestExtensionManifestKey"},
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
		s.Fatal("Creating test API connection failed: ", err)
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

	emitter, err := power.NewPowerManagerEmitter(ctx)
	if err != nil {
		s.Fatal("Unable to create power manager emitter: ", err)
	}
	defer func(cleanupCtx context.Context) {
		if err := emitter.Stop(cleanupCtx); err != nil {
			s.Log("Unable to stop emitter: ", err)
		}
	}(cleanupCtx)

	switch test {
	case signOutTest:
		longPressPowerButton(ctx, s, emitter)
		signOutButton := nodewith.Name("Sign out").HasClass("PowerButtonMenuItemView")
		pc.Click(signOutButton)(ctx)

		const state = "stopped"
		sm, err := session.NewSessionManager(ctx)
		if err != nil {
			s.Fatal(err, "failed to connect to session manager")
		}
		sw, err := sm.WatchSessionStateChanged(ctx, state)
		if err != nil {
			s.Fatal(err, "failed to watch for D-Bus signals")
		}
		defer sw.Close(ctx)

		if cr, err = chrome.New(ctx,
			chrome.ExtraArgs("--skip-force-online-signin-for-testing"),
			chrome.NoLogin(),
			chrome.KeepState(),
			chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
		); err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		defer cr.Close(cleanupCtx)

		if tconn, err = cr.SigninProfileTestAPIConn(ctx); err != nil {
			s.Fatal("Failed to re-establish test API connection")
		}

		if _, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.ReadyForPassword }, 10*time.Second); err != nil {
			s.Fatal("Failed to wait for login screen: ", err)
		}
	case shutDownTest:
		// Ensure "Power off" button shuts device down.
		longPressPowerButton(ctx, s, emitter)
		shutdownButton := nodewith.Name("Shut down").HasClass("PowerButtonMenuItemView")
		pc.Click(shutdownButton)(ctx)

		sdCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
		defer cancel()
		dut := s.DUT()
		if err := dut.WaitUnreachable(sdCtx); err != nil {
			s.Fatal("Failed to wait for the DUT being unreachable: ", err)
		}
	case closeMenuTest:
		longPressPowerButton(ctx, s, emitter)
		powerButtonMenu := nodewith.ClassName("PowerButtonMenuScreenView")
		ui := uiauto.New(tconn)

		if err := ui.WaitUntilExists(powerButtonMenu)(ctx); err != nil {
			s.Fatal("Failed to find PowerButtonMenuScreenView: ", err)
		}
		if err := pc.ClickAt(coords.NewPoint(5, 5))(ctx); err != nil {
			s.Fatal("Failed to press a point on the screen")
		}
		if err := ui.WaitUntilGone(powerButtonMenu)(ctx); err != nil {
			s.Fatal("PowerButtonMenuScreenView is not dismissed: ", err)
		}
	}
}

func longPressPowerButton(ctx context.Context, s *testing.State, p *power.PowerManagerEmitter) {
	eventType := pmpb.InputEvent_POWER_BUTTON_DOWN
	if err := p.EmitInputEvent(ctx, &pmpb.InputEvent{Type: &eventType}); err != nil {
		s.Fatal("Send POWER_BUTTON_DOWN failed: ", err)
	}
	testing.Sleep(ctx, 2*time.Second)
	eventType = pmpb.InputEvent_POWER_BUTTON_UP
	if err := p.EmitInputEvent(ctx, &pmpb.InputEvent{Type: &eventType}); err != nil {
		s.Fatal("Send POWER_BUTTON_UP failed: ", err)
	}
}
