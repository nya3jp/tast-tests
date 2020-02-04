// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
)

type installApp struct {
	ctx         context.Context
	a           *arc.ARC
	apkDataPath string
	pkg         string
}

// Setup installs an Android APK.
func (a *installApp) Setup() error {
	return a.a.Install(a.ctx, a.apkDataPath)
}

// Cleanup uninstalls the previously installed APK.
func (a *installApp) Cleanup() error {
	return a.a.Uninstall(a.ctx, a.pkg)
}

// InstallApp creates a setup action to install an android app.
func InstallApp(ctx context.Context, a *arc.ARC, apkDataPath string, pkg string) Action {
	return &installApp{ctx, a, apkDataPath, pkg}
}

type startActivity struct {
	ctx      context.Context
	a        *arc.ARC
	pkg      string
	activity string
}

// Setup starts an android activity.
func (a *startActivity) Setup() error {
	fullActivity := a.pkg + "/" + a.activity
	if err := a.a.Command(a.ctx, "am", "start", "-W", fullActivity).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to start activity %q", fullActivity)
	}
	return nil
}

// Cleanup stops all activities for a given package.
func (a *startActivity) Cleanup() error {
	if err := a.a.Command(a.ctx, "am", "force-stop", a.pkg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to stop package %q", a.pkg)
	}
	return nil
}

// StartActivity creates a setup action to start an Android activity.
func StartActivity(ctx context.Context, a *arc.ARC, pkg string, activity string) Action {
	return &startActivity{ctx, a, pkg, activity}
}
