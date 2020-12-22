// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DefaultAppsArcOff,
		Desc:         "Verify Default App Icons Launch Opt In Flow When PlayStore is Turned Off ",
		Contacts:     []string{"rnanjappan@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: chrome.LoginTimeout + arc.BootTimeout + 120*time.Second,
		Vars:    []string{"arc.username", "arc.password"},
	})
}

func DefaultAppsArcOff(ctx context.Context, s *testing.State) {

	const (
		defaultTimeout = 20 * time.Second
	)

	username := s.RequiredVar("arc.username")
	password := s.RequiredVar("arc.password")

	cr, err := chrome.New(ctx, chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"), chrome.ARCSupported(), chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Optin to PlayStore.
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}
	if err := optin.WaitForPlayStoreShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}

	// Turn off the Play Store
	if err := optin.SetPlayStoreEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to Turn Off Play Store: ", err)
	}

	// Wait for 10 seconds.
	if err := testing.Sleep(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	// Launch Play Games App.
	if err := launcher.LaunchApp(ctx, tconn, apps.PlayGames); err != nil {
		s.Log("Failed to Launch the Play Games: ", err)
	}

	// Find  "More" button and click
	MoreParams := chromeui.FindParams{
		Role: chromeui.RoleTypeButton,
		Name: "More",
	}
	more, err := chromeui.FindWithTimeout(ctx, tconn, MoreParams, defaultTimeout)
	if err != nil {
		s.Fatal("Failed to find More Button: ", err)
	}
	defer more.Release(ctx)

	if err := more.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click the GooglePlayStore Heading: ", err)
	}

	// Find the "Accept" button and click
	Accept := chromeui.FindParams{
		Role: chromeui.RoleTypeButton,
		Name: "Accept",
	}
	accept, err := chromeui.FindWithTimeout(ctx, tconn, Accept, defaultTimeout)
	if err != nil {
		s.Fatal("Failed to find Accept Button: ", err)
	}
	defer accept.Release(ctx)

	if err := accept.LeftClick(ctx); err != nil {
		s.Fatal("Failed to Click Accept Button: ", err)
	}

	// Verify Play Store is Enabled.
	if err = optin.WaitForPlayStoreReady(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for Play Store to be ready: ", err)
	}

}
