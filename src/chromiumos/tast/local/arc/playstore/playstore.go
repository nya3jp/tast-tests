// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package playstore provides set of util functions used to install applications through the playstore.
package playstore

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/testing"
)

const (
	defaultUITimeout = 20 * time.Second
	// The install button has different text depending on playstore version.
	installButtonRegex = "Install|INSTALL"
)

// SearchForApp uses the Play Store search bar to select an application.
// After searching, it will open the apps page.
// Play Store should be open to the homepage before running this function.
func SearchForApp(ctx context.Context, a *arc.ARC, d *ui.Device, pkgName string) error {
	const (
		searchBarID       = "com.android.vending:id/search_bar"
		searchContainerID = "com.android.vending:id/play_search_container"
		searchBarInputID  = "com.android.vending:id/search_bar_text_input"
		searchBoxInputID  = "com.android.vending:id/search_box_text_input"
		playCardID        = "com.android.vending:id/play_card"
	)

	// There are 2 versions of the playstore ui.
	// One version has a search bar and the other has a search container.
	// This selects the correct playstore flow.
	searchBar := d.Object(ui.ID(searchBarID))
	searchContainer := d.Object(ui.ID(searchContainerID))
	var searchInput *ui.Object
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := searchBar.Exists(ctx); err == nil {
			if err := searchBar.Click(ctx); err != nil {
				return testing.PollBreak(err)
			}
			searchInput = d.Object(ui.ID(searchBarInputID))
			return nil
		}

		if err := searchContainer.Exists(ctx); err == nil {
			if err := searchContainer.Click(ctx); err != nil {
				return testing.PollBreak(err)
			}
			searchInput = d.Object(ui.ID(searchBoxInputID))
			return nil
		}
		return errors.New("search bar not found")
	}, &testing.PollOptions{Timeout: defaultUITimeout}); err != nil {
		return err
	}

	// Input search query.
	if err := searchInput.WaitForExists(ctx, defaultUITimeout); err != nil {
		return err
	}
	if err := searchInput.Click(ctx); err != nil {
		return err
	}
	if err := searchInput.SetText(ctx, pkgName); err != nil {
		return err
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		return err
	}

	// Some versions of the Play Store automatically select the app when you search for it.
	// If the install button is not present, click the playcard to select the app.
	installButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches(installButtonRegex))
	if err := installButton.WaitForExists(ctx, defaultUITimeout); err != nil {
		testing.ContextLog(ctx, "Install button not found, selecting playcard to complete search")
		playCard := d.Object(ui.ID(playCardID))
		if err := playCard.WaitForExists(ctx, defaultUITimeout); err != nil {
			return err
		}
		return playCard.Click(ctx)
	}
	return nil
}

// InstallApp uses the Play Store to install an application.
// It will wait for the app to finish installing before returning.
// Play Store should be open to the homepage before running this function.
func InstallApp(ctx context.Context, a *arc.ARC, d *ui.Device, pkgName string) error {
	const (
		accountSetupText   = "Complete account setup"
		continueButtonText = "CONTINUE"
		skipButtonText     = "SKIP"
		permissionsText    = "needs access to"
		acceptButtonID     = "com.android.vending:id/continue_button"
		// The open button has different text depending on playstore version.
		openButtonRegex = "Open|OPEN"
	)

	testing.ContextLog(ctx, "Searching for app")
	if err := SearchForApp(ctx, a, d, pkgName); err != nil {
		return errors.Wrap(err, "failed to search for app")
	}

	// Wait for the app to install.
	testing.ContextLog(ctx, "Waiting for app to install")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// If the install button is enabled, click it.
		installButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches(installButtonRegex), ui.Enabled(true))
		if err := installButton.Exists(ctx); err == nil {
			if err := installButton.Click(ctx); err != nil {
				return err
			}
		}

		// Complete account setup if necessary.
		if err := d.Object(ui.Text(accountSetupText)).Exists(ctx); err == nil {
			testing.ContextLog(ctx, "Completing account setup")
			continueButton := d.Object(ui.ClassName("android.widget.Button"), ui.Text(continueButtonText))
			if err := continueButton.WaitForExists(ctx, defaultUITimeout); err != nil {
				return testing.PollBreak(err)
			}
			if err := continueButton.Click(ctx); err != nil {
				return testing.PollBreak(err)
			}
			skipButton := d.Object(ui.ClassName("android.widget.Button"), ui.Text(skipButtonText))
			if err := skipButton.WaitForExists(ctx, defaultUITimeout); err != nil {
				return testing.PollBreak(err)
			}
			if err := skipButton.Click(ctx); err != nil {
				return testing.PollBreak(err)
			}
		}

		// Grant permissions if necessary.
		if err := d.Object(ui.Text(permissionsText)).Exists(ctx); err == nil {
			testing.ContextLog(ctx, "Accepting app permissions")
			acceptButton := d.Object(ui.ID(acceptButtonID))
			if err := acceptButton.WaitForExists(ctx, defaultUITimeout); err != nil {
				return testing.PollBreak(err)
			}
			if err := acceptButton.Click(ctx); err != nil {
				return testing.PollBreak(err)
			}
		}

		// Installation is complete once the open button is enabled.
		if err := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches(openButtonRegex), ui.Enabled(true)).Exists(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for enabled open button")
		}
		return nil
	}, nil); err != nil {
		return err
	}

	// Ensure that the correct package is installed, just in case the Play Store ui changes again.
	pkgs, err := a.InstalledPackages(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list packages")
	}

	if _, ok := pkgs[pkgName]; !ok {
		return errors.Errorf("failed to install %s", pkgName)
	}
	return nil
}
