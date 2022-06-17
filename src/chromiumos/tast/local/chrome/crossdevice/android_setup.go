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
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	localadb "chromiumos/tast/local/android/adb"
	"chromiumos/tast/testing"
)

// PhoneIP is the address of the adb-over-wifi device to use in Cross device tests.
var PhoneIP = testing.RegisterVarString(
	"crossdevice.PhoneIP",
	"",
	"IP address of an Android Phone that has enabled adb-over-tcp",
)

// AdbSetup configures adb and connects to the Android device with adb root if available.
func AdbSetup(ctx context.Context) (*adb.Device, bool, error) {
	// TODO(b/207520262): Remove when we have Android support in skylab for configuring phone.
	setupadb := false
	var err error
	for i := 0; i < 3; i++ {
		// Load the ARC adb vendor key, which must be pre-loaded on the Android device to allow adb over usb without requiring UI interaction.
		if err = localadb.LaunchServer(ctx); err == nil {
			setupadb = true
			break
		} else {
			testing.ContextLog(ctx, "Failed to launch adb server")
			testing.Sleep(ctx, 3*time.Second)
		}
	}
	if !setupadb {
		return nil, false, errors.Wrap(err, "failed to launch adb server after multiple attempts")
	}
	var adbDevice *adb.Device

	if PhoneIP.Value() != "" {
		if err := ConnectToWifi(ctx); err != nil {
			return nil, false, errors.Wrap(err, "failed to connect CrOS device to Wifi")
		}
		adbDevice, err = AdbOverWifi(ctx)
		if err != nil {
			return nil, false, errors.Wrap(err, "failed to connect to adb over wifi device")
		}
	} else {
		waitForDevices := false
		for i := 0; i < 3; i++ {
			// Wait for the first available device, since we are assuming only a single Android device is connected to each CrOS device.
			adbDevice, err = adb.WaitForDevice(ctx, func(device *adb.Device) bool { return !strings.HasPrefix(device.Serial, "emulator-") }, 10*time.Second)
			if err == nil {
				waitForDevices = true
				break
			} else {
				testing.ContextLog(ctx, "Failed to list adb devices")
				testing.Sleep(ctx, 3*time.Second)
			}
		}
		if !waitForDevices {
			return nil, false, errors.Wrap(err, "failed to list adb devices after multiple attempts")
		}
	}
	// Check if adb root is available.
	rooted := true
	if err := adbDevice.Root(ctx); err != nil {
		testing.ContextLog(ctx, "ADB root access not available; operations requiring root access will be skipped")
		rooted = false
	}
	return adbDevice, rooted, nil

}

// AdbOverWifi connects to the adb-over-wifi Android device that was previously setup.
func AdbOverWifi(ctx context.Context) (*adb.Device, error) {
	// Connect to the adb-over-tcp Phone that was setup previously (e.g manually or via autotest control file).
	testing.ContextLogf(ctx, "Android phone IP is: %s", PhoneIP.Value())
	adbDevice, err := adb.Connect(ctx, PhoneIP.Value(), 1*time.Minute)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to adb over wifi")
	}
	testing.ContextLog(ctx, "Connected to remote Android device")

	// Wait for the Android device to be ready for use.
	if err := adbDevice.WaitForState(ctx, adb.StateDevice, 30*time.Second); err != nil {
		return nil, errors.Wrap(err, "wait for state failed")
	}
	return adbDevice, nil
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
		if f.Name == AccountUtilApk {
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
		return errors.Errorf("failed to find %v in %v", AccountUtilApk, accountUtilZipPath)
	}

	// Install the GoogleAccountUtil APK.
	if err := d.Install(ctx, filepath.Join(tempDir, AccountUtilApk), adb.InstallOptionGrantPermissions); err != nil {
		return errors.Wrap(err, "failed to install GoogleAccountUtil APK on the device")
	}

	// Try to remove the user account before re-adding it.
	testing.ContextLog(ctx, "Removing all GAIA users from the Android device")
	if err := RemoveAccounts(ctx, d); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Adding Nearby GAIA user to the Android device")
	if err := AddAccount(ctx, d, username, password); err != nil {
		return err
	}

	return nil
}

// AddAccount adds the specified Google account to the Android device. Assumes the GoogleAccountUtil APK has already been installed.
func AddAccount(ctx context.Context, d *adb.Device, username, password string) error {
	addAccountCmd := d.ShellCommand(ctx, "am", "instrument", "-w",
		"-e", "account", username, "-e", "password", password, "com.google.android.tradefed.account/.AddAccount",
	)
	// TODO(b/187795521): Re-adding the account immediately after removing it is flaky so retry until there is a deterministic indicator.
	retries := 3
	for i := 0; i < retries; i++ {
		testing.Sleep(ctx, 3*time.Second)
		accountAdded := true
		if out, err := addAccountCmd.Output(); err != nil {
			testing.ContextLog(ctx, "Failed to add account from the device")
			accountAdded = false
		} else if !strings.Contains(string(out), "INSTRUMENTATION_RESULT: result=SUCCESS") {
			testing.ContextLogf(ctx, "Failed to add account to the device (%v)", string(out))
			accountAdded = false
		}
		if accountAdded {
			return nil
		}
	}
	return errors.New("failed to add GAIA acccount to Android after multiple attempts")
}

// RemoveAccounts removes accounts from the Android device. Assumes the GoogleAccountUtil APK has already been installed.
func RemoveAccounts(ctx context.Context, d *adb.Device) error {
	removeAccountsCmd := d.ShellCommand(ctx, "am", "instrument", "-w", "com.google.android.tradefed.account/.RemoveAccounts")
	if out, err := removeAccountsCmd.Output(); err != nil {
		return errors.Wrap(err, "failed to run remove accounts command")
	} else if !strings.Contains(string(out), "INSTRUMENTATION_RESULT: result=SUCCESS") {
		return errors.Errorf("failed to remove accounts from the device (%v)", string(out))
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
	// If the PIN was left on from a previous test we need to remove it.
	// However depending on what state the Phone is in when you remove the PIN,
	// you might still be shown the lock screen PIN UI and the test will be blocked.
	// To guarantee the PIN is removed and reflected in the UI, we need to turn the screen on,
	// disable the PIN, and then turn off the screen so the next time it is turned on it will be disabled.
	if err := d.PressKeyCode(ctx, strconv.Itoa(int(ui.KEYCODE_WAKEUP))); err != nil {
		return errors.Wrap(err, "failed to wake screen")
	}
	// Remove any PIN on the phone that may be left from other tests.
	if err := d.ClearPIN(ctx); err != nil {
		return errors.Wrap(err, "failed to clear PIN")
	}
	if err := d.DisableLockscreen(ctx, true); err != nil {
		return errors.Wrap(err, "failed to disable lockscreen")
	}
	if err := d.WaitForLockscreenDisabled(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for lockscreen to be cleared")
	}
	if err := d.PressKeyCode(ctx, strconv.Itoa(int(ui.KEYCODE_POWER))); err != nil {
		return errors.Wrap(err, "failed to turn off the screen")
	}
	// Clear logcat so that logs start from this point on.
	clearLogcat := false
	// TODO(b/207520262): Remove when we have Android support in skylab for configuring phone.
	for i := 0; i < 3; i++ {
		if err := d.ClearLogcat(ctx); err == nil {
			clearLogcat = true
			break
		} else {
			testing.ContextLog(ctx, "Failed to clear previous logcat logs")
			testing.Sleep(ctx, 3*time.Second)
		}
	}
	if !clearLogcat {
		return errors.New("failed to clear logcat after multiple attempts")
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
		// Disable popups that can show when the Google Camera app starts up. These popups can distrupt tests that uses the phone's camera.
		if err := d.GrantPermission(ctx, "com.google.android.GoogleCamera", "android.permission.ACCESS_FINE_LOCATION"); err != nil {
			return errors.Wrap(err, "failed to grant location permission to Google Camera")
		}
		if err := d.SetSystemProperty(ctx, "camera.skip_oobe", "true"); err != nil {
			return errors.Wrap(err, "failed to disable OOBE popups for Google Camera")
		}

	}

	// Additionally, set the screen to stay awake while charging. Features such as Nearby Share do not work if the screen is off.
	if err := d.StayOnWhilePluggedIn(ctx); err != nil {
		return errors.Wrap(err, "failed to set the screen to stay on while charging")
	}

	return nil
}

// AndroidAttributes contains information about the Android device and its settings that are relevant to Cross Device features.
type AndroidAttributes struct {
	User                string
	GMSCoreVersion      int
	AndroidVersion      int
	SDKVersion          int
	ProductName         string
	ModelName           string
	DeviceName          string
	BluetoothMACAddress string
}

// GetAndroidAttributes returns the AndroidAttributes for the device.
func GetAndroidAttributes(ctx context.Context, adbDevice *adb.Device) (*AndroidAttributes, error) {
	var metadata AndroidAttributes

	user, err := adbDevice.GoogleAccount(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Android device user account")
	}
	metadata.User = user

	gmsVersion, err := adbDevice.GMSCoreVersion(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get GMS Core version")
	}
	metadata.GMSCoreVersion = gmsVersion

	androidVersion, err := adbDevice.AndroidVersion(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Android version")
	}
	metadata.AndroidVersion = androidVersion

	sdkVersion, err := adbDevice.SDKVersion(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Android SDK version")
	}
	metadata.SDKVersion = sdkVersion

	metadata.ProductName = adbDevice.Product
	metadata.ModelName = adbDevice.Model
	metadata.DeviceName = adbDevice.Device

	mac, err := adbDevice.BluetoothMACAddress(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the Bluetooth MAC address")
	}
	metadata.BluetoothMACAddress = mac

	return &metadata, nil
}

// BugReport pulls a bug report from the Android device.
func BugReport(ctx context.Context, device *adb.Device, outDir string) error {
	testing.ContextLog(ctx, "capturing Android bug report, this may take several minutes")
	if err := device.BugReport(ctx, outDir); err != nil {
		return errors.Wrap(err, "failed to generate bug report on the Android device")
	}

	// Find and rename the bug report to include "android" in the filename.
	// This is to take advantage of the autotest log throttler rule that prevents
	// files with "android" in the name from being trimmed/deleted, since the generated
	// report can be fairly large (tens of MBs).
	matches, err := filepath.Glob(filepath.Join(outDir, "bugreport*"))
	if err != nil {
		return errors.Wrapf(err, "bug report was not successfully pulled to %v", outDir)
	}
	if len(matches) != 1 {
		return errors.Errorf("unexpected number of bug reports; wanted 1, got %v", len(matches))
	}

	p := strings.Split(matches[0], string(os.PathSeparator))
	filename := p[len(p)-1]
	if err := os.Rename(filepath.Join(outDir, filename), filepath.Join(outDir, "android-"+filename)); err != nil {
		return errors.Wrap(err, "failed to prepend 'android' to the bug report filename")
	}

	// Clear the Android default bug report directory to clean up.
	if err := device.RemoveAll(ctx, "/bugreports/*"); err != nil {
		testing.ContextLog(ctx, "Failed to clear Android /bugreport directory")
	}

	return nil
}
