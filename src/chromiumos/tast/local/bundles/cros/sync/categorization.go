// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sync

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Categorization,
		Desc: "Checks that OS Sync is enabled for new users by default",
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
		Timeout: chrome.GAIALoginTimeout + 10*time.Minute,
	})
}

func Categorization(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(
		ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.EnableFeatures("SyncSettingsCategorization"),
		chrome.DontSkipOOBEAfterLogin())
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

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
}
