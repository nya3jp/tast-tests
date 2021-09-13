// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crossdevice

import (
	"archive/zip"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/errors"
	localadb "chromiumos/tast/local/android/adb"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysnippet"
	"chromiumos/tast/testing"
)

// AdbSetup configures adb and connects to the Android device with adb root if available.
func AdbSetup(ctx context.Context) (*adb.Device, bool, error) {
	// Load the ARC adb vendor key, which must be pre-loaded on the Android device to allow adb over usb without requiring UI interaction.
	if err := localadb.LaunchServer(ctx); err != nil {
		return nil, false, errors.Wrap(err, "failed to launch adb server")
	}
	// Wait for the first available device, since we are assuming only a single Android device is connected to each CrOS device.
	adbDevice, err := adb.WaitForDevice(ctx, func(device *adb.Device) bool { return !strings.HasPrefix(device.Serial, "emulator-") }, 10*time.Second)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to list adb devices")
	}
	// Check if adb root is available.
	rooted := true
	if err := adbDevice.Root(ctx); err != nil {
		testing.ContextLog(ctx, "ADB root access not available; operations requiring root access will be skipped")
		rooted = false
	}
	return adbDevice, rooted, nil
}

// GAIALogin removes existing user accounts from the device and adds the specified one using the tradefed GoogleAccountUtil APK.
// Note that only rooted Android devices can add/remove accounts in this way.
func GAIALogin(ctx context.Context, d *adb.Device, accountUtilZipPath, username, password string) error {
	// Unzip the APK to a temp dir.
	tempDir, err := ioutil.TempDir("", "account-util-apk")
	if err != nil {
		return errors.Wrap(err, "failed to create temp dir")
	}
	defer os.RemoveAll(tempDir)

	r, err := zip.OpenReader(accountUtilZipPath)
	if err != nil {
		return errors.Wrapf(err, "failed to unzip %v", accountUtilZipPath)
	}
	defer r.Close()

	var apkExists bool
	for _, f := range r.File {
		if f.Name == nearbysnippet.AccountUtilApk {
			src, err := f.Open()
			if err != nil {
				return errors.Wrap(err, "failed to open zip contents")
			}
			dstPath := filepath.Join(tempDir, f.Name)
			dst, err := os.Create(dstPath)
			if err != nil {
				return errors.Wrap(err, "failed to create file for copying APK")
			}
			defer dst.Close()

			if _, err := io.Copy(dst, src); err != nil {
				return errors.Wrap(err, "failed to extract apk from zip")
			}
			apkExists = true
			break
		}
	}
	if !apkExists {
		return errors.Errorf("failed to find %v in %v", nearbysnippet.AccountUtilApk, accountUtilZipPath)
	}

	// Install the GoogleAccountUtil APK.
	if err := d.Install(ctx, filepath.Join(tempDir, nearbysnippet.AccountUtilApk), adb.InstallOptionGrantPermissions); err != nil {
		return errors.Wrap(err, "failed to install GoogleAccountUtil APK on the device")
	}

	// Try to remove the user account before re-adding it.
	testing.ContextLog(ctx, "Removing all GAIA users from the Android device")
	removeAccountsCmd := d.ShellCommand(ctx, "am", "instrument", "-w", "com.google.android.tradefed.account/.RemoveAccounts")
	if out, err := removeAccountsCmd.Output(); err != nil {
		return errors.Wrap(err, "failed to run remove accounts command")
	} else if !strings.Contains(string(out), "INSTRUMENTATION_RESULT: result=SUCCESS") {
		return errors.Errorf("failed to remove accounts from the device (%v)", string(out))
	}

	// TODO(crbug/1185918): Re-adding the account immediately after removing it is flaky.
	// Waiting a few seconds fixes it, but we should find a deterministic way to tell when we can safely re-add the account.
	testing.Sleep(ctx, 3*time.Second)
	testing.ContextLog(ctx, "Adding Nearby GAIA user to the Android device")
	addAccountCmd := d.ShellCommand(ctx, "am", "instrument", "-w",
		"-e", "account", username, "-e", "password", password, "com.google.android.tradefed.account/.AddAccount",
	)
	if out, err := addAccountCmd.Output(); err != nil {
		return errors.Wrap(err, "failed to add account from the device")
	} else if !strings.Contains(string(out), "INSTRUMENTATION_RESULT: result=SUCCESS") {
		return errors.Errorf("failed to add account to the device (%v)", string(out))
	}

	return nil
}

// EnableVerboseLogging enables verbose logging on the Android device for the specified tags, and bluetooth+wifi if the device is rooted.
func EnableVerboseLogging(ctx context.Context, d *adb.Device, rooted bool, tags ...string) error {
	for _, tag := range tags {
		if err := d.EnableVerboseLoggingForTag(ctx, tag); err != nil {
			return errors.Wrapf(err, "failed to enable verbose logging for tag %v", tag)
		}
	}
	if rooted {
		if err := d.EnableBluetoothHciLogging(ctx); err != nil {
			return errors.Wrap(err, "failed to enable bluetooth hci logging")
		}
		if err := d.EnableVerboseWifiLogging(ctx); err != nil {
			return errors.Wrap(err, "failed to enable verbose wifi logging")
		}
	}
	return nil
}

// ConfigureDevice performs basic device preparation. This includes clearing logcat and waking the screen,
// and if the device is rooted, enabling bluetooth and extending the screen-off timeout.
func ConfigureDevice(ctx context.Context, d *adb.Device, rooted bool) error {
	// Clear logcat so that logs start from this point on.
	if err := d.ClearLogcat(ctx); err != nil {
		return errors.Wrap(err, "failed to clear previous logcat logs")
	}

	// Prepare the device for Nearby Sharing by waking+unlocking the screen, enabling bluetooth, and extending the screen-off timeout.
	if err := d.PressKeyCode(ctx, strconv.Itoa(int(ui.KEYCODE_WAKEUP))); err != nil {
		return errors.Wrap(err, "failed to wake screen")
	}
	if err := d.PressKeyCode(ctx, strconv.Itoa(int(ui.KEYCODE_MENU))); err != nil {
		return errors.Wrap(err, "failed to wake screen")
	}

	if rooted {
		if err := d.EnableBluetooth(ctx); err != nil {
			return errors.Wrap(err, "failed to enable bluetooth")
		}
		if err := d.SetScreenOffTimeout(ctx, 10*time.Minute); err != nil {
			return errors.Wrap(err, "failed to extend screen-off timeout")
		}
	}

	return nil
}
