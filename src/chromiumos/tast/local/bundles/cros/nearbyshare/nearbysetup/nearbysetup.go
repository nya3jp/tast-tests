// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbysetup is used to set up the environment for Nearby Share tests.
package nearbysetup

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android"
	"chromiumos/tast/local/android/adb"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/bundles/cros/nearbyshare/nearbysnippet"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/ui/ossettings"
	"chromiumos/tast/testing"
)

// DefaultScreenTimeout is the default screen-off timeout for the Android device.
// It is a sufficiently large value to guarantee most transfers can complete without the screen turning off,
// since Nearby Share on Android requires the screen to be on.
const DefaultScreenTimeout = 10 * time.Minute

// CrOSSetup enables Chrome OS Nearby Share and configures its settings through OS Settings. This allows tests to bypass onboarding.
// If deviceName is empty, the device display name will not be set and the default will be used.
func CrOSSetup(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, dataUsage nearbyshare.DataUsage, visibility nearbyshare.Visibility, deviceName string) error {
	if err := ossettings.Launch(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to launch OS Settings")
	}
	settingsConn, err := ossettings.ChromeConn(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to establish connection to OS Settings Chrome session")
	}
	defer settingsConn.Close()
	defer ossettings.Close(ctx, tconn)

	if err := settingsConn.WaitForExpr(ctx, `nearby_share !== undefined`); err != nil {
		return errors.Wrap(err, "failed waiting for nearby_share to load")
	}

	if err := settingsConn.Call(ctx, nil, `function() {nearby_share.getNearbyShareSettings().setEnabled(true)}`); err != nil {
		return errors.Wrap(err, "failed to enable Nearby Share from OS settings")
	}

	if err := settingsConn.Call(ctx, nil, `function(dataUsage) {nearby_share.getNearbyShareSettings().setDataUsage(dataUsage)}`, dataUsage); err != nil {
		return errors.Wrapf(err, "failed to call setDataUsage with value %v", dataUsage)
	}

	if err := settingsConn.Call(ctx, nil, `function(visibility) {nearby_share.getNearbyShareSettings().setVisibility(visibility)}`, visibility); err != nil {
		return errors.Wrapf(err, "failed to call setVisibility with value %v", visibility)
	}

	if deviceName != "" {
		var res nearbyshare.DeviceNameValidationResult
		if err := settingsConn.Call(ctx, &res, `async function(name) {
			r = await nearby_share.getNearbyShareSettings().setDeviceName(name);
			return r.result;
		}`, deviceName); err != nil {
			return errors.Wrapf(err, "failed to call setDeviceName with name %v", deviceName)
		}
		const baseError = "failed to set device name; validation result %v(%v)"
		switch res {
		case nearbyshare.DeviceNameValidationResultValid:
		case nearbyshare.DeviceNameValidationResultErrorEmpty:
			return errors.Errorf(baseError, res, "empty")
		case nearbyshare.DeviceNameValidationResultErrorTooLong:
			return errors.Errorf(baseError, res, "too long")
		case nearbyshare.DeviceNameValidationResultErrorNotValidUtf8:
			return errors.Errorf(baseError, res, "not valid UTF-8")
		default:
			return errors.Errorf(baseError, res, "unexpected value")
		}
	}
	return nil
}

// AndroidSetup prepares the connected Android device for Nearby Share tests.
func AndroidSetup(ctx context.Context, apkZipPath string, rooted bool, screenOff time.Duration, dataUsage nearbysnippet.DataUsage, visibility nearbysnippet.Visibility, name string) (*nearbysnippet.AndroidNearbyDevice, error) {
	// This loads the ARC adb vendor key, which must be pre-loaded on the Android device to allow adb over usb without requiring UI interaction.
	if err := adb.LaunchServer(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to launch adb server")
	}

	// Wait for the first available device, since we are assuming only a single device is connected.
	testDevice, err := adb.WaitForDevice(ctx, func(device *adb.Device) bool { return !strings.HasPrefix(device.Serial, "emulator-") }, 10*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list adb devices")
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
