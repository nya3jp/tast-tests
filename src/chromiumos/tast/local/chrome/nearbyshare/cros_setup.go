// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"io"
	"os"
	"time"

	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

// CrOSSetup enables ChromeOS Nearby Share and configures its settings using the nearby_share_settings
// interface which is available through chrome://nearby. This allows tests to bypass onboarding.
// If deviceName is empty, the device display name will not be set and the default will be used.
func CrOSSetup(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, dataUsage nearbycommon.DataUsage, visibility nearbycommon.Visibility, deviceName string) error {
	nearbyConn, err := cr.NewConn(ctx, "chrome://nearby")
	if err != nil {
		return errors.Wrap(err, "failed to launch chrome://nearby")
	}
	defer nearbyConn.Close()
	defer nearbyConn.CloseTarget(ctx)

	var nearbySettings chrome.JSObject
	if err := nearbyConn.Call(ctx, &nearbySettings, `async function() {
		return await import('./shared/nearby_share_settings.js').then(m => m.getNearbyShareSettings());
	}`); err != nil {
		return errors.Wrap(err, "failed to import nearby_share_settings.js")
	}

	if err := nearbySettings.Call(ctx, nil, `function() {this.setIsOnboardingComplete(true)}`); err != nil {
		return errors.Wrap(err, "failed to set onboarding complete Nearby Share from OS settings")
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
		var res nearbycommon.DeviceNameValidationResult
		if err := nearbySettings.Call(ctx, &res, `async function(name) {
			r = await this.setDeviceName(name);
			return r.result;
		}`, deviceName); err != nil {
			return errors.Wrapf(err, "failed to call setDeviceName with name %v", deviceName)
		}
		const baseError = "failed to set device name; validation result %v(%v)"
		switch res {
		case nearbycommon.DeviceNameValidationResultValid:
		case nearbycommon.DeviceNameValidationResultErrorEmpty:
			return errors.Errorf(baseError, res, "empty")
		case nearbycommon.DeviceNameValidationResultErrorTooLong:
			return errors.Errorf(baseError, res, "too long")
		case nearbycommon.DeviceNameValidationResultErrorNotValidUtf8:
			return errors.Errorf(baseError, res, "not valid UTF-8")
		default:
			return errors.Errorf(baseError, res, "unexpected value")
		}
	}

	// Enable verbose bluetooth logging.
	levels := bluetooth.LogVerbosity{
		Bluez:  true,
		Kernel: true,
	}
	if err := bluetooth.SetDebugLogLevels(ctx, levels); err != nil {
		return errors.Wrap(err, "failed to enable verbose bluetooth logging")
	}

	return nil
}

// GetCrosAttributes gets the Chrome version and combines it into a CrosAttributes strct with the provided values for easy logging with json.MarshalIndent.
func GetCrosAttributes(ctx context.Context, tconn *chrome.TestConn, displayName, username string, dataUsage nearbycommon.DataUsage, visibility nearbycommon.Visibility) (*nearbycommon.CrosAttributes, error) {
	// Get the base set of CrOS attributes used in all crossdevice tests.
	basicAttributes, err := crossdevice.GetCrosAttributes(ctx, tconn, username)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get base set of crossdevice CrOS attributes for reporting")
	}

	// Add nearby specific attributes.
	nearbyAttrs := nearbycommon.CrosAttributes{
		BasicAttributes: basicAttributes,
		DisplayName:     displayName,
	}

	if val, ok := nearbycommon.DataUsageStrings[dataUsage]; ok {
		nearbyAttrs.DataUsage = val
	} else {
		return nil, errors.Errorf("undefined dataUsage: %v", dataUsage)
	}
	if val, ok := nearbycommon.VisibilityStrings[visibility]; ok {
		nearbyAttrs.Visibility = val
	} else {
		return nil, errors.Errorf("undefined visibility: %v", visibility)
	}

	return &nearbyAttrs, nil
}

// StartLogging starts collecting logs from the specified log file, such as /var/log/chrome/chrome or /var/log/messages.
// Only log lines that appear after StartLogging is called will be collected, so logs for
// individual tests can be extracted if tests are running consecutively on a shared fixture or precondition.
// Callers should defer calling Save with the returned *syslog.LineReader to save the logs and free associated resources.
func StartLogging(ctx context.Context, path string) (*syslog.LineReader, error) {
	// Poll for a couple of secs only so that service code calling into this doesn't hang.
	reader, err := syslog.NewLineReader(ctx, path, false, &testing.PollOptions{Timeout: 2 * time.Second})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create LineReader")
	}
	return reader, nil
}

// SaveLogs saves the logs that have appeared since StartLogging was called, and then closes the individual line readers.
func SaveLogs(ctx context.Context, reader *syslog.LineReader, path string) error {
	// Ensure the LineReader is closed.
	defer reader.Close()

	log, err := os.Create(path)
	if err != nil {
		return errors.Wrapf(err, "failed to create %v", path)
	}
	defer log.Close()
	for {
		line, err := reader.ReadLine()
		if err == io.EOF {
			break
		} else if err != nil {
			return errors.Wrap(err, "failed to read log")
		}
		log.WriteString(line)
	}

	return nil
}
