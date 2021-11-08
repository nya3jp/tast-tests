// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbysetup is used to set up the environment for Nearby Share tests.
package nearbysetup

import (
	"context"
	"time"

	"chromiumos/tast/common/android"
	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/cros/nearbyshare/nearbysnippet"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/testing"
)

// DefaultScreenTimeout is the default screen-off timeout for the Android device.
// It is a sufficiently large value to guarantee most transfers can complete without the screen turning off,
// since Nearby Share on Android requires the screen to be on.
const DefaultScreenTimeout = 10 * time.Minute

// AndroidSetup prepares the connected Android device for Nearby Share tests.
func AndroidSetup(ctx context.Context, testDevice *adb.Device, accountUtilZipPath, username, password string, loggedIn bool, apkZipPath string, rooted bool, screenOff time.Duration, dataUsage nearbysnippet.DataUsage, visibility nearbysnippet.Visibility, name string) (*nearbysnippet.AndroidNearbyDevice, error) {
	// Clear the Android's default directory for receiving shares.
	if err := testDevice.RemoveContents(ctx, android.DownloadDir); err != nil {
		return nil, errors.Wrap(err, "failed to clear Android downloads directory")
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

// CrosAttributes contains information about the CrOS device that are relevant to Nearby Share.
type CrosAttributes struct {
	BasicAttributes *crossdevice.CrosAttributes
	DisplayName     string
	DataUsage       string
	Visibility      string
}
