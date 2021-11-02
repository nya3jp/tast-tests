// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbysetup is used to set up the environment for Nearby Share tests.
package nearbysetup

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

	"chromiumos/tast/common/android"
	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/cros/nearbyshare/nearbysnippet"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// DefaultScreenTimeout is the default screen-off timeout for the Android device.
// It is a sufficiently large value to guarantee most transfers can complete without the screen turning off,
// since Nearby Share on Android requires the screen to be on.
const DefaultScreenTimeout = 10 * time.Minute

// AndroidSetup prepares the connected Android device for Nearby Share tests.
func AndroidSetup(ctx context.Context, testDevice *adb.Device, accountUtilZipPath, username, password string, loggedIn bool, apkZipPath string, rooted bool, screenOff time.Duration, dataUsage nearbysnippet.DataUsage, visibility nearbysnippet.Visibility, name string) (*nearbysnippet.AndroidNearbyDevice, error) {
	// If the PIN was left on from a previous test we need to remove it.
	// However depending on what state the Phone is in when you remove the PIN,
	// you might still be shown the lock screen PIN UI and the test will be blocked.
	// To guarantee the PIN is removed and reflected in the UI, we need to turn the screen on,
	// disable the PIN, and then turn off the screen so the next time it is turned on it will be disabled.
	if err := testDevice.PressKeyCode(ctx, strconv.Itoa(int(ui.KEYCODE_WAKEUP))); err != nil {
		return nil, errors.Wrap(err, "failed to wake screen")
	}
	// Remove any PIN on the phone that may be left from other tests.
	if err := testDevice.ClearPIN(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to clear PIN")
	}
	if err := testDevice.DisableLockscreen(ctx, true); err != nil {
		return nil, errors.Wrap(err, "failed to clear PIN")
	}
	if err := testDevice.WaitForLockscreenDisabled(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to wait for lockscreen to be cleared")
	}
	if err := testDevice.PressKeyCode(ctx, strconv.Itoa(int(ui.KEYCODE_POWER))); err != nil {
		return nil, errors.Wrap(err, "failed to turn off screen")
	}

	// Clear logcat so that logs start from this point on.
	if err := testDevice.ClearLogcat(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to clear previous logcat logs")
	}

	// Clear the Android's default directory for receiving shares.
	if err := testDevice.RemoveContents(ctx, android.DownloadDir); err != nil {
		return nil, errors.Wrap(err, "failed to clear Android downloads directory")
	}

	// Prepare the device for Nearby Sharing by waking+unlocking the screen, enabling bluetooth, and extending the screen-off timeout.
	if err := testDevice.PressKeyCode(ctx, strconv.Itoa(int(ui.KEYCODE_WAKEUP))); err != nil {
		return nil, errors.Wrap(err, "failed to wake screen")
	}
	if err := testDevice.PressKeyCode(ctx, strconv.Itoa(int(ui.KEYCODE_MENU))); err != nil {
		return nil, errors.Wrap(err, "failed to wake screen")
	}

	// Enable verbose logging for Nearby Share.
	if err := ConfigureNearbyLogging(ctx, testDevice, rooted); err != nil {
		return nil, errors.Wrap(err, "failed to configure Android Nearby logs")
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
	// Root access is required for adding and removing accounts.
	if !loggedIn && rooted {
		// Unzip the APK to a temp dir.
		tempDir, err := ioutil.TempDir("", "account-util-apk")
		if err != nil {
			return nil, errors.Wrap(err, "failed to create temp dir")
		}
		defer os.RemoveAll(tempDir)

		r, err := zip.OpenReader(accountUtilZipPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unzip %v", accountUtilZipPath)
		}
		defer r.Close()

		var apkExists bool
		for _, f := range r.File {
			if f.Name == nearbysnippet.AccountUtilApk {
				src, err := f.Open()
				if err != nil {
					return nil, errors.Wrap(err, "failed to open zip contents")
				}
				dstPath := filepath.Join(tempDir, f.Name)
				dst, err := os.Create(dstPath)
				if err != nil {
					return nil, errors.Wrap(err, "failed to create file for copying APK")
				}
				defer dst.Close()

				if _, err := io.Copy(dst, src); err != nil {
					return nil, errors.Wrap(err, "failed to extract apk from zip")
				}
				apkExists = true
				break
			}
		}
		if !apkExists {
			return nil, errors.Errorf("failed to find %v in %v", nearbysnippet.AccountUtilApk, accountUtilZipPath)
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

		// TODO(crbug/1185918): Re-adding the account immediately after removing it is flaky.
		// Waiting a few seconds fixes it, but we should find a deterministic way to tell when we can safely re-add the account.
		testing.Sleep(ctx, 3*time.Second)
		testing.ContextLog(ctx, "Adding Nearby GAIA user to the Android device")
		addAccountCmd := testDevice.ShellCommand(ctx, "am", "instrument", "-w",
			"-e", "account", username, "-e", "password", password, "com.google.android.tradefed.account/.AddAccount",
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

	if err := AndroidConfigure(ctx, androidNearby, dataUsage, visibility, name); err != nil {
		return nil, errors.Wrap(err, "failed to configure Android Nearby Share settings")
	}

	return androidNearby, nil
}

// AndroidConfigure configures Nearby Share settings on an Android device.
func AndroidConfigure(ctx context.Context, androidNearby *nearbysnippet.AndroidNearbyDevice, dataUsage nearbysnippet.DataUsage, visibility nearbysnippet.Visibility, name string) error {
	if err := androidNearby.SetupDevice(ctx, dataUsage, visibility, name); err != nil {
		return errors.Wrap(err, "failed to configure Android Nearby Share settings")
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
		return errors.Wrap(err, "timed out waiting for Nearby Share settings to update")
	}

	// Force-sync after changing Nearby settings to ensure the phone's certificates are regenerated and uploaded.
	if err := androidNearby.Sync(ctx); err != nil {
		return errors.Wrap(err, "failed to sync contacts and certificates")
	}

	return nil
}

// ConfigureNearbyLogging enables verbose logging for Nearby modules, bluetooth, and wifi on Android.
func ConfigureNearbyLogging(ctx context.Context, d *adb.Device, rooted bool) error {
	tags := []string{
		"Nearby",
		"NearbyMessages",
		"NearbyDiscovery",
		"NearbyConnections",
		"NearbyMediums",
		"NearbySetup",
		"NearbySharing",
		"NearbyDirect",
		"Backup",
		"SmartDevice",
		"audioModem",
	}
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

// CrosAttributes contains information about the CrOS device that are relevant to Nearby Share.
// "Cros" is redundantly prepended to the field names to make them easy to distinguish from Android attributes in test logs.
type CrosAttributes struct {
	DisplayName     string
	User            string
	DataUsage       string
	Visibility      string
	ChromeVersion   string
	ChromeOSVersion string
	Board           string
	Model           string
}
