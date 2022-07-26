// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package taskswitchcuj

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// packageInfo contains info needed to launch and open an ARC application.
type packageInfo struct {
	name       string
	query      string // Search query to find the package in the launcher.
	skipSplash func(ctx context.Context) error
}

const (
	playStorePackageName = "com.android.vending"
	gmailPackageName     = "com.google.android.gm"
)

// getPackages returns a list of all the packages that should be opened
// during the test. ARC installation can be flaky, so the number of packages
// we open is limited.
func getPackages(ctx context.Context, tconn *chrome.TestConn, d *ui.Device) []packageInfo {
	return []packageInfo{
		{name: gmailPackageName, query: "Gmail", skipSplash: func(ctx context.Context) error {
			return skipGmailSplash(ctx, tconn, d)
		}},
		{name: playStorePackageName, query: "Play Store", skipSplash: func(ctx context.Context) error {
			return nil
		}},
	}
}

// installPackages installs each package in |packages|.
func installPackages(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, packages []packageInfo) error {
	pkgs, err := a.InstalledPackages(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list the installed packages")
	}

	var openedPlayStore bool
	for _, pkg := range packages {
		packageName := pkg.name
		if _, ok := pkgs[packageName]; ok {
			testing.ContextLogf(ctx, "%s is already installed", packageName)
			continue
		}

		testing.ContextLog(ctx, "Installing: ", packageName)
		openedPlayStore = true
		installCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		if err = playstore.InstallApp(installCtx, a, d, packageName, &playstore.Options{TryLimit: -1}); err != nil {
			cancel()
			return errors.Wrapf(err, "failed to install %s", packageName)
		}
		cancel()
	}

	if !openedPlayStore {
		return nil
	}

	if err := optin.ClosePlayStore(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to close Play Store")
	}

	// Add a sleep to stabilize closing the Play Store at the end of
	// install packages.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	return nil
}

// launchPackages launches each package in |packages| by calling
// |openAppList|, searching for the package name, and clicking on the
// first result. This returns the number of packages that were opened.
func launchPackages(ctx context.Context, tconn *chrome.TestConn, kw *input.KeyboardEventWriter, ac *uiauto.Context, packages []packageInfo) (int, error) {
	// Keep track of the initial number of windows to ensure
	// we open the right number of windows.
	var ws []*ash.Window
	var err error
	if ws, err = ash.GetAllWindows(ctx, tconn); err != nil {
		return 0, errors.Wrap(err, "failed to get window list")
	}
	initialNumWindows := len(ws)

	launchCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	for _, pkg := range packages {
		testing.ContextLog(ctx, "Launching: ", pkg.name)
		if err := action.Combine(
			"launch app and skip app splash screen",
			launcher.SearchAndLaunch(tconn, kw, pkg.query),
			func(ctx context.Context) error { return ash.WaitForVisible(ctx, tconn, pkg.name) },
			func(ctx context.Context) error {
				testing.ContextLog(ctx, "Skipping the splash screen of ", pkg.query)
				return nil
			},
			pkg.skipSplash,
			// Wait some time let the launcher animations stabilize.
			action.Sleep(10*time.Second),
		)(launchCtx); err != nil {
			return 0, errors.Wrapf(err, "failed to launch package %q by typing in the query %q", pkg.name, pkg.query)
		}
	}

	expectedNumWindows := initialNumWindows + len(packages)
	if ws, err := ash.GetAllWindows(ctx, tconn); err != nil {
		return 0, errors.Wrap(err, "failed to get window list after opening ARC apps")
	} else if len(ws) != expectedNumWindows {
		return 0, errors.Wrapf(err, "unexpected number of windows open after launching ARC applications, got: %d, expected: %d", len(ws), expectedNumWindows)
	}

	return len(packages), nil
}

// skipGmailSplash skips the splash screen for the Gmail ARC app.
func skipGmailSplash(ctx context.Context, tconn *chrome.TestConn, d *ui.Device) error {
	const (
		dialogID            = "com.google.android.gm:id/customPanel"
		dismissID           = "com.google.android.gm:id/gm_dismiss_button"
		customPanelMaxCount = 10
		actionTimeout       = 10 * time.Second
	)

	// Dismiss the ARC compatibility mode splash from Ash, if any.
	if err := cuj.DismissMobilePrompt(ctx, tconn); err != nil {
		return errors.Wrap(err, `failed to dismiss compatibility splash`)
	}

	gotIt := d.Object(ui.Text("GOT IT"))
	if err := gotIt.WaitForExists(ctx, actionTimeout); err != nil {
		testing.ContextLog(ctx, `Failed to find "GOT IT" button, believing splash screen has been dismissed already`)
		return nil
	}
	if err := gotIt.Click(ctx); err != nil {
		return errors.Wrap(err, `failed to click "GOT IT" button`)
	}

	// Sometimes, the account information might not be ready yet. In that case
	// a warning dialog appears.
	pleaseAdd := d.Object(ui.Text("Please add at least one email address"))
	if err := pleaseAdd.WaitForExists(ctx, actionTimeout); err == nil {
		// Even though the warning dialog appears, the email address should
		// appear already. Therefore, simply click the 'OK' button to
		// dismiss the warning dialog and moves on.
		if err := testing.Sleep(ctx, actionTimeout); err != nil {
			return errors.Wrap(err, "failed to wait for the email address to appear")
		}
		okButton := d.Object(ui.ClassName("android.widget.Button"), ui.Text("OK"))
		if err := okButton.Exists(ctx); err != nil {
			return errors.Wrap(err, "failed to find the OK button")
		}
		if err := okButton.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click the OK button")
		}
	}
	takeMe := d.Object(ui.Text("TAKE ME TO GMAIL"))
	if err := takeMe.WaitForExists(ctx, actionTimeout); err != nil {
		return errors.Wrap(err, `"TAKE ME TO GMAIL" is not shown`)
	}
	if err := takeMe.Click(ctx); err != nil {
		return errors.Wrap(err, `failed to click "TAKE ME TO GMAIL" button`)
	}
	// After clicking 'take me to gmail', it might show a series of dialogs to
	// finalize the setup. Skip these dialogs by clicking their 'ok'
	// buttons.
	for i := 0; i < customPanelMaxCount; i++ {
		dialog := d.Object(ui.ID(dialogID))
		if err := dialog.WaitForExists(ctx, actionTimeout); err != nil {
			return nil
		}
		dismiss := d.Object(ui.ID(dismissID))
		if err := dismiss.Exists(ctx); err != nil {
			return errors.Wrap(err, "dismiss button not found")
		}
		if err := dismiss.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click the dismiss button")
		}
	}
	return errors.New("too many dialog popups")
}
