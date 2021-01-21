// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/android/adb"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// adbPort is the Hard-coded port of ADB in ARC.
const adbPort = 5555

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

	// https://developer.android.com/studio/command-line/adb#notlisted shows that on certain conditions emulator may not be listed.
	// For safety, fallback to manually connecting to adb-proxy address.
	testing.ContextLog(ctx, "ARC failed to find emulator. Falling back to ip address")
	device, err := adb.Connect(ctx, fmt.Sprintf("localhost:%d", adbPort), ctxutil.MaxTimeout)
	if err != nil {
		return nil, err
	}
	return device, device.WaitForState(ctx, adb.StateDevice, ctxutil.MaxTimeout)
}

// Install installs an APK file to the Android system.
// By default, it uses InstallOptionReplaceApp and InstallOptionAllowVersionDowngrade.
func (a *ARC) Install(ctx context.Context, path string, installOptions ...adb.InstallOption) error {
	return a.device.Install(ctx, path, installOptions...)
}

// InstallMultiple installs a split APK to the Android system.
// By default, it uses InstallOptionReplaceApp and InstallOptionAllowVersionDowngrade.
func (a *ARC) InstallMultiple(ctx context.Context, apks []string, installOptions ...adb.InstallOption) error {
	return a.device.InstallMultiple(ctx, apks, installOptions...)
}

// InstalledPackages returns a set of currently-installed packages, e.g. "android".
// This operation is slow (700+ ms), so unnecessary calls should be avoided.
func (a *ARC) InstalledPackages(ctx context.Context) (map[string]struct{}, error) {
	return a.device.InstalledPackages(ctx)
}

// Uninstall uninstalls a package from the Android system.
func (a *ARC) Uninstall(ctx context.Context, pkg string) error {
	return a.device.Uninstall(ctx, pkg)
}

// IsConnected checks if ARC is connected through ADB.
func (a *ARC) IsConnected(ctx context.Context) error {
	return a.device.IsConnected(ctx)
}
