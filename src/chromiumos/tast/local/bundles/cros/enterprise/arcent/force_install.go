// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcent provides enterprise test related ARC utilities.
package arcent

import (
	"context"
	"sort"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

// LoginPoolVar is the account pool information
const LoginPoolVar = "arc.managedAccountPool"

// InstallTypeForceInstalled is the install type for app that is force-installed.
const InstallTypeForceInstalled = "FORCE_INSTALLED"

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
		if installType == "FORCE_INSTALLED" {
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
