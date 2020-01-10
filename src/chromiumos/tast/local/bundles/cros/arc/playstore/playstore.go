// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package playstore provides set of util functions used to install applications through the playstore.
package playstore

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/testing"
)

const (
	defaultUITimeout = 20 * time.Second
)

// SearchForApp uses the Play Store search bar to select an application.
// After searching, it will open the apps page.
// Play Store should be open to the homepage before running this function.
func SearchForApp(ctx context.Context, a *arc.ARC, d *ui.Device, pkgName string) error {
	const (
		searchIconID  = "com.android.vending:id/search_icon"
		searchInputID = "com.android.vending:id/search_bar_text_input"
		playCardID    = "com.android.vending:id/play_card"
	)

	// Wait for and click search icon.
	searchIcon := d.Object(ui.ID(searchIconID))
	if err := searchIcon.WaitForExists(ctx, defaultUITimeout); err != nil {
		return err
	}
	if err := searchIcon.Click(ctx); err != nil {
		return err
	}

	// Input search query.
	searchInput := d.Object(ui.ID(searchInputID))
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

	// Wait for and click play card.
	playCard := d.Object(ui.ID(playCardID))
	if err := playCard.WaitForExists(ctx, defaultUITimeout); err != nil {
		return err
	}
	return playCard.Click(ctx)
}

// InstallApp uses the Play Store to install an application.
// It will wait for the app to finish installing before returning.
// Play Store should be open to the homepage before running this function.
func InstallApp(ctx context.Context, a *arc.ARC, d *ui.Device, pkgName string) error {
	const (
		installButtonText  = "Install"
		continueButtonText = "CONTINUE"
		skipButtonText     = "SKIP"
		accountSetupText   = "Complete account setup"
		permissionsText    = "needs access to"
		acceptButtonID     = "com.android.vending:id/continue_button"
	)

	testing.ContextLog(ctx, "Searching for app")
	if err := SearchForApp(ctx, a, d, pkgName); err != nil {
		return errors.Wrap(err, "failed to search for app")
	}

	// Click install button.
	installButton := d.Object(ui.ClassName("android.widget.Button"), ui.Text(installButtonText))
	if err := installButton.WaitForExists(ctx, defaultUITimeout); err != nil {
		return err
	}
	if err := installButton.Click(ctx); err != nil {
		return err
	}

	// Wait for the app to install.
	testing.ContextLog(ctx, "Waiting for app to install")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
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

		// Check if installation complete.
		out, err := a.Command(ctx, "pm", "list", "packages").Output()
		if err != nil {
			return errors.Wrap(err, "failed to list packages")
		}
		wanted := "package:" + pkgName
		found := strings.Contains(string(out), wanted)
		if !found {
			return errors.New("package not installed yet")
		}
		return nil
	}, nil); err != nil {
		return err
	}
	return nil
}
