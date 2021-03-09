// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DefaultAppLaunchWhenArcIsOff,
		Desc:         "Verify Default App Icons Launch Opt In Flow When PlayStore is Off ",
		Contacts:     []string{"rnanjappan@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
		Vars:    []string{"arc.username", "arc.password"},
	})
}

func DefaultAppLaunchWhenArcIsOff(ctx context.Context, s *testing.State) {
	const (
		defaultTimeout = 20 * time.Second
	)

	username := s.RequiredVar("arc.username")
	password := s.RequiredVar("arc.password")

	cr, err := chrome.New(ctx,
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Launch Play Games App.
	if err := launcher.SearchAndWaitForAppOpen(tconn, kb, apps.PlayGames)(ctx); err != nil {
		s.Log("Failed to Launch the Play Games: ", err)
	}

	// Find  "More" button and click
	moreParams := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "More",
	}
	more, err := ui.FindWithTimeout(ctx, tconn, moreParams, defaultTimeout)
	if err != nil {
		s.Fatal("Failed to find More Button: ", err)
	}
	defer more.Release(ctx)

	if err := more.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click More button: ", err)
	}

	// Find the "Accept" button and click
	acceptParams := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "Accept",
	}
	accept, err := ui.FindWithTimeout(ctx, tconn, acceptParams, defaultTimeout)
	if err != nil {
		s.Fatal("Failed to find Accept Button: ", err)
	}
	defer accept.Release(ctx)

	if err := accept.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click Accept button: ", err)
	}

	// Verify Play Store is Enabled.
	if err = optin.WaitForPlayStoreReady(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for Play Store to be ready: ", err)
	}

}
