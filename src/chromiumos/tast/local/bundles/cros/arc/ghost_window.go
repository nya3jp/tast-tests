// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const (
	ghostWindowPlayStorePkgName     = "com.android.vending"
	defaultGhostWindowMessagePrefix = "Starting "
	fixupGhostWindowMessagePrefix   = "Updating Android System"
)

type gwTestParams struct {
	name string
	fn   func(context.Context, *testing.State)
}

var ghostWindowFeatureFlags = []string{
	"FullRestore",
	"ArcGhostWindow",
	"ArcWindowPredictor",
	"ArcFixupWindowFeature",
	"ArcGhostWindowNewStyle",
}

var fullrestoreGwTests = []gwTestParams{
	{"fullrestorePlayStore", testLaunchFromFullRestoreSinglePlayStore},
	{"fullrestorePlayStoreAndSetting", testLaunchFromFullRestorePlayStoreAndAndroidSetting},
	{"fullrestorePlayStoreInTabletMode", testLaunchFromFullRestorePlayStoreInTabletMode},
}

var generalLaunchGwTests = []gwTestParams{
	{"shelfLaunchPlayStore", testShelfLaunchPlayStore},
	{"launcherLaunchPlayStore", testLauncherLaunchPlayStore},
}

var fixupGwTests = []gwTestParams{
	{"fixup", testFixupPlayStore},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GhostWindow,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test ghost window for ARC Apps",
		Contacts:     []string{"sstan@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Timeout:      5 * time.Minute,
		Vars:         []string{"ui.gaiaPoolDefault"},
		Params: []testing.Param{{
			Name:              "general",
			Val:               generalLaunchGwTests,
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name: "general_r",
			Val:  generalLaunchGwTests,
			// Temporarily restrict it only for ARC R, not T or above version.
			ExtraSoftwareDeps: []string{"android_vm_r"},
		}, {
			Name:              "fullrestore",
			Val:               fullrestoreGwTests,
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name: "fullrestore_r",
			Val:  fullrestoreGwTests,
			// Temporarily restrict it only for ARC R, not T or above version.
			ExtraSoftwareDeps: []string{"android_vm_r"},
		}, {
			Name: "fixup_r",
			Val:  fixupGwTests,
			ExtraSoftwareDeps: []string{
				// The fixup is R-only feature.
				"android_vm_r",
				// Temporarily skip on ARCVM virtio-blk /data enabled boards,
				// as we cannot chown files over SSHFS.
				"no_arcvm_virtio_blk_data",
			},
		}},
	})
}

func GhostWindow(ctx context.Context, s *testing.State) {
	for _, test := range s.Param().([]gwTestParams) {
		s.Logf("Running %q sub-test", test.name)
		s.Run(ctx, test.name, test.fn)
	}
}

// testLaunchFromFullRestoreSinglePlayStore test restore single PlayStore task.
func testLaunchFromFullRestoreSinglePlayStore(ctx context.Context, s *testing.State) {
	// Reserve 10 seconds for clean-up tasks.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Test ghost window in logout case.
	cr, err := loginChrome(ctx, s, nil)
	if err != nil {
		s.Fatal("Failed to optin: ", err)
	}
	defer cr.Close(cleanupCtx)

	creds := cr.Creds()
	if err := optinAndLaunchPlayStore(ctx, cr); err != nil {
		s.Fatal("Failed to initial optin: ", err)
	}

	// Stop Chrome after window info saved.
	waitForWindowInfoSaved(ctx)

	// Re-login.
	if err := logoutChrome(ctx, cr); err != nil {
		s.Fatal("Failed to logout chrome: ", err)
	}
	cr, err = loginChrome(ctx, s, &creds)
	if err != nil {
		s.Fatal("Failed to login again: ", err)
	}
	defer cr.Close(cleanupCtx)

	if err := restoreAndVerifyGhostWindow(ctx, s, cr, false, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to launch ghost window: ", err)
	}

}

// testLaunchFromFullRestorePlayStoreAndAndroidSetting test restore PlayStore and Android Setting tasks.
func testLaunchFromFullRestorePlayStoreAndAndroidSetting(ctx context.Context, s *testing.State) {
	// Reserve 10 seconds for clean-up tasks.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Test ghost window in logout case.
	cr, err := loginChrome(ctx, s, nil)
	if err != nil {
		s.Fatal("Failed to optin: ", err)
	}
	defer cr.Close(cleanupCtx)

	creds := cr.Creds()
	if err := optinAndLaunchPlayStore(ctx, cr); err != nil {
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

	// Re-login.
	if err := logoutChrome(ctx, cr); err != nil {
		s.Fatal("Failed to logout chrome: ", err)
	}
	cr, err = loginChrome(ctx, s, &creds)
	if err != nil {
		s.Fatal("Failed to login again: ", err)
	}
	defer cr.Close(cleanupCtx)

	if err := restoreAndVerifyGhostWindow(ctx, s, cr, false, apps.AndroidSettings.ID); err != nil {
		s.Fatal("Failed to launch ghost window: ", err)
	}

}

// testLaunchFromFullRestorePlayStoreInTabletMode test restore single PlayStore task in tablet mode.
func testLaunchFromFullRestorePlayStoreInTabletMode(ctx context.Context, s *testing.State) {
	// Reserve 10 seconds for clean-up tasks.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Test ghost window in logout case.
	cr, err := loginChrome(ctx, s, nil)
	if err != nil {
		s.Fatal("Failed to optin: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	tabletModeStatus, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode status: ", err)
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeStatus)

	if err := ash.SetTabletModeEnabled(ctx, tconn, true); err != nil {
		s.Fatal("Failed to change device to tablet mode: ", err)
	}

	creds := cr.Creds()
	if err := optinAndLaunchPlayStore(ctx, cr); err != nil {
		s.Fatal("Failed to initial optin: ", err)
	}

	// Stop Chrome after window info saved.
	waitForWindowInfoSaved(ctx)

	// Re-login.
	if err := logoutChrome(ctx, cr); err != nil {
		s.Fatal("Failed to logout chrome: ", err)
	}
	cr, err = loginChrome(ctx, s, &creds)
	if err != nil {
		s.Fatal("Failed to login again: ", err)
	}
	defer cr.Close(cleanupCtx)

	if err := restoreAndVerifyGhostWindow(ctx, s, cr, false, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to launch ghost window: ", err)
	}
}

func testShelfLaunchPlayStore(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Test ghost window in logout case.
	cr, err := loginChrome(ctx, s, nil)
	if err != nil {
		s.Fatal("Failed to optin: ", err)
	}
	defer cr.Close(cleanupCtx)

	creds := cr.Creds()
	if err := optinAndLaunchPlayStore(ctx, cr); err != nil {
		s.Fatal("Failed to initial optin: ", err)
	}

	// Re-login to make sure the ARC has not finish boot when launch request sent.
	if err := logoutChrome(ctx, cr); err != nil {
		s.Fatal("Failed to logout chrome: ", err)
	}
	cr, err = loginChrome(ctx, s, &creds)
	if err != nil {
		s.Fatal("Failed to login again: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	if err := ash.WaitForShelf(ctx, tconn, 30*time.Second); err != nil {
		s.Fatal("Shelf did not appear after logging in: ", err)
	}

	// Launch from shelf require th app exist on the shelf, or pinned on the shelf.
	if err := ash.PinApp(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to pin PlayStore to the shelf: ", err)
	}

	if err = ash.LaunchAppFromShelf(ctx, tconn, apps.PlayStore.Name, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to launch PlayStore from shelf: ", err)
	}

	// Make sure ARC Ghost Window of PlayStore has popup.
	if err := waitGhostWindowShown(ctx, tconn, time.Minute, apps.PlayStore.ID, defaultGhostWindowMessagePrefix+apps.PlayStore.Name); err != nil {
		s.Fatal("Failed to wait for Ghost Window of PlayStore: ", err)
	}
}

func testLauncherLaunchPlayStore(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Test ghost window in logout case.
	cr, err := loginChrome(ctx, s, nil)
	if err != nil {
		s.Fatal("Failed to optin: ", err)
	}
	defer cr.Close(cleanupCtx)

	creds := cr.Creds()
	if err := optinAndLaunchPlayStore(ctx, cr); err != nil {
		s.Fatal("Failed to initial optin: ", err)
	}

	// Re-login to make sure the ARC has not finish boot when launch request sent.
	if err := logoutChrome(ctx, cr); err != nil {
		s.Fatal("Failed to logout chrome: ", err)
	}
	cr, err = loginChrome(ctx, s, &creds)
	if err != nil {
		s.Fatal("Failed to login again: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	if err := launcher.LaunchApp(tconn, apps.PlayStore.Name)(ctx); err != nil {
		s.Fatal("Failed to launch PlayStore from launcher: ", err)
	}

	// Make sure ARC Ghost Window of PlayStore has popup.
	if err := waitGhostWindowShown(ctx, tconn, time.Minute, apps.PlayStore.ID, defaultGhostWindowMessagePrefix+apps.PlayStore.Name); err != nil {
		s.Fatal("Failed to wait for Ghost Window of PlayStore: ", err)
	}
}

func testFixupPlayStore(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := loginChrome(ctx, s, nil)
	if err != nil {
		s.Fatal("Failed to optin: ", err)
	}
	defer cr.Close(cleanupCtx)

	creds := cr.Creds()

	if err := optinAndLaunchPlayStore(ctx, cr); err != nil {
		s.Fatal("Failed to initial optin: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to wait for ARC boot: ", err)
	}

	s.Log("Preparing to trigger fixup on next sign in")
	cleanupFunc, err := prepareFixup(ctx, a, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to prepare fixup: ", err)
	}
	defer cleanupFunc(cleanupCtx)

	// Re-login.
	if err := logoutChrome(ctx, cr); err != nil {
		s.Fatal("Failed to logout chrome: ", err)
	}
	cr, err = loginChrome(ctx, s, &creds)
	if err != nil {
		s.Fatal("Failed to re-optin: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	if err := launcher.LaunchApp(tconn, apps.PlayStore.Name)(ctx); err != nil {
		s.Fatal("Failed to launch PlayStore from launcher: ", err)
	}

	// Check that the fixup ghost window is popped up.
	if err := waitGhostWindowShown(ctx, tconn, time.Minute, apps.PlayStore.ID, fixupGhostWindowMessagePrefix); err != nil {
		s.Fatal("Failed to wait for Ghost Window of Play Store: ", err)
	}

	// TODO(b/257375894): Check that ghost window is replaced with ARC window.
}

func waitARCWindowShown(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration, pkgName string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := ash.GetARCAppWindowInfo(ctx, tconn, pkgName); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

func waitGhostWindowShown(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration, appID, messagePrefix string) error {
	ui := uiauto.New(tconn)
	return testing.Poll(ctx, func(ctx context.Context) error {
		window, err := ash.GetARCGhostWindowInfo(ctx, tconn, appID)
		if err != nil {
			return err
		}
		if messagePrefix != "" {
			windowFinder := nodewith.HasClass(window.Name).Role(role.Window)
			label := nodewith.Ancestor(windowFinder).Role(role.StaticText).HasClass("Label").NameStartingWith(messagePrefix)
			if err := ui.Exists(label)(ctx); err != nil {
				return err
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

func loginChrome(ctx context.Context, s *testing.State, creds *chrome.Creds) (*chrome.Chrome, error) {
	if creds != nil {
		cr, err := chrome.New(ctx,
			chrome.NoLogin(),
			chrome.ARCSupported(),
			chrome.EnableFeatures(ghostWindowFeatureFlags...),
			chrome.KeepState(),
			chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
			chrome.ExtraArgs(arc.DisableSyncFlags()...),
		)
		if err != nil {
			return nil, errors.Wrap(err, "chrome start failed")
		}

		tconn, err := cr.SigninProfileTestAPIConn(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to re-establish test API connection")
		}

		if _, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.ReadyForPassword }, 10*time.Second); err != nil {
			return nil, errors.Wrap(err, "failed to wait for login screen")
		}

		keyboard, err := input.Keyboard(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get keyboard")
		}
		defer keyboard.Close()

		if err = lockscreen.EnterPassword(ctx, tconn, creds.User, creds.Pass, keyboard); err != nil {
			return nil, errors.Wrap(err, "failed to enter password")
		}

		if err := lockscreen.WaitForLoggedIn(ctx, tconn, chrome.LoginTimeout); err != nil {
			s.Fatal("Failed to login: ", err)
		}

		return cr, nil
	}
	// Setup Chrome for a new cred.
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.EnableFeatures(ghostWindowFeatureFlags...),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Chrome")
	}
	return cr, nil
}

func logoutChrome(ctx context.Context, cr *chrome.Chrome) error {
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		return errors.Wrap(err, "failed to restart ui")
	}
	return nil
}

func clickRestoreButtonNormalStatus(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, s *testing.State) error {
	ui := uiauto.New(tconn).WithTimeout(time.Minute)
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "click_normal_restore")

	notificationDialog := nodewith.HasClass("AshNotificationView").NameStartingWith("Restore apps?")
	restoreButton := nodewith.Name("Restore").Role(role.Button).Ancestor(notificationDialog)
	if err := uiauto.Combine("restore playstore",
		// Click Restore on the restore alert.
		ui.LeftClick(restoreButton))(ctx); err != nil {
		return err
	}
	return nil
}

func clickRestoreButtonCrashedStatus(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, s *testing.State) error {
	ui := uiauto.New(tconn).WithTimeout(time.Minute)
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "click_crash_restore")

	// Full text is "Your *Chromebook* restarted unexpectedly".
	alertDialog := nodewith.HasClass("AshNotificationView").NameStartingWith("Your").Role(role.AlertDialog)
	restoreButton := nodewith.Name("Restore").Role(role.Button).Ancestor(alertDialog)

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

func optinAndLaunchPlayStore(ctx context.Context, cr *chrome.Chrome) error {
	// Optin to Play Store.
	testing.ContextLog(ctx, "Opting into Play Store")
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
	const ghostWindowARCSettingsPkgName = "com.android.settings"

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

func restoreAndVerifyGhostWindow(ctx context.Context, s *testing.State, cr *chrome.Chrome, isCrash bool, appID string) error {
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
	// In full restore cases, ghost windows don't have any messages on it.
	if err := waitGhostWindowShown(ctx, tconn, time.Minute, appID, ""); err != nil {
		return errors.Wrap(err, "failed to wait for Play Store")
	}
	return nil
}

func createDirectoryWithMediaRWUGID(path string) error {
	const mediaRWUGID = 656383

	if err := os.Mkdir(path, 0770); err != nil {
		return errors.Wrapf(err, "failed to create directory %s", path)
	}
	// By default, directories created with os.Mkdir has root UID and are not visible from
	// Android. Change their UID to media_rw to avoid it.
	if err := os.Chown(path, mediaRWUGID, mediaRWUGID); err != nil {
		return errors.Wrapf(err, "failed to chown the directory %s", path)
	}
	return nil
}

// prepareFixup sets up the package data in the SDCard partition so that a long fixup happens for
// Play Store after the user re-login.
func prepareFixup(ctx context.Context, a *arc.ARC, user string) (func(context.Context) error, error) {
	const (
		// The name of the extended attribute to mark the completion of the fixup.
		fixupXAttr = "arc.fixed"
		// The number of directories to create in Play Store's package directory in the
		// SDCard partition.
		numberOfDirectories = 10000
	)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	cleanup, err := arc.MountSDCardPartitionOnHostWithSSHFSIfVirtioBlkDataEnabled(ctx, a, user)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make Android's SDCard partition available on host")
	}
	defer cleanup(cleanupCtx)

	playStoreDataDir, err := arc.PkgDataDir(ctx, user, ghostWindowPlayStorePkgName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Play Store's package data directory")
	}
	// Wait for the Play Store path to appear.
	testing.ContextLogf(ctx, "Waiting for path %q", playStoreDataDir)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err = os.Stat(playStoreDataDir); err != nil {
			return errors.Wrapf(err, "path %s still does not exist", playStoreDataDir)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return nil, errors.Wrapf(err, "failed to wait for path %s", playStoreDataDir)
	}

	// Create a lot of empty directories in Play Store's package directory in SDCard partition.
	targetDir := filepath.Join(playStoreDataDir, "testdirs")
	if err := createDirectoryWithMediaRWUGID(targetDir); err != nil {
		return nil, errors.Wrapf(err, "failed to set up the target dir %s", targetDir)
	}
	cleanupFunc := func(ctx context.Context) error {
		cleanup, err := arc.MountSDCardPartitionOnHostWithSSHFSIfVirtioBlkDataEnabled(ctx, a, user)
		if err != nil {
			return errors.Wrap(err, "failed to make Android's SDCard partition available on host for cleanup")
		}
		defer cleanup(ctx)

		return os.RemoveAll(targetDir)
	}
	for i := 0; i < numberOfDirectories; i++ {
		dirPath := filepath.Join(targetDir, fmt.Sprintf("dir_%d", i))
		if err := createDirectoryWithMediaRWUGID(dirPath); err != nil {
			return cleanupFunc, errors.Wrapf(err, "failed to set up directory %s for fixup", dirPath)
		}
	}

	androidDataDir, err := arc.AndroidDataDir(ctx, user)
	if err != nil {
		return cleanupFunc, errors.Wrap(err, "failed to get android-data dir")
	}
	androidDirInSDCardPartition := filepath.Join(androidDataDir, "data/media/0/Android")

	for _, path := range []string{playStoreDataDir, androidDirInSDCardPartition} {
		cmd := testexec.CommandContext(ctx, "attr", "-r", fixupXAttr, path)
		if err := cmd.Run(testexec.DumpLogOnError); err != nil {
			return cleanupFunc, errors.Wrapf(err, "failed to unset fixup completion mark for %s", path)
		}
	}
	return cleanupFunc, nil
}
