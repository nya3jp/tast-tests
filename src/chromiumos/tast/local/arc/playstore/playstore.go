// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package playstore provides set of util functions used to install applications through the playstore.
package playstore

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type operation string

const (
	installApp operation = "install"
	updateApp  operation = "update"
)

// Options contains options used when installing or updating an app.
type Options struct {
	// TryLimit limits number of tries to install or update an app.
	// Default value is 3, and -1 means unlimited.
	TryLimit int

	// DefaultUITimeout is used when waiting for UI elements.
	// Default value is 20 sec.
	DefaultUITimeout time.Duration

	// ShortUITimeout is used when waiting for "Complete account setup" button.
	// Default value is 10 sec.
	ShortUITimeout time.Duration

	// InstallationTimeout is used when waiting for app installation.
	// Default value is 90 sec.
	InstallationTimeout time.Duration
}

// FindAndDismissDialog finds a dialog containing text with a corresponding button and presses the button.
func FindAndDismissDialog(ctx context.Context, d *ui.Device, dialogText, buttonText string, timeout time.Duration) error {
	if err := d.Object(ui.TextMatches("(?i)"+dialogText)).WaitForExists(ctx, time.Second); err == nil {
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

// printPercentageOfAppInstalled func prints the percentage of app installed so far.
func printPercentageOfAppInstalled(ctx context.Context, d *ui.Device) {
	const (
		currentInstallPercentInGBText = ".*GB"
		currentInstallPercentInMBText = ".*MB"
		currentPerInfoClassName       = "android.widget.TextView"
	)
	for _, val := range []struct {
		currentPercentInfoClassName string
		currentInstallPercentInText string
	}{
		{currentPerInfoClassName, currentInstallPercentInMBText},
		{currentPerInfoClassName, currentInstallPercentInGBText},
	} {
		currPerInfo := d.Object(ui.ClassName(val.currentPercentInfoClassName), ui.TextMatches("(?i)"+val.currentInstallPercentInText))
		if err := currPerInfo.WaitForExists(ctx, time.Second); err == nil {
			getInfo, err := currPerInfo.GetText(ctx)
			if err == nil {
				testing.ContextLogf(ctx, "Percentage of app installed so far: %v ", getInfo)
			}
		}
	}
}

// installOrUpdate uses the Play Store to install or update an application.
func installOrUpdate(ctx context.Context, a *arc.ARC, d *ui.Device, pkgName string, opt *Options, op operation) error {
	const (
		accountSetupText          = "Complete account setup"
		permissionsText           = "needs access to"
		cantDownloadText          = "Can.t download.*"
		cantInstallText           = "Can.t install.*"
		versionText               = "Your device isn.t compatible with this version."
		compatibleText            = "Your device is not compatible with this item."
		openMyAppsText            = "Please open my apps.*"
		termsOfServiceText        = "Terms of Service"
		linkPaypalAccountText     = "Want to link your PayPal account.*"
		installAppsFromDeviceText = "Install apps from your devices"
		serverBusyText            = "Server busy, please try again later."
		internalProblemText       = "There.s an internal problem with your device.*"

		acceptButtonText   = "accept"
		continueButtonText = "continue"
		gotItButtonText    = "got it"
		installButtonText  = "install"
		updateButtonText   = "update"
		okButtonText       = "ok"
		openButtonText     = "open"
		playButtonText     = "play"
		retryButtonText    = "retry"
		tryAgainButtonText = "try again"
		skipButtonText     = "skip"
		noThanksButtonText = "No thanks"

		intentActionView = "android.intent.action.VIEW"
	)

	o := *opt
	tryLimit := 3
	if o.TryLimit != 0 {
		tryLimit = o.TryLimit
	}
	defaultUITimeout := 20 * time.Second
	if o.DefaultUITimeout != 0 {
		defaultUITimeout = o.DefaultUITimeout
	}
	shortUITimeout := 10 * time.Second
	if o.ShortUITimeout != 0 {
		shortUITimeout = o.ShortUITimeout
	}
	installationTimeout := 90 * time.Second
	if o.InstallationTimeout != 0 {
		installationTimeout = o.InstallationTimeout
	}
	testing.ContextLogf(ctx, "Using TryLimit=%d, DefaultUITimeout=%s, ShortUITimeout=%s, InstallationTimeout=%s",
		tryLimit, defaultUITimeout, shortUITimeout, installationTimeout)

	testing.ContextLog(ctx, "Opening Play Store with Intent")
	if err := a.WaitIntentHelper(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for ArcIntentHelper")
	}

	playStoreAppPageURI := "market://details?id=" + pkgName
	if err := a.SendIntentCommand(ctx, intentActionView, playStoreAppPageURI).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to send intent to open the Play Store")
	}

	var opButton *ui.Object // Operation button - install or update.
	switch op {
	case installApp:
		// Look for install button.
		opButton = d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)"+installButtonText), ui.Enabled(true))
	case updateApp:
		// Look for update button.
		opButton = d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)"+updateButtonText), ui.Enabled(true))
	default:
		return errors.Errorf("operation %s is not supported", op)
	}

	// Wait for the app to install or update.
	testing.ContextLogf(ctx, "Waiting for app to %s", op)

	tries := 0
	return testing.Poll(ctx, func(ctx context.Context) error {
		for _, val := range []struct {
			dialogText string
			buttonText string
		}{
			// Sometimes a dialog of "Can't download <app name>" pops up. Press "Got it" to
			// dismiss the dialog. This check needs to be done before checking the
			// install button since the install button exists underneath.
			{cantDownloadText, gotItButtonText},
			// Similarly, press "Got it" button if "Can't install <app name>" dialog pops up.
			{cantInstallText, gotItButtonText},
			// Also, press Ok to dismiss the dialog if "Please open my apps" dialog pops up.
			{openMyAppsText, okButtonText},
			// Also, press "NO THANKS" to dismiss the dialog if "Install apps from your devices" dialog pops up.
			{installAppsFromDeviceText, noThanksButtonText},
			// When Play Store hits the rate limit it sometimes show "Your device is not compatible with this item." error.
			// This error is incorrect and should be ignored like the "Can't download <app name>" error.
			{compatibleText, okButtonText},
			// Somehow, playstore shows a ToS dialog upon opening even after playsore
			// optin finishes. Click "accept" button to accept and dismiss.
			{termsOfServiceText, acceptButtonText},
			// Press "Try again" if "Server busy, please try again later." screen is shown.
			{serverBusyText, tryAgainButtonText},
			// Press Ok to dismiss the dialog if "There\'s an internal problem with your device" dialog pops up.
			{internalProblemText, okButtonText},
		} {
			if err := FindAndDismissDialog(ctx, d, val.dialogText, val.buttonText, defaultUITimeout); err != nil {
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
		if err := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches(fmt.Sprintf("(?i)(%s|%s)", retryButtonText, tryAgainButtonText))).Exists(ctx); err == nil {
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

		// If the install or update button is enabled, click it.
		if err := opButton.Exists(ctx); err == nil {
			// Limit number of tries to help mitigate Play Store rate limiting across test runs.
			if tryLimit == -1 || tries < tryLimit {
				tries++
				testing.ContextLogf(ctx, "Trying to hit the %s button. Total attempts so far: %d", op, tries)
				if err := opButton.Click(ctx); err != nil {
					return err
				}
			} else {
				return testing.PollBreak(errors.Errorf("hit %s attempt limit of %d times", op, tryLimit))
			}
		}

		// Grant permissions if necessary.
		if err := FindAndDismissDialog(ctx, d, permissionsText, acceptButtonText, defaultUITimeout); err != nil {
			return testing.PollBreak(err)
		}

		// Handle "Want to link your PayPal account" if necessary.
		testing.ContextLogf(ctx, "Checking existence of : %s", linkPaypalAccountText)
		if err := d.Object(ui.TextMatches("(?i)"+linkPaypalAccountText), ui.Enabled(true)).WaitForExists(ctx, defaultUITimeout); err == nil {
			testing.ContextLog(ctx, "Want to link your paypal account does exist")
			noThanksButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)"+noThanksButtonText))
			if err := noThanksButton.WaitForExists(ctx, defaultUITimeout); err != nil {
				return testing.PollBreak(err)
			}
			if err := noThanksButton.Click(ctx); err != nil {
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

		// Complete account setup if necessary.
		testing.ContextLogf(ctx, "Checking existence of : %s", accountSetupText)
		if err := d.Object(ui.Text(accountSetupText), ui.Enabled(true)).WaitForExists(ctx, shortUITimeout); err == nil {
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
		if err := FindAndDismissDialog(ctx, d, permissionsText, acceptButtonText, defaultUITimeout); err != nil {
			return testing.PollBreak(err)
		}

		// Wait until progress bar is gone.
		testing.ContextLog(ctx, "Checking existence of progress bar")
		progressBar := d.Object(ui.ClassName("android.widget.ProgressBar"))
		if err := progressBar.WaitForExists(ctx, defaultUITimeout); err == nil {
			// Print the percentage of app installed so far.
			printPercentageOfAppInstalled(ctx, d)
			testing.ContextLog(ctx, "Wait until progress bar is gone")
			if err := progressBar.WaitUntilGone(ctx, installationTimeout); err != nil {
				return errors.Wrap(err, "progress bar still exists")
			}
		}

		// Make sure we are still on the Play Store installation page by checking whether the "open" or "play" button exists.
		// If not, reopen the Play Store page by sending the same intent again.
		if err := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches(fmt.Sprintf("(?i)(%s|%s)", openButtonText, playButtonText))).Exists(ctx); err != nil {
			testing.ContextLog(ctx, "App installation page disappeared; reopen it")
			if err := a.SendIntentCommand(ctx, intentActionView, playStoreAppPageURI).Run(testexec.DumpLogOnError); err != nil {
				return errors.Wrap(err, "failed to send intent to reopen the Play Store")
			}
		}

		installed, err := a.PackageInstalled(ctx, pkgName)
		if err != nil {
			return errors.Wrap(err, "failed to check app installation status")
		}
		if !installed {
			return errors.New("app not yet installed")
		}

		return nil
	}, &testing.PollOptions{Interval: time.Second})
}

// InstallApp uses the Play Store to install an application.
// It will wait for the app to finish installing before returning.
// Play Store should be open to the homepage before running this function.
func InstallApp(ctx context.Context, a *arc.ARC, d *ui.Device, pkgName string, opt *Options) error {
	installed, err := a.PackageInstalled(ctx, pkgName)
	if err != nil {
		return err
	}
	if installed {
		return nil
	}

	if err := installOrUpdate(ctx, a, d, pkgName, opt, installApp); err != nil {
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

// InstallOrUpdateAppAndClose installs or updates an application via Play Store, closes Play Store after installation.
// If the application is already installed, it updates the app if an update is available.
// It will wait for the app to finish installing/updating and closes Play Store before returning.
func InstallOrUpdateAppAndClose(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, pkgName string, opt *Options) error {
	installed, err := a.PackageInstalled(ctx, pkgName)
	if err != nil {
		return err
	}

	var installOperation operation
	if installed {
		testing.ContextLog(ctx, "App has already been installed; check if an update is available")
		installOperation = updateApp
	} else {
		testing.ContextLog(ctx, "App is not installed yet; check if an installation is available")
		installOperation = installApp
	}

	if err := installOrUpdate(ctx, a, d, pkgName, opt, installOperation); err != nil {
		return err
	}
	return optin.ClosePlayStore(ctx, tconn)
}
