// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// GrantAndroidPermission grants an Android permission to an app.
func GrantAndroidPermission(ctx context.Context, a *arc.ARC, pkg, permission string) (CleanupCallback, error) {
	testing.ContextLogf(ctx, "Granting permission %q to Android app %q", permission, pkg)
	if err := a.Command(ctx, "pm", "grant", pkg, permission).Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrapf(err, "failed to grant permission %q to %q", permission, pkg)
	}

	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Revoking permission %q from Android app %q", permission, pkg)
		return a.Command(ctx, "pm", "revoke", pkg, permission).Run(testexec.DumpLogOnError)
	}, nil
}

// InstallApp installs an Android APK.
func InstallApp(ctx context.Context, a *arc.ARC, apkDataPath string, pkg string) (CleanupCallback, error) {
	testing.ContextLogf(ctx, "Installing Android app %q", pkg)
	if err := a.Install(ctx, apkDataPath); err != nil {
		return nil, errors.Wrapf(err, "failed to install apk %q", apkDataPath)
	}

	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Uninstalling Android app %q", pkg)
		return a.Uninstall(ctx, pkg)
	}, nil
}

// StartActivityOptions holds all optional parameters of StartActivity.
type StartActivityOptions struct {
	// Optional: prefixes and suffixes to pkgName/activityName. This is useful for intent arguments.
	// See also: https://developer.android.com/studio/command-line/adb.html#IntentSpec
	Prefixes []string
	Suffixes []string

	// Raises an error if the activity is no longer running at cleanup time, if set to true.
	AssertStillRunningOnTeardown bool
}

// StartActivityOption sets an optional parameter of StartActivity.
type StartActivityOption func(*StartActivityOptions)

// Prefixes sets the optional prefixes parameter of StartActivity.
func Prefixes(prefixes []string) StartActivityOption {
	return func(args *StartActivityOptions) {
		args.Prefixes = prefixes
	}
}

// Suffixes sets the optional suffixes parameter of StartActivity.
func Suffixes(suffixes []string) StartActivityOption {
	return func(args *StartActivityOptions) {
		args.Suffixes = suffixes
	}
}

// AssertStillRunningOnTeardown sets a paramter that determines if an error should be thrown at the end of the test if the activity is no longer running.
func AssertStillRunningOnTeardown(value bool) StartActivityOption {
	return func(args *StartActivityOptions) {
		args.AssertStillRunningOnTeardown = value
	}
}

// StartActivity starts an Android activity.
func StartActivity(ctx context.Context, a *arc.ARC, pkg string, activityName string, setters ...StartActivityOption) (CleanupCallback, error) {
	// Default options.
	args := &StartActivityOptions{
		Prefixes:                     []string{},
		Suffixes:                     []string{},
		AssertStillRunningOnTeardown: true,
	}

	for _, setter := range setters {
		setter(args)
	}

	testing.ContextLogf(ctx, "Starting activity %s/%s", pkg, activityName)
	activity, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create activity %q in package %q", activityName, pkg)
	}
	if err := activity.StartWithArgs(ctx, args.Prefixes, args.Suffixes); err != nil {
		return nil, errors.Wrapf(err, "failed to start activity %q in package %q", activityName, pkg)
	}

	return func(ctx context.Context) error {
		const (
			activityNotRunningMessage = "tried to close activity %q but it was no longer running"
		)

		// Check if the app is still running.
		isRunning, err := activity.IsRunning(ctx)
		if err != nil {
			return err
		}

		testing.ContextLogf(ctx, "Stopping activities in package %s", pkg)

		if !isRunning {
			if args.AssertStillRunningOnTeardown {
				return errors.Errorf(activityNotRunningMessage, activityName)
			}
			return nil
		}

		defer activity.Close()
		return activity.Stop(ctx)
	}, nil
}

// AdbMkdir runs 'adb shell mkdir <path>'.
func AdbMkdir(ctx context.Context, a *arc.ARC, path string) (CleanupCallback, error) {
	testing.ContextLogf(ctx, "mkdir %s", path)
	if err := a.Command(ctx, "mkdir", path).Run(testexec.DumpLogOnError); err != nil {
		return nil, err
	}

	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "rm -rf %s", path)
		return a.Command(ctx, "rm", "-rf", path).Run(testexec.DumpLogOnError)
	}, nil
}
