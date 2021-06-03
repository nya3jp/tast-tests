// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GhostWindow,
		Desc:         "Test ghost window for ARC Apps",
		Contacts:     []string{"sstan@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		//HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture: "arcBooted",
		Timeout: 4 * time.Minute,
		Vars:    []string{"ui.gaiaPoolDefault"},
	})
}

func waitForFullRestoreSaveWindow(ctx context.Context) {
	// According to the PRD of Full Restore go/chrome-os-full-restore-dd,
	// it uses a throttle of 2.5s to save the app launching and window statue information to the backend.
	// Therefore, sleep 3 seconds here.
	testing.Sleep(ctx, 3*time.Second)
}

func waitPlayStoreShown(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration, arcWindow bool) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if window, err := ash.GetARCAppWindowInfo(ctx, tconn, "com.google.vending"); err != nil {
			return testing.PollBreak(err)
		} else if window == nil {
			return errors.New("PlayStore ARC app window is not shown yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

func firstBoot(ctx context.Context, s *testing.State) {
	// Setup Chrome.
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.EnableFeatures("FullRestore"),
		chrome.EnableFeatures("ArcGhostWindow"),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Optin to Play Store.
	s.Log("Opting into Play Store")
	maxAttempts := 1

	if err := optin.PerformWithRetry(ctx, cr, maxAttempts); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// In this case we cannot use this func, since it inspect App by check shelf ID.
	// After ghost window finish ash shelf integration, the ghost window will also
	// carry the corresponding app's ID into shelf. Here we need to check actuall
	// aura window.
	// if err := optin.WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {
	// 	s.Fatal("Failed to wait for Play Store: ", err)
	// }
	if err := waitPlayStoreShown(ctx, tconn, time.Minute, false); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}

}

func GhostWindow(ctx context.Context, s *testing.State) {
	firstBoot(ctx, s)
	waitForFullRestoreSaveWindow(ctx)

}
