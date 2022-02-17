// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crossdevice is for controlling Cross Device features involving a paired Android phone and Chromebook.
package crossdevice

import (
	"context"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/android/mobly"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// AndroidPhotosPath is the directory where photos taken on the device can be located.
const AndroidPhotosPath = "/sdcard/DCIM/Camera"

// AndroidDevice represents an Android device that's been paired with the Chromebook (i.e. from the "Connected devices" section of OS settings).
// Android control is achieved by making RPCs to the Multidevice Snippet running on the Android device, or by using ADB commands directly.
type AndroidDevice struct {
	device        *adb.Device
	snippetClient *mobly.SnippetClient
}

// NewAndroidDevice returns an AndroidDevice that can be used to control the Android phone in Cross Device tests.
// Callers should defer Cleanup to ensure the resources used by the AndroidDevice are freed.
func NewAndroidDevice(ctx context.Context, d *adb.Device, apkZipPath string) (*AndroidDevice, error) {
	// Launch the snippet and create a client.
	snippetClient, err := mobly.NewSnippetClient(ctx, d, MultideviceSnippetMoblyPackage, apkZipPath, MultideviceSnippetApkName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start the snippet client for the Multidevice Snippet")
	}
	return &AndroidDevice{device: d, snippetClient: snippetClient}, nil
}

// ReconnectToSnippet restarts a connection to the Multidevice Snippet on Android device.
func (c *AndroidDevice) ReconnectToSnippet(ctx context.Context) error {
	return c.snippetClient.ReconnectToSnippet(ctx)
}

// Cleanup stops the Multidevice Snippet, removes port forwarding, and closes the TCP connection.
// This should be deferred after calling NewAndroidDevice to ensure the resources used by the AndroidDevice are released at the end of tests.
func (c *AndroidDevice) Cleanup(ctx context.Context) {
	c.snippetClient.Cleanup(ctx)
}

// DumpLogs saves the Android device's logcat output to a file.
func (c *AndroidDevice) DumpLogs(ctx context.Context, outDir, filename string) error {
	filePath := filepath.Join(outDir, filename)
	if err := c.device.DumpLogcat(ctx, filePath); err != nil {
		testing.ContextLog(ctx, "Failed to dump Android logs: ", err)
		return errors.Wrap(err, "failed to dump Android logs")
	}
	return nil
}

// ClearLogcat clears logcat so each test run can have only relevant logs.
func (c *AndroidDevice) ClearLogcat(ctx context.Context) error {
	if err := c.device.ClearLogcat(ctx); err != nil {
		return errors.Wrap(err, "failed to clear previous logcat logs")
	}
	return nil
}

// StartScreenRecording starts screen recording on the Android device.
// Defer the returned function to save the recording and clean up on the Android side.
func (c *AndroidDevice) StartScreenRecording(ctx context.Context, filename, outDir string) (func(context.Context, func() bool) error, error) {
	return c.device.StartScreenRecording(ctx, filename, outDir)
}

// Pair pairs the Android device to nearby Chromebooks signed in with the same GAIA account.
// This will cause the phone to be listed under the "Connected devices" section of OS settings,
// allowing use of Cross Device features (Smart Lock, Phone Hub, etc.) on the Chromebook.
// Calling this function effectively bypasses the crossdevice onboarding flow that is triggered from OOBE or OS settings.
// One notable difference is unlike the normal onboarding flow, not all features in the "Connected devices" page will be
// enabled by default. Some (Phone Hub, WiFi Sync) may need to be toggled on after calling Pair.
func (c *AndroidDevice) Pair(ctx context.Context) error {
	user, err := c.device.GoogleAccount(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get device user account")
	}
	res, err := c.snippetClient.RPC(ctx, mobly.DefaultRPCResponseTimeout, "enableBetterTogetherHost", user)
	if err != nil {
		return err
	}
	callbackID := res.Callback

	// Wait for the snippet to confirm that it has completed the crossdevice setup operation on the Android side.
	if _, err := c.snippetClient.EventWaitAndGet(ctx, callbackID, "onBeToHostEnableStatus", 30*time.Second); err != nil {
		return errors.Wrap(err, "failed waiting for onBeToHostEnableStatus event to know that crossdevice setup is complete on Android")
	}

	return nil
}

// GenerateMessageNotification creates a message notification on the phone.
// The notification has inline reply enabled, so it can be used to test Phone Hub's notification features.
// The notification title and message text can be specified by the inputs.
// You can create multiple distinct notifications by calling this with different notification IDs.
// The returned function will retrieve the reply to the notification.
func (c *AndroidDevice) GenerateMessageNotification(ctx context.Context, id int, title, text string) (func(context.Context) (string, error), error) {
	res, err := c.snippetClient.RPC(ctx, mobly.DefaultRPCResponseTimeout, "generateMessageNotification", title, text, id)
	if err != nil {
		return nil, err
	}
	callbackID := res.Callback

	// Return a function that will wait for a reply to the message notification and return the replied text.
	return func(ctx context.Context) (string, error) {
		res, err := c.snippetClient.EventWaitAndGet(ctx, callbackID, "replyReceived", 10*time.Second)
		if err != nil {
			return "", errors.Wrap(err, "failed to wait for replyReceived snippet event")
		}
		reply, ok := res.Data["reply"]
		if !ok {
			return "", errors.New("replyReceived event did not contain a reply")
		}
		replyStr, ok := reply.(string)
		if !ok {
			return "", errors.Wrap(err, "reply in replyReceived's response was not a string")
		}
		return replyStr, nil
	}, nil
}

// EnablePhoneHubNotifications sets the flag on Android to allow Phone Hub to receive notification updates.
func (c *AndroidDevice) EnablePhoneHubNotifications(ctx context.Context) error {
	return c.device.ShellCommand(ctx, "cmd", "notification", "allow_listener", "com.google.android.gms/.auth.proximity.phonehub.PhoneHubNotificationListenerService").Run()
}

// SetPIN sets a screen lock PIN on Android.
func (c *AndroidDevice) SetPIN(ctx context.Context) error {
	if err := c.device.SetPIN(ctx); err != nil {
		return errors.Wrap(err, "failed to set a screen lock PIN on Android")
	}
	return nil
}

// ClearPIN clears a screen lock PIN on Android.
func (c *AndroidDevice) ClearPIN(ctx context.Context) error {
	if err := c.device.ClearPIN(ctx); err != nil {
		return errors.Wrap(err, "failed to clear  a screen lock PIN on Android")
	}
	return nil
}

// GetAndroidAttributes returns the AndroidAttributes for the device.
func (c *AndroidDevice) GetAndroidAttributes(ctx context.Context) (*AndroidAttributes, error) {
	return GetAndroidAttributes(ctx, c.device)
}

// ToggleDoNotDisturb toggles the Do Not Disturb setting on the Android device.
func (c *AndroidDevice) ToggleDoNotDisturb(ctx context.Context, enable bool) error {
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
func (c *AndroidDevice) DoNotDisturbEnabled(ctx context.Context) (bool, error) {
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
func (c *AndroidDevice) WaitForDoNotDisturb(ctx context.Context, enabled bool, timeout time.Duration) error {
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

// TakePhoto takes a photo with Camera app and returns the name of the new photo taken.
func (c *AndroidDevice) TakePhoto(ctx context.Context) (string, error) {
	if err := c.device.SendIntentCommand(ctx, "android.media.action.STILL_IMAGE_CAMERA", "").Run(); err != nil {
		return "", errors.Wrap(err, "failed to open camera")
	}
	// Close the Camera app by pressing the back button when this function exits.
	defer c.device.PressKeyCode(ctx, "KEYCODE_BACK")

	mostRecentPhoto, err := c.GetMostRecentPhoto(ctx)
	if err != nil {
		return "", err
	}

	uiDevice, err := ui.NewDevice(ctx, c.device)
	if err != nil {
		return "", errors.Wrap(err, "failed to connect to the UI Automator server")
	}
	defer uiDevice.Close(ctx)
	shutterButton := uiDevice.Object(ui.DescriptionMatches("Take.*photo"))
	if err := shutterButton.WaitForExists(ctx, 3*time.Second); err != nil {
		return "", errors.Wrap(err, "cannot find the shutter button in the Google Camera app")
	}
	if err := shutterButton.Click(ctx); err != nil {
		return "", errors.Wrap(err, "failed to take a photo")
	}

	// Wait for a new photo to appear.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if photo, err := c.GetMostRecentPhoto(ctx); err != nil {
			return testing.PollBreak(err)
		} else if photo == mostRecentPhoto {
			return errors.Errorf("Still waiting for new photo to appear, current most recent photo is %s", mostRecentPhoto)
		} else {
			mostRecentPhoto = photo
			return nil
		}
	}, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil {
		return "", errors.Wrap(err, "no new photo found")
	}

	return mostRecentPhoto, nil
}

// GetMostRecentPhoto returns the file name of the most recent photo taken on the device.
// Returns an empty string if there's no photo on the device.
func (c *AndroidDevice) GetMostRecentPhoto(ctx context.Context) (string, error) {
	photos, err := c.device.ListContents(ctx, AndroidPhotosPath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to list files under %s", AndroidPhotosPath)
	}
	mostRecentPhoto := ""
	if len(photos) > 0 {
		mostRecentPhoto = photos[len(photos)-1]
	}
	return mostRecentPhoto, nil
}

// FileSize returns the size of the specified file in bytes. Returns an error if the file does not exist.
func (c *AndroidDevice) FileSize(ctx context.Context, filename string) (int64, error) {
	return c.device.FileSize(ctx, filename)
}

// SHA256Sum returns the sha256sum of the specified file as a string.
func (c *AndroidDevice) SHA256Sum(ctx context.Context, filename string) (string, error) {
	return c.device.SHA256Sum(ctx, filename)
}

// RemoveMediaFile removes the media file specified by filePath from the Android device's storage and media gallery.
func (c *AndroidDevice) RemoveMediaFile(ctx context.Context, filePath string) error {
	return c.device.RemoveMediaFile(ctx, filePath)
}

// BatteryLevel returns the current battery level.
func (c *AndroidDevice) BatteryLevel(ctx context.Context) (int, error) {
	res, err := c.device.ShellCommand(ctx, "dumpsys", "battery").Output(testexec.DumpLogOnError)
	if err != nil {
		return -1, errors.Wrap(err, "failed to get battery status")
	}

	r := regexp.MustCompile(`level: (\d+)`)
	m := r.FindStringSubmatch(string(res))
	if len(m) == 0 {
		return -1, errors.Wrap(err, "failed to extract battery percentage from adb response")
	}
	level, err := strconv.Atoi(m[1])
	if err != nil {
		return -1, errors.Wrapf(err, "failed to convert battery level %v to int", m[0])
	}
	return level, nil
}
