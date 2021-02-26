// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbysetup is used to set up the environment for Nearby Share tests.
package nearbysetup

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android"
	"chromiumos/tast/local/android/adb"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysnippet"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// DefaultScreenTimeout is the default screen-off timeout for the Android device.
// It is a sufficiently large value to guarantee most transfers can complete without the screen turning off,
// since Nearby Share on Android requires the screen to be on.
const DefaultScreenTimeout = 10 * time.Minute

// CrOSSetup enables Chrome OS Nearby Share and configures its settings using the nearby_share_settings
// interface which is available through chrome://nearby. This allows tests to bypass onboarding.
// If deviceName is empty, the device display name will not be set and the default will be used.
func CrOSSetup(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, dataUsage DataUsage, visibility Visibility, deviceName string) error {
	nearbyConn, err := cr.NewConn(ctx, "chrome://nearby")
	if err != nil {
		return errors.Wrap(err, "failed to launch chrome://nearby")
	}
	defer nearbyConn.Close()
	defer nearbyConn.CloseTarget(ctx)

	var nearbySettings chrome.JSObject
	if err := nearbyConn.Call(ctx, &nearbySettings, `async function() {
		return await import('./shared/nearby_share_settings.m.js').then(m => m.getNearbyShareSettings());
	}`); err != nil {
		return errors.Wrap(err, "failed to import nearby_share_settings.m.js")
	}

	if err := nearbySettings.Call(ctx, nil, `function() {this.setEnabled(true)}`); err != nil {
		return errors.Wrap(err, "failed to enable Nearby Share from OS settings")
	}

	if err := nearbySettings.Call(ctx, nil, `function(dataUsage) {this.setDataUsage(dataUsage)}`, dataUsage); err != nil {
		return errors.Wrapf(err, "failed to call setDataUsage with value %v", dataUsage)
	}

	if err := nearbySettings.Call(ctx, nil, `function(visibility) {this.setVisibility(visibility)}`, visibility); err != nil {
		return errors.Wrapf(err, "failed to call setVisibility with value %v", visibility)
	}

	if deviceName != "" {
		var res DeviceNameValidationResult
		if err := nearbySettings.Call(ctx, &res, `async function(name) {
			r = await this.setDeviceName(name);
			return r.result;
		}`, deviceName); err != nil {
			return errors.Wrapf(err, "failed to call setDeviceName with name %v", deviceName)
		}
		const baseError = "failed to set device name; validation result %v(%v)"
		switch res {
		case DeviceNameValidationResultValid:
		case DeviceNameValidationResultErrorEmpty:
			return errors.Errorf(baseError, res, "empty")
		case DeviceNameValidationResultErrorTooLong:
			return errors.Errorf(baseError, res, "too long")
		case DeviceNameValidationResultErrorNotValidUtf8:
			return errors.Errorf(baseError, res, "not valid UTF-8")
		default:
			return errors.Errorf(baseError, res, "unexpected value")
		}
	}

	// Enable verbose bluetooth logging.
	levels := bluetooth.LogVerbosity{
		Dispatcher: true,
		Newblue:    true,
		Bluez:      true,
		Kernel:     true,
	}
	if err := bluetooth.SetDebugLogLevels(ctx, levels); err != nil {
		return errors.Wrap(err, "failed to enable verbose bluetooth logging")
	}

	return nil
}

// AndroidSetup prepares the connected Android device for Nearby Share tests.
func AndroidSetup(ctx context.Context, accountUtilZipPath, username, password string, loggedIn bool, apkZipPath string, rooted bool, screenOff time.Duration, dataUsage nearbysnippet.DataUsage, visibility nearbysnippet.Visibility, name string) (*nearbysnippet.AndroidNearbyDevice, error) {
	// This loads the ARC adb vendor key, which must be pre-loaded on the Android device to allow adb over usb without requiring UI interaction.
	if err := adb.LaunchServer(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to launch adb server")
	}

	// Wait for the first available device, since we are assuming only a single device is connected.
	testDevice, err := adb.WaitForDevice(ctx, func(device *adb.Device) bool { return !strings.HasPrefix(device.Serial, "emulator-") }, 10*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list adb devices")
	}

	// Clear logcat logs and enable verbose logging for Nearby-related tags.
	if err := ConfigureNearbyLogging(ctx, testDevice); err != nil {
		return nil, errors.Wrap(err, "failed to configure Android Nearby logs")
	}

	// Clear the Android's default directory for receiving shares.
	if err := testDevice.RemoveAll(ctx, android.DownloadDir); err != nil {
		return nil, errors.Wrap(err, "failed to clear Android downloads directory")
	}

	// Prepare the device for Nearby Sharing by waking+unlocking the screen, enabling bluetooth, and extending the screen-off timeout.
	if err := testDevice.PressKeyCode(ctx, strconv.Itoa(int(ui.KEYCODE_WAKEUP))); err != nil {
		return nil, errors.Wrap(err, "failed to wake screen")
	}
	if err := testDevice.PressKeyCode(ctx, strconv.Itoa(int(ui.KEYCODE_MENU))); err != nil {
		return nil, errors.Wrap(err, "failed to wake screen")
	}

	if rooted {
		if err := testDevice.EnableBluetooth(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to enable bluetooth")
		}
		if err := testDevice.SetScreenOffTimeout(ctx, screenOff); err != nil {
			return nil, errors.Wrap(err, "failed to extend screen-off timeout")
		}
	}

	// Remove and re-add the specified account. A GAIA login is required to configure Nearby Share on the Android device.
	if !loggedIn {
		// Unzip the APK to a temp dir.
		tempDir, err := ioutil.TempDir("", "account-util-apk")
		if err != nil {
			return nil, errors.Wrap(err, "failed to create temp dir")
		}
		defer os.RemoveAll(tempDir)
		if err := testexec.CommandContext(ctx, "unzip", accountUtilZipPath, nearbysnippet.AccountUtilApk, "-d", tempDir).Run(testexec.DumpLogOnError); err != nil {
			return nil, errors.Wrapf(err, "failed to unzip %v from %v", nearbysnippet.AccountUtilApk, accountUtilZipPath)
		}

		// Install the GoogleAccountUtil APK.
		if err := testDevice.Install(ctx, filepath.Join(tempDir, nearbysnippet.AccountUtilApk), adb.InstallOptionGrantPermissions); err != nil {
			return nil, errors.Wrap(err, "failed to install GoogleAccountUtil APK on the device")
		}

		// Try to remove the user account before re-adding it.
		testing.ContextLog(ctx, "Removing all GAIA users from the Android device")
		removeAccountsCmd := testDevice.ShellCommand(ctx, "am", "instrument", "-w", "com.google.android.tradefed.account/.RemoveAccounts")
		if out, err := removeAccountsCmd.Output(); err != nil {
			return nil, errors.Wrap(err, "failed to run remove accounts command")
		} else if !strings.Contains(string(out), "INSTRUMENTATION_RESULT: result=SUCCESS") {
			return nil, errors.Errorf("failed to remove accounts from the device (%v)", string(out))
		}

		testing.ContextLog(ctx, "Adding Nearby GAIA user to the Android device")
		addAccountCmd := testDevice.ShellCommand(ctx, "am", "instrument", "-w",
			"-e", "account", username, "-e", "password", password, "-e", "sync", "true", "com.google.android.tradefed.account/.AddAccount",
		)
		if out, err := addAccountCmd.Output(); err != nil {
			return nil, errors.Wrap(err, "failed to add account from the device")
		} else if !strings.Contains(string(out), "INSTRUMENTATION_RESULT: result=SUCCESS") {
			return nil, errors.Errorf("failed to add account to the device (%v)", string(out))
		}
	}

	// Launch and start the snippet server. Don't override GMS Core flags if specified in the runtime vars.
	androidNearby, err := nearbysnippet.New(ctx, testDevice, apkZipPath, rooted)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up the snippet server")
	}

	if err := androidNearby.Initialize(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to initialize snippet server")
	}

	if err := androidNearby.SetupDevice(ctx, dataUsage, visibility, name); err != nil {
		return nil, errors.Wrap(err, "failed to configure Android Nearby Share settings")
	}

	// androidNearby.SetupDevice is asynchronous, so we need to poll until the settings changes have taken effect.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if n, err := androidNearby.GetDeviceName(ctx); err != nil {
			return testing.PollBreak(err)
		} else if n != name {
			return errors.Errorf("current device name (%v) not yet updated to %v", n, name)
		}

		if v, err := androidNearby.GetVisibility(ctx); err != nil {
			return testing.PollBreak(err)
		} else if v != visibility {
			return errors.Errorf("current visibility (%v) not yet updated to %v", v, visibility)
		}

		if d, err := androidNearby.GetDataUsage(ctx); err != nil {
			return testing.PollBreak(err)
		} else if d != dataUsage {
			return errors.Errorf("current data usage (%v) not yet updated to %v", d, dataUsage)
		}

		return nil
	}, &testing.PollOptions{Interval: 2 * time.Second, Timeout: 10 * time.Second}); err != nil {
		return nil, errors.Wrap(err, "timed out waiting for Nearby Share settings to update")
	}

	return androidNearby, nil
}

// ConfigureNearbyLogging clears existing logcat logs and enables verbose logging for Nearby modules.
func ConfigureNearbyLogging(ctx context.Context, d *adb.Device) error {
	if err := d.ClearLogcat(ctx); err != nil {
		return errors.Wrap(err, "failed to clear previous logcat logs")
	}
	tags := []string{
		"Nearby",
		"NearbyMessages",
		"NearbyDiscovery",
		"NearbyConnections",
		"NearbyMediums",
		"NearbySetup",
		"NearbySharing",
	}
	for _, tag := range tags {
		if err := d.EnableVerboseLoggingForTag(ctx, tag); err != nil {
			return errors.Wrapf(err, "failed to enable verbose logging for tag %v", tag)
		}
	}
	return nil
}
