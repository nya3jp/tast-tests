// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const ghostWindowPlayStorePkgName = "com.android.vending"
const ghostWindowARCSettingsPkgName = "com.android.settings"

func init() {
	testing.AddTest(&testing.Test{
		Func:         GhostWindow,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test ghost window for ARC Apps",
		Contacts:     []string{"sstan@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "arc"},
		Timeout:      5 * time.Minute,
		Vars:         []string{"ui.gaiaPoolDefault"},
	})
}

func waitARCWindowShown(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration, pkgName string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := ash.GetARCAppWindowInfo(ctx, tconn, pkgName); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

func waitGhostWindowShown(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration, appID string) error {
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

func clickRestoreButtonNormalStatus(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, s *testing.State) error {
	alertDialog := nodewith.NameStartingWith("Restore apps?").Role(role.AlertDialog)
	restoreButton := nodewith.Name("RESTORE").Role(role.Button).Ancestor(alertDialog)

	ui := uiauto.New(tconn)
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "click_normal_restore")

	if err := uiauto.Combine("restore playstore",
		// Click Restore on the restore alert.
		ui.LeftClick(restoreButton))(ctx); err != nil {
		return err
	}
	return nil
}

func clickRestoreButtonCrashedStatus(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, s *testing.State) error {
	// Full text is "Your *Chromebook* restarted unexpectedly".
	alertDialog := nodewith.NameStartingWith("Your").Role(role.AlertDialog)
	restoreButton := nodewith.Name("RESTORE").Role(role.Button).Ancestor(alertDialog)

	ui := uiauto.New(tconn)
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "click_crash_restore")

	if err := uiauto.Combine("restore playstore",
		// Click Restore on the restore alert.
		ui.LeftClick(restoreButton))(ctx); err != nil {
		return err
	}
	return nil
}

func waitForWindowInfoSaved(ctx context.Context) {
	// According to the PRD of Full Restore go/chrome-os-full-restore-dd,
	// it uses a throttle of 2.5s to save the app launching and window status
	// information to the backend. Therefore, sleep 5 seconds here.
	testing.Sleep(ctx, 5*time.Second)
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
	if err := waitARCWindowShown(ctx, tconn, time.Minute, ghostWindowPlayStorePkgName); err != nil {
		return errors.Wrap(err, "failed to wait for Play Store")
	}

	return nil
}

// launchAndroidSettings opens the ARC Settings Page from Chrome Settings.
func launchAndroidSettings(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)
	playStoreButton := nodewith.Name("Google Play Store").Role(role.Button)
	settingPage, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "apps", ui.Exists(playStoreButton))
	if err != nil {
		return errors.Wrap(err, "failed to launch apps settings page")
	}

	if err := uiauto.Combine("open Android settings",
		ui.FocusAndWait(playStoreButton),
		ui.LeftClick(playStoreButton),
		ui.LeftClick(nodewith.Name("Manage Android preferences").Role(role.Link)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to open ARC settings page")
	}

	if err := waitARCWindowShown(ctx, tconn, 10*time.Second, ghostWindowARCSettingsPkgName); err != nil {
		return errors.Wrapf(err, "failed to wait ARC window %s shown", ghostWindowARCSettingsPkgName)
	}

	// Close ChromeOS setting page to avoid affect window restore.
	return settingPage.Close(ctx)
}

func verifyGhostWindow(ctx context.Context, s *testing.State, cr *chrome.Chrome, isCrash bool, appID string) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create test API connection")
	}

	if isCrash {
		if err := clickRestoreButtonCrashedStatus(ctx, cr, tconn, s); err != nil {
			return errors.Wrap(err, "failed to click Restore button on crash restore notification")
		}
	} else {
		if err := clickRestoreButtonNormalStatus(ctx, cr, tconn, s); err != nil {
			return errors.Wrap(err, "failed to click Restore button on normal restore notification")
		}
	}

	// Make sure ARC Ghost Window of PlayStore has popup.
	if err := waitGhostWindowShown(ctx, tconn, time.Minute, appID); err != nil {
		return errors.Wrap(err, "failed to wait for Play Store")
	}
	return nil
}

func GhostWindow(ctx context.Context, s *testing.State) {
	// Reserve 10 seconds for clean-up tasks.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Test restore single PlayStore task.
	{
		// Test ghost window in logout case.
		cr, err := loginChrome(ctx, s, nil)
		if err != nil {
			s.Fatal("Failed to optin: ", err)
		}
		defer cr.Close(cleanupCtx)

		creds := cr.Creds()
		if err := optinAndLaunchPlayStore(ctx, s, cr); err != nil {
			s.Fatal("Failed to initial optin: ", err)
		}

		// Stop Chrome after window info saved.
		waitForWindowInfoSaved(ctx)
		if err := upstart.RestartJob(ctx, "ui"); err != nil {
			s.Fatal("Failed to log out: ", err)
		}

		// Re-login.
		cr, err = loginChrome(ctx, s, &creds)
		if err != nil {
			s.Fatal("Failed to re-optin: ", err)
		}
		defer cr.Close(cleanupCtx)

		if err := verifyGhostWindow(ctx, s, cr, false, apps.PlayStore.ID); err != nil {
			s.Fatal("Failed to launch ghost window: ", err)
		}
	}

	// Test restore PlayStore and Android Setting tasks.
	{
		// Test ghost window in logout case.
		cr, err := loginChrome(ctx, s, nil)
		if err != nil {
			s.Fatal("Failed to optin: ", err)
		}
		defer cr.Close(cleanupCtx)

		creds := cr.Creds()
		if err := optinAndLaunchPlayStore(ctx, s, cr); err != nil {
			s.Fatal("Failed to initial optin: ", err)
		}

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to create Test API connection: ", err)
		}

		if err := launchAndroidSettings(ctx, cr, tconn); err != nil {
			s.Fatal("Failed to launch ARC setting: ", err)
		}

		// Stop Chrome after window info saved.
		waitForWindowInfoSaved(ctx)
		if err := upstart.RestartJob(ctx, "ui"); err != nil {
			s.Fatal("Failed to log out: ", err)
		}

		// Re-login
		cr, err = loginChrome(ctx, s, &creds)
		if err != nil {
			s.Fatal("Failed to re-optin: ", err)
		}
		defer cr.Close(cleanupCtx)

		if err := verifyGhostWindow(ctx, s, cr, false, apps.AndroidSettings.ID); err != nil {
			s.Fatal("Failed to launch ghost window: ", err)
		}
	}
}
