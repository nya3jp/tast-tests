// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/setup"
	"chromiumos/tast/local/testexec"
)

type installApp struct {
	ctx         context.Context
	a           *ARC
	apkDataPath string
	pkg         string
}

// Setup installs an APK.
func (a *installApp) Setup() error {
	return a.a.Install(a.ctx, a.apkDataPath)
}

// Cleanup uninstalls the previously installed package.
func (a *installApp) Cleanup() error {
	return a.a.Uninstall(a.ctx, a.pkg)
}

// InstallApp creates a setup.SetupAction that installs an APK.
func InstallApp(ctx context.Context, a *ARC, apkDataPath string, pkg string) setup.SetupAction {
	return &installApp{ctx, a, apkDataPath, pkg}
}

type startActivity struct {
	ctx      context.Context
	a        *ARC
	pkg      string
	activity string
}

// Setup starts an Android Activity.
func (a *startActivity) Setup() error {
	fullActivity := a.pkg + "/" + a.activity
	if err := a.a.Command(a.ctx, "am", "start", "-W", fullActivity).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to start activity %q", fullActivity)
	}
	return nil
}

// Cleanup stops all activities for a package.
func (a *startActivity) Cleanup() error {
	if err := a.a.Command(a.ctx, "am", "force-stop", a.pkg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to stop package %q", a.pkg)
	}
	return nil
}

// StartActivity creates a setup.SetupAction that starts an activity.
func StartActivity(ctx context.Context, a *ARC, pkg string, activity string) setup.SetupAction {
	return &startActivity{ctx, a, pkg, activity}
}

type disableSELinux struct {
	ctx     context.Context
	notOnVM bool
}

// Setup disables SELinux enforcement.
func (a *disableSELinux) Setup() error {
	output, err := testexec.CommandContext(a.ctx, "getenforce").Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to read SELinux enforcement")
	}
	trimmed := strings.TrimSpace(string(output))
	if trimmed != "Enforcing" {
		return errors.Errorf("selinux not Enforcing %q", trimmed)
	}
	vmEnabled, err := VMEnabled()
	if err != nil {
		return errors.Wrap(err, "failed to check if VM is enabled")
	}
	if vmEnabled && a.notOnVM {
		// We only need to disable SELinux enforcement to be able to read
		// available memory in ARC++.
		return nil
	}
	if err := testexec.CommandContext(a.ctx, "setenforce", "0").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to disable SELinux enforcement")
	}
	return nil
}

// Cleanup reenabled SELinux enforcement.
func (a *disableSELinux) Cleanup() error {
	if err := testexec.CommandContext(a.ctx, "setenforce", "1").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to reenable SELinux enforcement")
	}
	return nil
}

// DisableSELinux creates a setup.SetupAction that disables SELinux
// enfrocement. If notOnVM is true, SELinux is not disabled when running ARCVM.
func DisableSELinux(ctx context.Context, notOnVM bool) setup.SetupAction {
	return &disableSELinux{ctx, notOnVM}
}
