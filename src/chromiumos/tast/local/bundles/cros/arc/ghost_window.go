// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const ghostWindowPlayStorePkgName = "com.android.vending"
const appID = "cnbgggchhmkkdmeppjobngjoejnihlei"

const testTimeout = 5 * time.Minute

func init() {
	testing.AddTest(&testing.Test{
		Func:         GhostWindow,
		Desc:         "Test ghost window for ARC Apps",
		Contacts:     []string{"sstan@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Timeout:      testTimeout,
		Vars:         []string{"ui.gaiaPoolDefault"},
	})
}

func waitPlayStoreShown(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := ash.GetARCAppWindowInfo(ctx, tconn, ghostWindowPlayStorePkgName); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

func waitPlayStoreGhostWindowShown(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := ash.GetARCGhostWindowInfo(ctx, tconn, appID); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

func loginChrome(ctx context.Context, s *testing.State, creds *chrome.Creds) (*chrome.Chrome, error) {
	if creds != nil {
		// Setup Chrome. Login by the creds.
		cr, err := chrome.New(ctx,
			chrome.GAIALogin(*creds),
			chrome.ARCSupported(),
			chrome.EnableFeatures("FullRestore"),
			chrome.EnableFeatures("ArcGhostWindow"),
			chrome.RemoveNotification(false),
			chrome.KeepState(),
			chrome.ExtraArgs(arc.DisableSyncFlags()...))
		if err != nil {
			return nil, errors.Wrap(err, "failed to start Chrome")
		}
		return cr, nil
	}
	// Setup Chrome for a new cred.
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.EnableFeatures("FullRestore"),
		chrome.EnableFeatures("ArcGhostWindow"),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Chrome")
	}
	return cr, nil
}

func clickRestoreButtonNormalStatus(ctx context.Context, tconn *chrome.TestConn, s *testing.State) error {
	alertDialog := nodewith.NameStartingWith("Restore apps?").Role(role.AlertDialog)
	restoreButton := nodewith.Name("RESTORE").Role(role.Button).Ancestor(alertDialog)

	ui := uiauto.New(tconn)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := uiauto.Combine("restore playstore",
		// Click Restore on the restore alert.
		ui.LeftClick(restoreButton))(ctx); err != nil {
		return err
	}
	return nil
}

func clickRestoreButtonCrashedStatus(ctx context.Context, tconn *chrome.TestConn, s *testing.State) error {
	// Full text is "Your *Chromebook* restarted unexpectedly".
	alertDialog := nodewith.NameStartingWith("Your").Role(role.AlertDialog)
	restoreButton := nodewith.Name("RESTORE").Role(role.Button).Ancestor(alertDialog)

	ui := uiauto.New(tconn)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := uiauto.Combine("restore playstore",
		// Click Restore on the restore alert.
		ui.LeftClick(restoreButton))(ctx); err != nil {
		return err
	}
	return nil
}

func optinAndLaunchPlayStore(ctx context.Context, s *testing.State, cr *chrome.Chrome) error {
	// Optin to Play Store.
	s.Log("Opting into Play Store")
	maxAttempts := 1

	if err := optin.PerformWithRetry(ctx, cr, maxAttempts); err != nil {
		return errors.Wrap(err, "failed to optin to Play Store")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create test API connection")
	}

	// The PlayStore only popup automatically on first optin of an account.
	// Launch it here in case it's not the first optin.
	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		return errors.Wrap(err, "failed to launch Play Store")
	}

	// In this case we cannot use this func, since it inspect App by check shelf ID.
	// After ghost window finish ash shelf integration, the ghost window will also
	// carry the corresponding app's ID into shelf. Here we need to check actual
	// aura window.
	if err := waitPlayStoreShown(ctx, tconn, testTimeout); err != nil {
		return errors.Wrap(err, "failed to wait for Play Store")
	}

	// According to the PRD of Full Restore go/chrome-os-full-restore-dd,
	// it uses a throttle of 2.5s to save the app launching and window status
	// information to the backend. Therefore, sleep 5 seconds here.
	testing.Sleep(ctx, 5*time.Second)

	return nil
}

func verifyGhostWindow(ctx context.Context, s *testing.State, cr *chrome.Chrome, isCrash bool) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create test API connection")
	}

	if isCrash {
		if err := clickRestoreButtonCrashedStatus(ctx, tconn, s); err != nil {
			return errors.Wrap(err, "failed to click Restore button on crash restore notification")
		}
	} else {
		if err := clickRestoreButtonNormalStatus(ctx, tconn, s); err != nil {
			return errors.Wrap(err, "failed to click Restore button on normal restore notification")
		}
	}

	// Make sure ARC Ghost Window of PlayStore has popup.
	if err := waitPlayStoreGhostWindowShown(ctx, tconn, testTimeout); err != nil {
		return errors.Wrap(err, "failed to wait for Play Store")
	}
	return nil
}

func GhostWindow(ctx context.Context, s *testing.State) {
	// Test ghost window in logout case.
	cr, err := loginChrome(ctx, s, nil)
	if err != nil {
		s.Fatal("Fail to optin: ", err)
	}
	defer cr.Close(ctx)
	creds := cr.Creds()
	if err := optinAndLaunchPlayStore(ctx, s, cr); err != nil {
		s.Fatal("Failed to initial optin: ", err)
	}

	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to log out: ", err)
	}

	// Re-login
	cr, err = loginChrome(ctx, s, &creds)
	if err != nil {
		s.Fatal("Fail to re-optin: ", err)
	}
	defer cr.Close(ctx)

	if err := verifyGhostWindow(ctx, s, cr, false); err != nil {
		s.Fatal("Failed to launch ghost window: ", err)
	}
}
