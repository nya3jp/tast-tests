// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

// InstallApp installs an Android APK.
func InstallApp(ctx context.Context, a *arc.ARC, apkDataPath string, pkg string, chain CleanupChain) (CleanupChain, error) {
	setupFailed, guard := SetupFailureGuard(chain)
	defer guard(ctx)

	if err := a.Install(ctx, apkDataPath); err != nil {
		return nil, errors.Wrapf(err, "failed to install apk %q", apkDataPath)
	}
	testing.ContextLogf(ctx, "Installed Android app %q", pkg)

	return SetupSucceeded(setupFailed, chain, func(ctx context.Context) error {
		if err := a.Uninstall(ctx, pkg); err != nil {
			return errors.Wrapf(err, "failed to uninstall package %q", pkg)
		}
		testing.ContextLogf(ctx, "Uninstalled Android app %q", pkg)
		return nil
	})
}

// StartActivity starts an Android activity.
func StartActivity(ctx context.Context, a *arc.ARC, pkg string, activityName string, chain CleanupChain) (CleanupChain, error) {
	setupFailed, guard := SetupFailureGuard(chain)
	defer guard(ctx)

	activity, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create activity %q in package %q", activityName, pkg)
	}
	if err := activity.Start(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to start activity %q in package %q", activityName, pkg)
	}
	testing.ContextLogf(ctx, "Started activity %s/%s", pkg, activityName)

	return SetupSucceeded(setupFailed, chain, func(ctx context.Context) error {
		defer activity.Close()
		if err := activity.Stop(ctx); err != nil {
			errors.Wrapf(err, "failed to stop activity in package %q", pkg)
		}
		testing.ContextLogf(ctx, "Stopped activities in package %s", pkg)
		return nil
	})
}
