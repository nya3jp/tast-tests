// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
)

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
		if forceInstalled[p] {
			delete(forceInstalled, p)
		} else {
			return errors.Errorf("the next package is not FORCE_INSTALLED by policy: %s", p)
		}
	}
	if len(forceInstalled) != 0 {
		return errors.Errorf("Extra FORCE_INSTALLED packages in ArcPolicy: %s", makeList(forceInstalled))
	}
	return nil
}
