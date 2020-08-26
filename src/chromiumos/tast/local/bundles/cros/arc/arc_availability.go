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
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ArcAvailability,
		Desc:         "Verifies that ARC is available in different scenarios.",
		Contacts:     []string{"timkovich@chromium.org", "arc-eng@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arc.ArcAvailability.username", "arc.ArcAvailability.password"},
	})
}

// Checks that the Play Store icon is visible and that the table of contents is active.
func isPlayStoreOpen(ctx context.Context, s *testing.State, d *ui.Device, tconn *chrome.TestConn) bool {
	shelfItems, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get shelf items: ", err)
	}
	hasIcon := false
	for _, shelfItem := range shelfItems {
		if shelfItem.AppID == apps.PlayStore.ID {
			hasIcon = true
			break
		}
	}

	if !hasIcon {
		return false
	}

	toc := d.Object(ui.ID("com.android.vending:id/play_card"))
	err = toc.WaitForExists(ctx, 10*time.Second)
	return err == nil
}

func ArcAvailability(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("arc.ArcAvailability.username")
	password := s.RequiredVar("arc.ArcAvailability.password")

	// Setup Chrome.
	cr, err := chrome.New(ctx, chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"), chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// Optin to Play Store.
	s.Log("Opting into Play Store")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}
	if err := optin.WaitForPlayStoreShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	isPlayStoreOpen(ctx, s, d, tconn)
}
