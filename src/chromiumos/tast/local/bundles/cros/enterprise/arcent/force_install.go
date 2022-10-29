// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcent provides enterprise test related ARC utilities.
package arcent

import (
	"context"
	"fmt"
	"sort"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

// LoginPoolVar is the account pool information
const LoginPoolVar = "arc.managedAccountPool"

// InstallTypeForceInstalled is the install type for app that is force-installed.
const InstallTypeForceInstalled = "FORCE_INSTALLED"

// InstallTypeAvailable is the install type for app that is allowed.
const InstallTypeAvailable = "AVAILABLE"

// InstallTypeBlocked is the install type for app that is blocked.
const InstallTypeBlocked = "BLOCKED"

// SetupPolicyServerWithArcApps sets up a fake policy server with ARC enabled and a list of packages with the corresponding install type
func SetupPolicyServerWithArcApps(ctx context.Context, outDir, policyUser string, packages []string, installType string) (fdms *fakedms.FakeDMS, retErr error) {
	arcPolicy := CreateArcPolicyWithApps(packages, installType)
	arcEnabledPolicy := &policy.ArcEnabled{Val: true}
	policies := []policy.Policy{arcEnabledPolicy, arcPolicy}

	return policyutil.SetUpFakePolicyServer(ctx, outDir, policyUser, policies)
}

// VerifyArcPolicyForceInstalled matches ArcPolicy FORCE_INSTALLED apps list with expected packages.
func VerifyArcPolicyForceInstalled(ctx context.Context, tconn *chrome.TestConn, forceInstalledPackages []string) error {
	dps, err := policyutil.PoliciesFromDUT(ctx, tconn)
	if err != nil {
		return err
	}

	expected := &policy.ArcPolicy{}
	actual, ok := dps.Chrome[expected.Name()]
	if !ok {
		return errors.New("no such a policy ArcPolicy")
	}
	value, err := expected.UnmarshalAs(actual.ValueJSON)
	if err != nil {
		return err
	}
	arcPolicyValue, ok := value.(policy.ArcPolicyValue)
	if !ok {
		return errors.Errorf("cannot extract ArcPolicyValue %s", value)
	}

	apps := arcPolicyValue.Applications
	forceInstalled := make(map[string]bool)
	for _, application := range apps {
		packageName := application.PackageName
		installType := application.InstallType
		if installType == InstallTypeForceInstalled {
			forceInstalled[packageName] = true
		}
	}
	for _, p := range forceInstalledPackages {
		if !forceInstalled[p] {
			return errors.Errorf("the next package is not FORCE_INSTALLED by policy: %s", p)
		}
		delete(forceInstalled, p)
	}

	if len(forceInstalled) != 0 {
		testing.ContextLogf(ctx, "WARNING: Extra FORCE_INSTALLED packages in ArcPolicy that can cause the test to timeout: %s", makeList(forceInstalled))
	}

	return nil
}

// makeList returns a list of keys from map.
// TODO: there's several duplication of makeList. Unify them.
func makeList(packages map[string]bool) []string {
	var packagesList []string
	for pkg := range packages {
		packagesList = append(packagesList, pkg)
	}
	sort.Strings(packagesList)
	return packagesList
}

// CreateArcPolicyWithApps creates a policy with specified packages as force installs.
func CreateArcPolicyWithApps(packages []string, installType string) *policy.ArcPolicy {
	var forceInstalledApps []policy.Application
	for _, packageName := range packages {
		forceInstalledApps = append(forceInstalledApps, policy.Application{
			PackageName: packageName,
			InstallType: installType,
		})
	}
	arcPolicy := &policy.ArcPolicy{
		Val: &policy.ArcPolicyValue{
			Applications:              forceInstalledApps,
			PlayLocalPolicyEnabled:    true,
			PlayEmmApiInstallDisabled: true,
		},
	}

	return arcPolicy
}

// EnsurePlayStoreNotEmpty ensures that the asset browser does not display empty screen.
func EnsurePlayStoreNotEmpty(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, a *arc.ARC, outDir string, runID int) (retErr error) {
	const (
		searchBarTextStart = "Search for apps"
		emptyPlayStoreText = "No results found."
		serverErrorText    = "Server error|Error.*server.*"
		tryAgainButtonText = "Try again"
	)

	defer faillog.SaveScreenshotToFileOnError(ctx, cr, outDir, func() bool {
		return retErr != nil
	}, fmt.Sprintf("play_store_%d.png", runID))

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize UI Automator")
	}
	defer d.Close(ctx)

	return testing.Poll(ctx, func(ctx context.Context) error {
		// if GMS Core updates after launch, it can cause Play Store to be closed so we have to
		// launch it again.
		act, err := playstore.LaunchAssetBrowserActivity(ctx, tconn, a)
		if err != nil {
			return err
		}
		defer act.Close()

		return testing.Poll(ctx, func(ctx context.Context) error {
			if running, err := act.IsRunning(ctx); err != nil {
				return testing.PollBreak(err)
			} else if !running {
				return testing.PollBreak(errors.New("Play Store closed"))
			}

			if err := d.Object(ui.Text(emptyPlayStoreText)).Exists(ctx); err == nil {
				return errors.New("Play Store is empty")
			}

			if err := playstore.FindAndDismissDialog(ctx, d, serverErrorText, tryAgainButtonText, 2*time.Second); err != nil {
				return testing.PollBreak(err)
			}

			if err := d.Object(ui.TextStartsWith(searchBarTextStart)).Exists(ctx); err != nil {
				return errors.Wrap(err, "Play Store UI screen not shown")
			}

			return nil
		}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 30 * time.Second})
	}, &testing.PollOptions{Interval: 10 * time.Second})
}
