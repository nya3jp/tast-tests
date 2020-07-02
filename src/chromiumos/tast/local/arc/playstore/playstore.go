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
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// InstallApp uses the Play Store to install an application.
// It will wait for the app to finish installing before returning.
// Play Store should be open to the homepage before running this function.
func InstallApp(ctx context.Context, a *arc.ARC, d *ui.Device, pkgName string, tryLimit int) error {
	const (
		defaultUITimeout = 20 * time.Second

		accountSetupText = "Complete account setup"
		permissionsText  = "needs access to"
		cantDownloadText = "Can.t download.*"
		versionText      = "Your device isn.t compatible with this version."
		compatibleText   = "Your device is not compatible with this item."
		acceptButtonID   = "com.android.vending:id/continue_button"

		continueButtonText = "continue"
		installButtonText  = "install"
		openButtonText     = "open"
		okButtonText       = "ok"
		retryButtonText    = "retry"
		skipButtonText     = "skip"
	)

	testing.ContextLog(ctx, "Opening Play Store with Intent")
	if err := a.WaitIntentHelper(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for ArcIntentHelper")
	}
	if err := a.SendIntentCommand(ctx, "android.intent.action.VIEW", "market://details?id="+pkgName).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to send intent to open the Play Store")
	}

	// Wait for the app to install.
	testing.ContextLog(ctx, "Waiting for app to install")
	tries := 0
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Sometimes a dialog of "Can't download <app name>" pops up. Press Okay to
		// dismiss the dialog. This check needs to be done before checking the
		// install button since the install button exists underneath.
		if err := d.Object(ui.TextMatches(cantDownloadText)).Exists(ctx); err == nil {
			testing.ContextLog(ctx, `"Can't download" popup found. Skipping`)
			okButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)"+okButtonText))
			if err := okButton.WaitForExists(ctx, defaultUITimeout); err != nil {
				return testing.PollBreak(err)
			}
			if err := okButton.Click(ctx); err != nil {
				return testing.PollBreak(err)
			}
		}

		// When Play Store hits the rate limit it sometimes show "Your device is not compatible with this item." error.
		// This error is incorrect and should be ignored like the "Can't download <app name>" error.
		if err := d.Object(ui.TextMatches(compatibleText)).Exists(ctx); err == nil {
			testing.ContextLog(ctx, `"Item incompatibiltiy" popup found. Skipping`)
			okButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)"+okButtonText))
			if err := okButton.WaitForExists(ctx, defaultUITimeout); err != nil {
				return testing.PollBreak(err)
			}
			if err := okButton.Click(ctx); err != nil {
				return testing.PollBreak(err)
			}
		}

		// If the version isn't compatible with the device, no install button will be available.
		// Fail immediately.
		if err := d.Object(ui.TextMatches(versionText)).Exists(ctx); err == nil {
			return testing.PollBreak(errors.New("app not compatible with this device"))
		}

		// If retry button appears, clicking it tends not to fix the issue.
		// Simply notify the user of this for better error messages.
		if err := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)"+retryButtonText)).Exists(ctx); err == nil {
			return testing.PollBreak(errors.New("Play Store failed to load, retry button showing"))
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
		if err := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)"+openButtonText), ui.Enabled(true)).Exists(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for enabled open button")
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second}); err != nil {
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
