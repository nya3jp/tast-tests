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
func InstallApp(ctx context.Context, a *arc.ARC, d *ui.Device, pkgName string) error {
	const (
		defaultUITimeout = 20 * time.Second

		accountSetupText = "Complete account setup"
		permissionsText  = "needs access to"
		acceptButtonID   = "com.android.vending:id/continue_button"

		continueButtonText = "continue"
		installButtonText  = "install"
		openButtonText     = "open"
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
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// If the install button is enabled, click it.
		installButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)"+installButtonText), ui.Enabled(true))
		if err := installButton.Exists(ctx); err == nil {
			if err := installButton.Click(ctx); err != nil {
				return err
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
