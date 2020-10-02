// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/adb"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const adbAddr = "127.0.0.1:5555"

// connectADB connects to the remote ADB daemon.
// After this function returns successfully, we can assume that ADB connection is ready.
func connectADB(ctx context.Context) (*adb.Device, error) {
	ctx, st := timing.Start(ctx, "connect_adb")
	defer st.End()

	// ADBD thinks that there is an Android emulator running because it notices adb-proxy listens on localhost:5555.
	if device, err := adb.WaitForDevice(ctx, func(d *adb.Device) bool {
		return strings.HasPrefix(d.Serial, "emulator-")
	}, 10*time.Second); err == nil {
		return device, device.WaitForState(ctx, adb.StateDevice, ctxutil.MaxTimeout)
	}

	// https://developer.android.com/studio/command-line/adb#notlisted shows that on certain conditions emulator may not be listed. For safety, fallback to manually connecting to adb-proxy address.
	testing.ContextLog(ctx, "ARC failed to find emulator. Falling back to ip address")
	device, err := adb.Connect(ctx, adbAddr, ctxutil.MaxTimeout)
	if err != nil {
		return nil, err
	}
	return device, device.WaitForState(ctx, adb.StateDevice, ctxutil.MaxTimeout)
}

// InstallOption defines possible options to pass to "adb install".
type InstallOption string

// ADB install options listed in "adb help".
const (
	InstallOptionLockApp               InstallOption = "-l"
	InstallOptionReplaceApp            InstallOption = "-r"
	InstallOptionAllowTestPackage      InstallOption = "-t"
	InstallOptionSDCard                InstallOption = "-s"
	InstallOptionAllowVersionDowngrade InstallOption = "-d"
	InstallOptionGrantPermissions      InstallOption = "-g"
	InstallOptionEphemeralInstall      InstallOption = "--instant"
)

var showAPKPathWarningOnce sync.Once

// Install installs an APK file to the Android system.
// By default, it uses InstallOptionReplaceApp and InstallOptionAllowVersionDowngrade.
func (a *ARC) Install(ctx context.Context, path string, installOptions ...InstallOption) error {
	if strings.HasPrefix(path, apkPathPrefix) {
		showAPKPathWarningOnce.Do(func() {
			testing.ContextLog(ctx, "WARNING: When files under tast-tests/android are modified, APKs on the DUT should be pushed manually. See tast-tests/android/README.md")
		})
	}

	if err := a.Command(ctx, "settings", "put", "global", "verifier_verify_adb_installs", "0").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed disabling verifier_verify_adb_installs")
	}

	installOptions = append(installOptions, InstallOptionReplaceApp)
	installOptions = append(installOptions, InstallOptionAllowVersionDowngrade)
	commandArgs := []string{"install"}
	for _, installOption := range installOptions {
		commandArgs = append(commandArgs, string(installOption))
	}
	commandArgs = append(commandArgs, path)
	out, err := a.device.Command(ctx, commandArgs...).Output(testexec.DumpLogOnError)
	if err != nil {
		return err
	}

	// "Success" is the only possible positive result. See runInstall() here:
	// https://android.googlesource.com/platform/frameworks/base/+/bdd94d9979e28c39539e25fbb98621df3cbe86f2/services/core/java/com/android/server/pm/PackageManagerShellCommand.java#901
	matched, err := regexp.Match("^Success", out)
	if err != nil {
		return err
	}
	if !matched {
		return errors.Errorf("failed to install %v %q", path, string(out))
	}
	return nil
}

// InstalledPackages returns a set of currently-installed packages, e.g. "android".
// This operation is slow (700+ ms), so unnecessary calls should be avoided.
func (a *ARC) InstalledPackages(ctx context.Context) (map[string]struct{}, error) {
	ctx, st := timing.Start(ctx, "installed_packages")
	defer st.End()

	out, err := a.Command(ctx, "pm", "list", "packages").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "listing packages failed")
	}

	pkgs := make(map[string]struct{})
	for _, pkg := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		// |pm list packages| prepends "package:" to installed packages. Not needed.
		n := strings.TrimPrefix(pkg, "package:")
		pkgs[n] = struct{}{}
	}
	return pkgs, nil
}

// Uninstall a package from the Android system.
func (a *ARC) Uninstall(ctx context.Context, pkg string) error {
	out, err := a.device.Command(ctx, "uninstall", pkg).Output(testexec.DumpLogOnError)
	if err != nil {
		return err
	}

	// "Success" is the only possible positive result. See runUninstall() here:
	// https://android.googlesource.com/platform/frameworks/base/+/bdd94d9979e28c39539e25fbb98621df3cbe86f2/services/core/java/com/android/server/pm/PackageManagerShellCommand.java#1428
	matched, err := regexp.Match("^Success", out)
	if err != nil {
		return err
	}
	if !matched {
		return errors.Errorf("failed to uninstall %v %q", pkg, string(out))
	}
	return nil
}
