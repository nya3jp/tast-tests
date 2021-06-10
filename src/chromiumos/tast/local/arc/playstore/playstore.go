// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package playstore provides set of util functions used to install applications through the playstore.
package playstore

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func findAndDismissDialog(ctx context.Context, d *ui.Device, dialogText, buttonText string, timeout time.Duration) error {
	if err := d.Object(ui.TextMatches(dialogText)).Exists(ctx); err == nil {
		testing.ContextLogf(ctx, `%q popup found. Skipping`, dialogText)
		okButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)"+buttonText))
		if err := okButton.WaitForExists(ctx, timeout); err != nil {
			return err
		}
		if err := okButton.Click(ctx); err != nil {
			return err
		}
	}

	return nil
}

// InstallApp uses the Play Store to install an application.
// It will wait for the app to finish installing before returning.
// Play Store should be open to the homepage before running this function.
func InstallApp(ctx context.Context, a *arc.ARC, d *ui.Device, pkgName string, tryLimit int) error {
	const (
		defaultUITimeout = 20 * time.Second

		accountSetupText = "Complete account setup"
		permissionsText  = "needs access to"
		cantDownloadText = "Can.t download.*"
		cantInstallText  = "Can.t install.*"
		versionText      = "Your device isn.t compatible with this version."
		compatibleText   = "Your device is not compatible with this item."
		openMyAppsText   = "Please open my apps.*"

		acceptButtonText   = "accept"
		continueButtonText = "continue"
		gotItButtonText    = "got it"
		installButtonText  = "install"
		okButtonText       = "ok"
		openButtonText     = "open"
		playButtonText     = "play"
		retryButtonText    = "retry"
		skipButtonText     = "skip"

		intentActionView = "android.intent.action.VIEW"
	)

	installed, err := a.PackageInstalled(ctx, pkgName)
	if err != nil {
		return err
	}
	if installed {
		return nil
	}

	testing.ContextLog(ctx, "Opening Play Store with Intent")
	if err := a.WaitIntentHelper(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for ArcIntentHelper")
	}

	playStoreAppPageURI := "market://details?id=" + pkgName
	if err := a.SendIntentCommand(ctx, intentActionView, playStoreAppPageURI).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to send intent to open the Play Store")
	}

	// Wait for the app to install.
	testing.ContextLog(ctx, "Waiting for app to install")
	tries := 0
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		for _, val := range []struct {
			dialogText string
			buttonText string
		}{
			// Sometimes a dialog of "Can't download <app name>" pops up. Press Okay to
			// dismiss the dialog. This check needs to be done before checking the
			// install button since the install button exists underneath.
			{cantDownloadText, okButtonText},
			// Also press "Got it" button if ""Can't download <app name>" pops up.
			{cantDownloadText, gotItButtonText},
			// Similarly, press "Got it" button if "Can't install <app name>" dialog pops up.
			{cantInstallText, gotItButtonText},
			// Also, press Ok to dismiss the dialog if "Please open my apps" dialog pops up.
			{openMyAppsText, okButtonText},
			// When Play Store hits the rate limit it sometimes show "Your device is not compatible with this item." error.
			// This error is incorrect and should be ignored like the "Can't download <app name>" error.
			{compatibleText, okButtonText},
		} {
			if err := findAndDismissDialog(ctx, d, val.dialogText, val.buttonText, defaultUITimeout); err != nil {
				return testing.PollBreak(err)
			}
		}

		// If the version isn't compatible with the device, no install button will be available.
		// Fail immediately.
		if err := d.Object(ui.TextMatches(versionText)).Exists(ctx); err == nil {
			return testing.PollBreak(errors.New("app not compatible with this device"))
		}

		// If retry button appears, reopen the Play Store page by sending the same intent again.
		// (It tends to work better than clicking the retry button.)
		if err := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)"+retryButtonText)).Exists(ctx); err == nil {
			if tryLimit == -1 || tries < tryLimit {
				tries++
				testing.ContextLogf(ctx, "Retry button is shown. Trying to reopen the Play Store. Total attempts so far: %d", tries)
				if err := a.SendIntentCommand(ctx, intentActionView, playStoreAppPageURI).Run(testexec.DumpLogOnError); err != nil {
					return errors.Wrap(err, "failed to send intent to reopen the Play Store")
				}
			} else {
				return testing.PollBreak(errors.Errorf("reopen Play Store attempt limit of %d times", tryLimit))
			}
		}

		// If the install button is enabled, click it.
		installButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)"+installButtonText), ui.Enabled(true))
		if err := installButton.Exists(ctx); err == nil {
			// Limit number of tries to help mitigate Play Store rate limiting across test runs.
			if tryLimit == -1 || tries < tryLimit {
				tries++
				testing.ContextLogf(ctx, "Trying to hit the install button. Total attempts so far: %d", tries)
				if err := installButton.Click(ctx); err != nil {
					return err
				}
			} else {
				return testing.PollBreak(errors.Errorf("hit install attempt limit of %d times", tryLimit))
			}
		}

		// Complete account setup if necessary.
		if err := d.Object(ui.Text(accountSetupText)).Exists(ctx); err == nil {
			testing.ContextLog(ctx, "Completing account setup")
			continueButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)"+continueButtonText))
			if err := continueButton.WaitForExists(ctx, defaultUITimeout); err != nil {
				return testing.PollBreak(err)
			}
			if err := continueButton.Click(ctx); err != nil {
				return testing.PollBreak(err)
			}
			skipButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)"+skipButtonText))
			if err := skipButton.WaitForExists(ctx, defaultUITimeout); err != nil {
				return testing.PollBreak(err)
			}
			if err := skipButton.Click(ctx); err != nil {
				return testing.PollBreak(err)
			}
		}

		// Grant permissions if necessary.
		if err := findAndDismissDialog(ctx, d, permissionsText, acceptButtonText, defaultUITimeout); err != nil {
			return testing.PollBreak(err)
		}

		// Installation is complete once the open button or the play button is enabled.
		if err := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches(fmt.Sprintf("(?i)(%s|%s)", openButtonText, playButtonText)), ui.Enabled(true)).Exists(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for enabled open button or play button")
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second}); err != nil {
		return err
	}

	// Ensure that the correct package is installed, just in case the Play Store ui changes again.
	installed, err = a.PackageInstalled(ctx, pkgName)
	if err != nil {
		return err
	}
	if !installed {
		return errors.Errorf("failed to install %s", pkgName)
	}
	return nil
}
