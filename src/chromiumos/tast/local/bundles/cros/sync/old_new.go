// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sync

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OldNew,
		Desc: "Checks that prefs are synced between sync categorization enabled and disabled states",
		Contacts: []string{
			"rsorokin@google.com",
			"cros-oac@google.com",
			"chromeos-sw-engprod@google.com",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
		},
		Attr: []string{"group:mainline", "informational"},
		VarDeps: []string{
			"ui.gaiaPoolDefault",
		},
		Timeout: chrome.GAIALoginTimeout + chrome.LoginTimeout + 10*time.Minute,
	})
}

func OldNew(ctx context.Context, s *testing.State) {
	var creds chrome.Creds
	func() {
		cr, err := chrome.New(
			ctx,
			chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
			chrome.DisableFeatures("SyncSettingsCategorization"),
			chrome.DontSkipOOBEAfterLogin())
		if err != nil {
			s.Fatal("Chrome login failed: ", err)
		}
		defer cr.Close(ctx)
		creds = cr.Creds()

		oobeConn, err := cr.WaitForOOBEConnection(ctx)
		if err != nil {
			s.Fatal("Failed to create OOBE connection: ", err)
		}
		defer oobeConn.Close()

		// Go through the sync consent screen
		if err := oobeConn.Eval(ctx, "OobeAPI.advanceToScreen('sync-consent')", nil); err != nil {
			s.Fatal("Failed to advance to the sync consent: ", err)
		}

		if err := oobeConn.WaitForExprFailOnErr(ctx, "!document.querySelector('#sync-consent').$.syncConsentOverviewDialog.hidden"); err != nil {
			s.Fatal("Failed to wait for the sync dialog visible: ", err)
		}

		if err := oobeConn.Eval(ctx, "document.querySelector('#sync-consent').$.nonSplitSettingsAcceptButton.click()", nil); err != nil {
			s.Fatal("Failed to click on the next button: ", err)
		}

		if err := oobeConn.Eval(ctx, "OobeAPI.skipPostLoginScreens()", nil); err != nil {
			s.Fatal("Failed to skip post login screens: ", err)
		}

		if err = cr.WaitForOOBEConnectionToBeDismissed(ctx); err != nil {
			s.Fatal("Failed to wait for OOBE to be closed: ", err)
		}

		tconn, err := cr.TestAPIConn(ctx)
		info, err := display.GetPrimaryInfo(ctx, tconn)

		if err != nil {
			s.Fatal("Failed to find the primary display info: ", err)
		}

		if err := ash.SetShelfBehavior(ctx, tconn, info.ID, ash.ShelfBehaviorAlwaysAutoHide); err != nil {
			s.Fatal("Failed to sete the shelf behavior to 'never auto-hide' for display ID ", info.ID)
		}
	}()

	// New login with the feature enabled.
	cr, err := chrome.New(
		ctx,
		chrome.GAIALogin(creds),
		chrome.EnableFeatures("SyncSettingsCategorization"),
		chrome.DontSkipOOBEAfterLogin())
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)
	creds = cr.Creds()

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to create OOBE connection: ", err)
	}
	defer oobeConn.Close()

	// Go through the sync consent screen
	if err := oobeConn.Eval(ctx, "OobeAPI.advanceToScreen('sync-consent')", nil); err != nil {
		s.Fatal("Failed to advance to the sync consent: ", err)
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx, "!document.querySelector('#sync-consent').$.syncConsentOverviewDialog.hidden"); err != nil {
		s.Fatal("Failed to wait for the sync dialog visible: ", err)
	}

	if err := oobeConn.Eval(ctx, "document.querySelector('#sync-consent').$.nonSplitSettingsAcceptButton.click()", nil); err != nil {
		s.Fatal("Failed to click on the next button: ", err)
	}

	if err := oobeConn.Eval(ctx, "OobeAPI.skipPostLoginScreens()", nil); err != nil {
		s.Fatal("Failed to skip post login screens: ", err)
	}

	if err = cr.WaitForOOBEConnectionToBeDismissed(ctx); err != nil {
		s.Fatal("Failed to wait for OOBE to be closed: ", err)
	}

	syncConn, err := cr.NewConn(ctx, "chrome://sync-internals")
	if err != nil {
		s.Fatal("Failed to open sync internals page: ", err)
	}
	res := false
	if err = syncConn.Eval(ctx, `
			function foo() {
				for (const a of document.querySelectorAll('td')) {
					if (a.textContent.includes('Chrome OS Sync Feature'))
					return a.nextElementSibling.textContent.includes('Enforced Enabled');
				}
				return false;
			}
			foo();
			`, &res); err != nil {
		s.Fatal("Failed to execute javascript: ", err)
	}
	if !res {
		s.Fatal("Chrome OS Sync Feature is not enabled")
	}

	tconn, err := cr.TestAPIConn(ctx)
	// This test assumes shelf visibility, setting the shelf behavior explicitly.
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find the primary display info: ", err)
	}
	shelfBehavior, err := ash.GetShelfBehavior(ctx, tconn, info.ID)
	if err != nil {
		s.Fatal("Failed to get the shelf behavior for display ID ", info.ID)
	}
	if shelfBehavior != ash.ShelfBehaviorAlwaysAutoHide {
		s.Fatal("Shelf behavior did not sync")
	}
}
