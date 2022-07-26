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
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// packageInfo contains info needed to launch and open an ARC application.
type packageInfo struct {
	name       string
	query      string // query is the search query used find the package in the launcher.
	skipSplash action.Action
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
		if _, ok := pkgs[pkg.name]; ok {
			testing.ContextLogf(ctx, "%s is already installed", pkg.name)
			continue
		}

		testing.ContextLog(ctx, "Installing: ", pkg.name)
		openedPlayStore = true
		installCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		if err = playstore.InstallApp(installCtx, a, d, pkg.name, &playstore.Options{TryLimit: -1}); err != nil {
			cancel()
			return errors.Wrapf(err, "failed to install %s", pkg.name)
		}
		cancel()
	}

	if !openedPlayStore {
		return nil
	}

	if err := optin.ClosePlayStore(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to close Play Store")
	}

	if err := ash.WaitForHidden(ctx, tconn, playStorePackageName); err != nil {
		return errors.Wrap(err, "failed to wait for Play Store to disappear")
	}
	return nil
}

// launchPackages launches each package in |packages| by calling
// |openAppList|, searching for the package name, and clicking on the
// first result. This returns the number of packages that were opened.
func launchPackages(ctx context.Context, tconn *chrome.TestConn, kw *input.KeyboardEventWriter, ac *uiauto.Context, packages []packageInfo) (int, error) {
	// Keep track of the initial number of windows, to ensure
	// we open the right number of windows.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get window list")
	}
	initialNumWindows := len(ws)

	launchCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	for _, pkg := range packages {
		testing.ContextLog(ctx, "Launching: ", pkg.name)
		if err := launcher.SearchAndLaunch(tconn, kw, pkg.query)(launchCtx); err != nil {
			return 0, errors.Wrapf(err, "failed to search and launch %s", pkg.query)
		}

		if err := ash.WaitForVisible(launchCtx, tconn, pkg.name); err != nil {
			return 0, errors.Wrapf(err, "failed to wait for %s to be visible", pkg.name)
		}

		testing.ContextLog(ctx, "Skipping the splash screen of ", pkg.query)
		if err := pkg.skipSplash(launchCtx); err != nil {
			return 0, errors.Wrapf(err, "failed to skip splash screen for %s", pkg.query)
		}

		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return 0, errors.Wrap(err, "wait for windows to stabilize")
		}
	}

	if ws, err := ash.GetAllWindows(ctx, tconn); err != nil {
		return 0, errors.Wrap(err, "failed to get window list after opening ARC apps")
	} else if expectedNumWindows := initialNumWindows + len(packages); len(ws) != expectedNumWindows {
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
		return errors.Wrap(err, "failed to dismiss compatibility splash")
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
		// dismiss the warning dialog and move on.
		if err := testing.Sleep(ctx, actionTimeout); err != nil {

		}
		okButton := d.Object(ui.ClassName("android.widget.Button"), ui.Text("OK"))
		if err := okButton.WaitForExists(ctx, actionTimeout); err != nil {
			return errors.Wrap(err, "failed to wait for the email address to appear")
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
	// finalize the setup. Skip these dialogs by clicking their 'ok' buttons.
	for i := 0; i < customPanelMaxCount; i++ {
		dialog := d.Object(ui.ID(dialogID))
		if err := dialog.WaitForExists(ctx, actionTimeout); err != nil {
			return nil
		}
		dismiss := d.Object(ui.ID(dismissID))
		if err := dismiss.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click the dismiss button")
		}
	}
	return errors.New("too many dialog popups")
}
