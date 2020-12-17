// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbysetup is used to set up the environment for Nearby Share tests.
package nearbysetup

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/ui/ossettings"
)

// CrOSSetup enables Chrome OS Nearby Share and configures its settings through OS Settings. This allows tests to bypass onboarding.
// If deviceName is empty, the device display name will not be set and the default will be used.
func CrOSSetup(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, dataUsage nearbyshare.CrOSDataUsage, visibility nearbyshare.CrOSVisibility, deviceName string) error {
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
		var res nearbyshare.CrOSDeviceNameValidationResult
		if err := settingsConn.Call(ctx, &res, `async function(name) {
			r = await nearby_share.getNearbyShareSettings().setDeviceName(name);
			return r.result;
		}`, deviceName); err != nil {
			return errors.Wrapf(err, "failed to call setDeviceName with name %v", deviceName)
		}
		baseError := "failed to set device name; validation result %v(%v)"
		switch res {
		case nearbyshare.CrOSDeviceNameValidationResultValid:
		case nearbyshare.CrOSDeviceNameValidationResultErrorEmpty:
			return errors.Errorf(baseError, res, "empty")
		case nearbyshare.CrOSDeviceNameValidationResultErrorTooLong:
			return errors.Errorf(baseError, res, "too long")
		case nearbyshare.CrOSDeviceNameValidationResultErrorNotValidUtf8:
			return errors.Errorf(baseError, res, "not valid UTF-8")
		default:
			return errors.Errorf(baseError, res, "unexpected value")
		}
	}
	return nil
}
