// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcent

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// EnsurePackagesUninstall verifies that packages have desired uninstall behavior.
func EnsurePackagesUninstall(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, packages []string, shouldUninstall bool) error {
	assertUninstall := func(isUninstalled bool, packageName string) error {

		action := "cannot"
		if isUninstalled {
			action = "can"
		}

		message := fmt.Sprintf("Package %q %s be uninstalled", packageName, action)

		if isUninstalled == shouldUninstall {
			testing.ContextLog(ctx, message)
			return nil
		}
		return errors.New(message)
	}

	testing.ContextLog(ctx, "Trying to uninstall packages")
	for _, p := range packages {
		err := a.Uninstall(ctx, p)
		isUninstalled := err == nil
		if err := assertUninstall(isUninstalled, p); err != nil {
			return err
		}
	}

	return nil
}

// WaitForUninstall waits for package to uninstall.
func WaitForUninstall(ctx context.Context, a *arc.ARC, blockedPackage string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if installed, err := a.PackageInstalled(ctx, blockedPackage); err != nil {
			return testing.PollBreak(err)
		} else if installed {
			return errors.New("Package not yet uninstalled")
		}
		return nil
	}, &testing.PollOptions{Interval: 1 * time.Second})
}

// DumpBugReportOnError dumps bug report on error.
func DumpBugReportOnError(ctx context.Context, hasError func() bool, a *arc.ARC, filePath string) {
	if !hasError() {
		return
	}

	testing.ContextLog(ctx, "Dumping Bug Report")
	if err := a.BugReport(ctx, filePath); err != nil {
		testing.ContextLog(ctx, "Failed to get bug report: ", err)
	}
}

// ConfigureProvisioningLogs enables verbose logging for important modules and increases the log buffer size.
func ConfigureProvisioningLogs(ctx context.Context, a *arc.ARC) error {
	verboseTags := []string{"clouddpc", "Finsky", "Volley", "PlayCommon"}
	if err := a.EnableVerboseLogging(ctx, verboseTags...); err != nil {
		return err
	}
	return IncreaseLogcatBufferSize(ctx, a)
}

// IncreaseLogcatBufferSize increases the log buffer size to 10 MB.
func IncreaseLogcatBufferSize(ctx context.Context, a *arc.ARC) error {
	return a.Command(ctx, "logcat", "-G", "10M").Run(testexec.DumpLogOnError)
}

// WaitForProvisioning waits for provisioning to finish and dumps logcat if doesn't.
func WaitForProvisioning(ctx context.Context, a *arc.ARC, attempt int) error {
	// CloudDPC sign-in timeout set in code is 3 minutes.
	const provisioningTimeout = 3 * time.Minute

	if err := a.WaitForProvisioning(ctx, provisioningTimeout); err != nil {
		if err := optin.DumpLogCat(ctx, strconv.Itoa(attempt)); err != nil {
			testing.ContextLogf(ctx, "WARNING: Failed to dump logcat: %s", err)
		}
		return err
	}
	return nil

}
