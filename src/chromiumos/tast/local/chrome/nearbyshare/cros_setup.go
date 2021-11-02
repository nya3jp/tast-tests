// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"io"
	"os"
	"regexp"
	"time"

	"chromiumos/tast/common/cros/nearbyshare/nearbysetup"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/systemlogs"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

// CrOSSetup enables Chrome OS Nearby Share and configures its settings using the nearby_share_settings
// interface which is available through chrome://nearby. This allows tests to bypass onboarding.
// If deviceName is empty, the device display name will not be set and the default will be used.
func CrOSSetup(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, dataUsage nearbysetup.DataUsage, visibility nearbysetup.Visibility, deviceName string) error {
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
		var res nearbysetup.DeviceNameValidationResult
		if err := nearbySettings.Call(ctx, &res, `async function(name) {
			r = await this.setDeviceName(name);
			return r.result;
		}`, deviceName); err != nil {
			return errors.Wrapf(err, "failed to call setDeviceName with name %v", deviceName)
		}
		const baseError = "failed to set device name; validation result %v(%v)"
		switch res {
		case nearbysetup.DeviceNameValidationResultValid:
		case nearbysetup.DeviceNameValidationResultErrorEmpty:
			return errors.Errorf(baseError, res, "empty")
		case nearbysetup.DeviceNameValidationResultErrorTooLong:
			return errors.Errorf(baseError, res, "too long")
		case nearbysetup.DeviceNameValidationResultErrorNotValidUtf8:
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
func GetCrosAttributes(ctx context.Context, tconn *chrome.TestConn, displayName, username string, dataUsage nearbysetup.DataUsage, visibility nearbysetup.Visibility) (*nearbysetup.CrosAttributes, error) {
	attrs := nearbysetup.CrosAttributes{
		DisplayName: displayName,
		User:        username,
	}
	if val, ok := nearbysetup.DataUsageStrings[dataUsage]; ok {
		attrs.DataUsage = val
	} else {
		return nil, errors.Errorf("undefined dataUsage: %v", dataUsage)
	}
	if val, ok := nearbysetup.VisibilityStrings[visibility]; ok {
		attrs.Visibility = val
	} else {
		return nil, errors.Errorf("undefined visibility: %v", visibility)
	}

	const expectedKey = "CHROME VERSION"
	version, err := systemlogs.GetSystemLogs(ctx, tconn, expectedKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed getting system logs to check Chrome version")
	}
	if version == "" {
		return nil, errors.Wrap(err, "system logs result empty")
	}
	// The output on test images contains 'unknown' for the channel, i.e. '91.0.4435.0 unknown', so just extract the channel version.
	const versionPattern = `([0-9\.]+) [\w+]`
	r, err := regexp.Compile(versionPattern)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile Chrome version pattern")
	}
	versionMatch := r.FindStringSubmatch(version)
	if len(versionMatch) == 0 {
		return nil, errors.New("failed to find valid Chrome version")
	}
	attrs.ChromeVersion = versionMatch[1]

	lsb, err := lsbrelease.Load()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read lsb-release")
	}
	osVersion, ok := lsb[lsbrelease.Version]
	if !ok {
		return nil, errors.Wrap(err, "failed to read ChromeOS version from lsb-release")
	}
	attrs.ChromeOSVersion = osVersion

	board, ok := lsb[lsbrelease.Board]
	if !ok {
		return nil, errors.Wrap(err, "failed to read board from lsb-release")
	}
	attrs.Board = board

	model, err := testexec.CommandContext(ctx, "cros_config", "/", "name").Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read model from cros_config")
	}
	attrs.Model = string(model)

	return &attrs, nil
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
