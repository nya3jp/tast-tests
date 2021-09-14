// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package multidevice is for interacting with a connected Android device and controlling the multidevice suite.
package multidevice

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/android/mobly"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// ConnectedAndroidDevice represents an Android device that's been connected to the Chromebook as part of the multidevice feature suite.
// Android control is achieved by making RPCs to the Multidevice Snippet running on the Android device.
type ConnectedAndroidDevice struct {
	device        *adb.Device
	snippetClient *mobly.SnippetClient
}

// NewConnectedAndroidDevice prepares the target Android device for Multidevice feature control
// by setting up the Multidevice Snippet on the device and initializing a Mobly snippet client to communicate with it.
// Callers should defer Cleanup to ensure the resources used by the ConnectedAndroidDevice are freed.
func NewConnectedAndroidDevice(ctx context.Context, d *adb.Device, apkZipPath string) (*ConnectedAndroidDevice, error) {
	// Launch the snippet and create a client.
	snippetClient, err := mobly.NewSnippetClient(ctx, d, SnippetMoblyPackage, apkZipPath, SnippetApkName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start the snippet client")
	}
	return &ConnectedAndroidDevice{device: d, snippetClient: snippetClient}, nil
}

// ReconnectToSnippet restarts a connection to the Multidevice Snippet on Android device.
func (c *ConnectedAndroidDevice) ReconnectToSnippet(ctx context.Context) error {
	return c.snippetClient.ReconnectToSnippet(ctx)
}

// Cleanup stops the Multidevice Snippet, removes port forwarding, and closes the TCP connection.
// This should be deferred after calling New to ensure the resources used by the ConnectedAndroidDevice are released at the end of tests.
func (c *ConnectedAndroidDevice) Cleanup(ctx context.Context) {
	c.snippetClient.Cleanup(ctx)
}

// DumpLogs saves the Android device's logcat output to a file.
func (c *ConnectedAndroidDevice) DumpLogs(ctx context.Context, outDir, filename string) error {
	filePath := filepath.Join(outDir, filename)
	if err := c.device.DumpLogcat(ctx, filePath); err != nil {
		testing.ContextLog(ctx, "Failed to dump Android logs: ", err)
		return errors.Wrap(err, "failed to dump Android logs")
	}
	return nil
}

// ClearLogcat clears logcat so each test run can have only relevant logs.
func (c *ConnectedAndroidDevice) ClearLogcat(ctx context.Context) error {
	if err := c.device.ClearLogcat(ctx); err != nil {
		return errors.Wrap(err, "failed to clear previous logcat logs")
	}
	return nil
}

// Connect connects the Android device to nearby Chromebooks signed in with the same GAIA account.
// This enables features under the "Connected devices" section of OS settings (Smart Lock, Phone Hub, etc.) to be used on the Chromebook.
// Calling this function allows you to bypass the multidevice onboarding flow that is triggered from OOBE or OS settings.
func (c *ConnectedAndroidDevice) Connect(ctx context.Context, timeout time.Duration) error {
	user, err := c.device.GoogleAccount(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get device user account")
	}
	res, err := c.snippetClient.RPC(ctx, mobly.DefaultRPCResponseTimeout, "enableBetterTogetherHost", user)
	if err != nil {
		return err
	}
	callbackID := res.Callback

	// Wait for the snippet to confirm that it has completed the multidevice setup operation on the Android side.
	if _, err := c.snippetClient.EventWaitAndGet(ctx, callbackID, "onBeToHostEnableStatus", timeout); err != nil {
		return errors.Wrap(err, "failed waiting for onBeToHostEnableStatus event to know that multidevice setup is complete on Android")
	}

	return nil
}

// ToggleDoNotDisturb toggles the Do Not Disturb setting on the Android device.
func (c *ConnectedAndroidDevice) ToggleDoNotDisturb(ctx context.Context, enable bool) error {
	status := "off"
	if enable {
		status = "on"
	}
	if err := c.device.ShellCommand(ctx, "cmd", "notification", "set_dnd", status).Run(); err != nil {
		return errors.Wrapf(err, "failed to set Do Not Disturb to %v", status)
	}
	return nil
}

// DoNotDisturbEnabled returns true if Do Not Disturb is enabled, and false if it is disabled.
func (c *ConnectedAndroidDevice) DoNotDisturbEnabled(ctx context.Context) (bool, error) {
	res, err := c.device.ShellCommand(ctx, "sh", "-c", "settings list global | grep zen_mode=").Output(testexec.DumpLogOnError)
	if err != nil {
		return false, errors.Wrap(err, "failed to get Do Not Disturb status")
	}
	if strings.Contains(string(res), "0") {
		return false, nil
	}
	// Any status that's not 0 corresponds to DND being enabled.
	return true, nil
}

// WaitForDoNotDisturb waits for Do Not Disturb to be enabled/disabled.
func (c *ConnectedAndroidDevice) WaitForDoNotDisturb(ctx context.Context, enabled bool, timeout time.Duration) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if curr, err := c.DoNotDisturbEnabled(ctx); err != nil {
			return err
		} else if curr != enabled {
			return errors.New("current Do Not Disturb status does not match the desired status")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed waiting for desired Do Not Disturb status")
	}
	return nil
}

// AndroidAttributes contains information about the Android device and its settings that are relevant to Nearby Share.
// "Android" is redundantly prepended to the field names to make them easy to distinguish from CrOS attributes in test logs.
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
func (c *ConnectedAndroidDevice) GetAndroidAttributes(ctx context.Context) (*AndroidAttributes, error) {
	var metadata AndroidAttributes

	user, err := c.device.GoogleAccount(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get device user account")
	}
	metadata.User = user

	gmsVersion, err := c.device.GMSCoreVersion(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get GMS Core version")
	}
	metadata.GMSCoreVersion = gmsVersion

	androidVersion, err := c.device.AndroidVersion(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Android version")
	}
	metadata.AndroidVersion = androidVersion

	sdkVersion, err := c.device.SDKVersion(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Android SDK version")
	}
	metadata.SDKVersion = sdkVersion

	metadata.ProductName = c.device.Product
	metadata.ModelName = c.device.Model
	metadata.DeviceName = c.device.Device

	mac, err := c.device.BluetoothMACAddress(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the Bluetooth MAC address")
	}
	metadata.BluetoothMACAddress = mac

	return &metadata, nil
}
