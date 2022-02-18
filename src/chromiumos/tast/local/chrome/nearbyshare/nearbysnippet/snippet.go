// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbysnippet is for interacting with the Nearby Snippet which provides automated control of Android Nearby share.
package nearbysnippet

import (
	"context"
	"encoding/json"
	"path/filepath"
	"time"

	"chromiumos/tast/common/android"
	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/android/mobly"
	"chromiumos/tast/common/android/ui"
	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/testing"
)

// AndroidNearbyDevice represents a connected Android device equipped with Nearby Share controls.
// Nearby Share control is achieved by making RPCs to the Nearby Snippet running on the Android device.
type AndroidNearbyDevice struct {
	Device           *adb.Device
	snippetClient    *mobly.SnippetClient
	transferCallback string
	uiDevice         *ui.Device
}

// New initializes the specified Android device for Nearby Sharing by setting up the Nearby snippet on the device
// and initializing a Mobly snippet client to communicate with it.
// Callers should defer Cleanup to ensure the resources used by the AndroidNearbyDevice are freed.
func New(ctx context.Context, d *adb.Device, apkZipPath string, overrideGMS bool) (a *AndroidNearbyDevice, err error) {
	a = &AndroidNearbyDevice{Device: d}
	// Override the necessary GMS Core flags.
	if overrideGMS {
		if err := overrideGMSCoreFlags(ctx, a.Device); err != nil {
			return a, err
		}
	}

	// Grant the MANAGE_EXTERNAL_STORAGE permission to the Nearby Snippet if the SDK version is 30+ (Android 11+).
	// This is required for the Android sender flow, since the Nearby Snippet sends files from external storage.
	const needsStoragePermissionsVersion = 30
	var permissions []string
	if sdkVersion, err := a.Device.SDKVersion(ctx); err != nil {
		return a, errors.Wrap(err, "failed to get android sdk version")
	} else if sdkVersion >= needsStoragePermissionsVersion {
		permissions = append(permissions, "MANAGE_EXTERNAL_STORAGE")
	}

	// Launch the snippet and create a client.
	snippetClient, err := mobly.NewSnippetClient(ctx, a.Device, moblyPackage, apkZipPath, ApkName, permissions...)
	if err != nil {
		return a, errors.Wrap(err, "failed to start the snippet client")
	}
	a.snippetClient = snippetClient
	return a, nil
}

// ReconnectToSnippet restarts a connection to the Nearby Snippet on Android device.
func (a *AndroidNearbyDevice) ReconnectToSnippet(ctx context.Context) error {
	return a.snippetClient.ReconnectToSnippet(ctx)
}

// Cleanup stops the Nearby Snippet, removes port forwarding, and closes the TCP connection.
// This should be deferred after calling New to ensure the resources used by the AndroidNearbyDevice are released at the end of tests.
func (a *AndroidNearbyDevice) Cleanup(ctx context.Context) {
	a.snippetClient.Cleanup(ctx)
}

// gmsOverrideCmd constructs the shell commands to override the GMS Core flags required by the Nearby Snippet.
func gmsOverrideCmd(ctx context.Context, device *adb.Device, pack, flag string) *testexec.Cmd {
	return device.ShellCommand(ctx,
		"am", "broadcast", "-a", "com.google.android.gms.phenotype.FLAG_OVERRIDE",
		"--es", "package", pack,
		"--es", "user", `\*`,
		"--esa", "flags", flag,
		"--esa", "values", "true",
		"--esa", "types", "boolean",
		"com.google.android.gms")
}

// overrideGMSCoreFlags overrides the GMS Core flags required by the Nearby Snippet.
// Overriding the flags over adb requires the device to be rooted (i.e. userdebug build).
func overrideGMSCoreFlags(ctx context.Context, device *adb.Device) error {
	// Get root access.
	rootCmd := device.Command(ctx, "root")
	if err := rootCmd.Run(); err != nil {
		return errors.Wrap(err, "failed to start adb as root")
	}

	overrideCmd1 := gmsOverrideCmd(ctx, device, "com.google.android.gms.nearby", "sharing_package_whitelist_check_bypass")
	overrideCmd2 := gmsOverrideCmd(ctx, device, "com.google.android.gms", "GoogleCertificatesFlags__enable_debug_certificates")

	if err := overrideCmd1.Run(); err != nil {
		return errors.Wrap(err, "failed to override sharing_package_whitelist_check_bypass flag")
	}
	if err := overrideCmd2.Run(); err != nil {
		return errors.Wrap(err, "failed to override GoogleCertificatesFlags__enable_debug_certificates flag")
	}
	return nil
}

// DumpLogs saves the Android device's logcat output to a file.
func (a *AndroidNearbyDevice) DumpLogs(ctx context.Context, outDir, filename string) error {
	filePath := filepath.Join(outDir, filename)
	if err := a.Device.DumpLogcat(ctx, filePath); err != nil {
		testing.ContextLog(ctx, "Failed to dump Android logs: ", err)
		return errors.Wrap(err, "failed to dump Android logs")
	}
	return nil
}

// ClearLogcat clears logcat so each test run can have only relevant logs.
func (a *AndroidNearbyDevice) ClearLogcat(ctx context.Context) error {
	if err := a.Device.ClearLogcat(ctx); err != nil {
		return errors.Wrap(err, "failed to clear previous logcat logs")
	}
	return nil
}

// SHA256Sum computes the sha256sum of the specified file on the Android device.
func (a *AndroidNearbyDevice) SHA256Sum(ctx context.Context, filename string) (string, error) {
	return a.Device.SHA256Sum(ctx, filename)
}

// StageFile pushes the specified file to the Android device to be used in sending.
func (a *AndroidNearbyDevice) StageFile(ctx context.Context, file string) error {
	androidDst := filepath.Join(android.DownloadDir, SendDir, filepath.Base(file))
	if err := a.Device.PushFile(ctx, file, androidDst); err != nil {
		return errors.Wrapf(err, "failed to push %v to %v", file, androidDst)
	}
	return nil
}

// ClearDownloads clears the device's Downloads folder, where outgoing shares are staged and incoming shares are received.
func (a *AndroidNearbyDevice) ClearDownloads(ctx context.Context) error {
	if err := a.Device.RemoveContents(ctx, android.DownloadDir); err != nil {
		return errors.Wrap(err, "failed to clear Android downloads directory")
	}
	return nil
}

// GetNearbySharingVersion retrieves the Android device's Nearby Sharing version.
func (a *AndroidNearbyDevice) GetNearbySharingVersion(ctx context.Context) (string, error) {
	res, err := a.snippetClient.RPC(ctx, mobly.DefaultRPCResponseTimeout, "getNearbySharingVersion")
	if err != nil {
		return "", err
	}
	var version string
	if err := json.Unmarshal(res.Result, &version); err != nil {
		return "", errors.Wrap(err, "failed to parse version number from json result")
	}
	return version, nil
}

// settingTimeoutSeconds is the time to wait for the Nearby Snippet to return settings values.
// Only used by getDeviceName, getDataUsage, and getVisibility RPCs.
const settingTimeoutSeconds = 10

// GetDeviceName retrieve's the Android device's display name for Nearby Share.
func (a *AndroidNearbyDevice) GetDeviceName(ctx context.Context) (string, error) {
	var name string
	res, err := a.snippetClient.RPC(ctx, settingTimeoutSeconds*time.Second, "getDeviceName", settingTimeoutSeconds)
	if err != nil {
		return name, err
	}
	if err := json.Unmarshal(res.Result, &name); err != nil {
		return "", errors.Wrap(err, "failed to parse device name from json result")
	}
	return name, nil
}

// GetDataUsage retrieve's the Android device's Nearby Share data usage setting.
func (a *AndroidNearbyDevice) GetDataUsage(ctx context.Context) (DataUsage, error) {
	var data DataUsage
	res, err := a.snippetClient.RPC(ctx, settingTimeoutSeconds*time.Second, "getDataUsage", settingTimeoutSeconds)
	if err != nil {
		return data, err
	}
	if err := json.Unmarshal(res.Result, &data); err != nil {
		return data, errors.Wrap(err, "failed to parse data usage from json result")
	}
	return data, nil
}

// GetVisibility retrieve's the Android device's Nearby Share visibility setting.
func (a *AndroidNearbyDevice) GetVisibility(ctx context.Context) (Visibility, error) {
	var vis Visibility
	res, err := a.snippetClient.RPC(ctx, settingTimeoutSeconds*time.Second, "getVisibility", settingTimeoutSeconds)
	if err != nil {
		return vis, err
	}
	if err := json.Unmarshal(res.Result, &vis); err != nil {
		return vis, errors.Wrap(err, "failed to parse device visibility from json result")
	}
	return vis, nil
}

// SetupDevice configures the Android device's Nearby Share settings.
func (a *AndroidNearbyDevice) SetupDevice(ctx context.Context, dataUsage DataUsage, visibility Visibility, name string) error {
	_, err := a.snippetClient.RPC(ctx, mobly.DefaultRPCResponseTimeout, "setupDevice", dataUsage, visibility, name)
	return err
}

// SetEnabled sets Nearby Share enabled.
func (a *AndroidNearbyDevice) SetEnabled(ctx context.Context, enabled bool) error {
	_, err := a.snippetClient.RPC(ctx, mobly.DefaultRPCResponseTimeout, "setEnabled", enabled)
	return err
}

// ReceiveFile starts receiving with a timeout.
// Sets the AndroidNearbyDevice's transferCallback, which is needed when awaiting follow-up SnippetEvents when calling eventWaitAndGet.
func (a *AndroidNearbyDevice) ReceiveFile(ctx context.Context, senderName, receiverName string, isHighVisibility bool, turnaroundTime time.Duration) error {
	// Reset the transferCallback between shares.
	a.transferCallback = ""
	res, err := a.snippetClient.RPC(ctx, mobly.DefaultRPCResponseTimeout, "receiveFile", senderName, receiverName, isHighVisibility, int(turnaroundTime.Seconds()))
	if err != nil {
		return err
	}
	a.transferCallback = res.Callback
	return nil
}

// AwaitReceiverAccept should be used to wait for the onAwaitingReceiverAccept SnippetEvent, which indicates
// that the Android sender has successfully connected to the receiver. The response includes the secure connection token.
func (a *AndroidNearbyDevice) AwaitReceiverAccept(ctx context.Context, timeout time.Duration) (string, error) {
	if a.transferCallback == "" {
		return "", errors.New("transferCallback is not set, a share needs to be initiated first")
	}
	res, err := a.snippetClient.EventWaitAndGet(ctx, a.transferCallback, string(SnippetEventOnAwaitingReceiverAccept), timeout)
	if err != nil {
		return "", errors.Wrap(err, "failed waiting for onAwaitingReceiverAccept event to know that Android sender has connected to receiver")
	}

	token, ok := res.Data["token"]
	if !ok {
		return "", errors.New("onAwaitingReceiverAccept event did not include a token")
	}
	tokenStr, ok := token.(string)
	if !ok {
		return "", errors.Wrap(err, "share token in onAwaitingReceiverAccept response was not a string")
	}
	return tokenStr, nil
}

// AwaitReceiverConfirmation should be used after ReceiveFile to wait for the onLocalConfirmation SnippetEvent, which indicates
// that the Android device has detected the incoming share and is awaiting confirmation to begin the transfer.
func (a *AndroidNearbyDevice) AwaitReceiverConfirmation(ctx context.Context, timeout time.Duration) error {
	if a.transferCallback == "" {
		return errors.New("transferCallback is not set, ReceiveFile should be executed first")
	}
	if _, err := a.snippetClient.EventWaitAndGet(ctx, a.transferCallback, string(SnippetEventOnLocalConfirmation), timeout); err != nil {
		return errors.Wrap(err, "failed waiting for onLocalConfirmation event to know that Android is ready to start the transfer")
	}
	return nil
}

// AwaitSharingStopped waits for the onStop event, which indicates that sharing has stopped and Android Nearby Share teardown
// tasks have been completed. It does not necessarily indicate that the transfer succeeded.
func (a *AndroidNearbyDevice) AwaitSharingStopped(ctx context.Context, timeout time.Duration) error {
	if a.transferCallback == "" {
		return errors.New("transferCallback is not set, a share needs to be initiated first")
	}
	if _, err := a.snippetClient.EventWaitAndGet(ctx, a.transferCallback, string(SnippetEventOnStop), timeout); err != nil {
		return errors.Wrap(err, "failed waiting for onStop event to know that transfer is complete on Android")
	}
	return nil
}

// AcceptTheSharing accepts the share on the receiver side.
func (a *AndroidNearbyDevice) AcceptTheSharing(ctx context.Context, token string) error {
	var err error
	if token == "" {
		// Sometimes there will be no sharing token for in-contact shares.
		// In this case, sending nil as the RPC param will make the snippet skip the token verification.
		_, err = a.snippetClient.RPC(ctx, mobly.DefaultRPCResponseTimeout, "acceptTheSharing", nil)
	} else {
		_, err = a.snippetClient.RPC(ctx, mobly.DefaultRPCResponseTimeout, "acceptTheSharing", token)
	}
	return err
}

// CancelReceivingFile ends Nearby Share on the receiving side. This is used to fail fast instead of waiting for ReceiveFile's timeout.
func (a *AndroidNearbyDevice) CancelReceivingFile(ctx context.Context) error {
	_, err := a.snippetClient.RPC(ctx, mobly.DefaultRPCResponseTimeout, "cancelReceivingFile")
	return err
}

// SendFile starts sending with a timeout.
// Sets the AndroidNearbyDevice's transferCallback, which is needed when awaiting follow-up SnippetEvents when calling eventWaitAndGet.
func (a *AndroidNearbyDevice) SendFile(ctx context.Context, senderName, receiverName, shareFileName string, mimetype nearbycommon.MimeType, turnaroundTime time.Duration) error {
	// Reset the transferCallback between shares.
	a.transferCallback = ""
	res, err := a.snippetClient.RPC(ctx, mobly.DefaultRPCResponseTimeout, "sendFile", senderName, receiverName, shareFileName, mimetype, int(turnaroundTime.Seconds()))
	if err != nil {
		return err
	}
	a.transferCallback = res.Callback
	return nil
}

// CancelSendingFile ends Nearby Share on the sending side. This is used to fail fast instead of waiting for SendFile's timeout.
func (a *AndroidNearbyDevice) CancelSendingFile(ctx context.Context) error {
	_, err := a.snippetClient.RPC(ctx, mobly.DefaultRPCResponseTimeout, "cancelSendingFile")
	return err
}

// Sync synchronizes contact information and certificates on the Android device. This should be used before attempting to receive a contacts share.
func (a *AndroidNearbyDevice) Sync(ctx context.Context) error {
	_, err := a.snippetClient.RPC(ctx, mobly.DefaultRPCResponseTimeout, "sync")
	return err
}

// InitUI initializes a UI automator connection to the Android device. Callers should defer CloseUI to free the associated resources.
func (a *AndroidNearbyDevice) InitUI(ctx context.Context) error {
	d, err := ui.NewDevice(ctx, a.Device)
	if err != nil {
		return errors.Wrap(err, "failed initializing UI automator")
	}
	a.uiDevice = d
	return nil
}

// CloseUI closes the UI automator connection.
func (a *AndroidNearbyDevice) CloseUI(ctx context.Context) error {
	return a.uiDevice.Close(ctx)
}

// WaitForInContactSenderUI waits for the sharing UI that appears when there is an incoming share from a contact.
func (a *AndroidNearbyDevice) WaitForInContactSenderUI(ctx context.Context, sender string, timeout time.Duration) error {
	senderText := a.uiDevice.Object(ui.ID("com.google.android.gms:id/title"))
	return senderText.WaitForText(ctx, sender, timeout)
}

// AcceptUI accepts the incoming contacts share through the UI and waits for the share to finish by waiting for the receiving UI to be gone.
func (a *AndroidNearbyDevice) AcceptUI(ctx context.Context, timeout time.Duration) error {
	acceptBtn := a.uiDevice.Object(ui.ID("com.google.android.gms:id/accept_btn"))
	if err := acceptBtn.WaitForExists(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed waiting for Accept button to exist")
	}
	if err := acceptBtn.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Accept button")
	}
	receiveCard := a.uiDevice.Object(ui.ID("com.google.android.gms:id/card"))
	if err := receiveCard.WaitUntilGone(ctx, timeout); err != nil {
		return errors.Wrap(err, "failed waiting for receive UI to be gone")
	}
	return nil
}

// AndroidAttributes contains information about the Android device and its settings that are relevant to Nearby Share.
// "Android" is redundantly prepended to the field names to make them easy to distinguish from CrOS attributes in test logs.
type AndroidAttributes struct {
	BasicAttributes    *crossdevice.AndroidAttributes
	DisplayName        string
	DataUsage          string
	Visibility         string
	NearbyShareVersion string
}

// GetAndroidAttributes returns the AndroidAttributes for the device.
func (a *AndroidNearbyDevice) GetAndroidAttributes(ctx context.Context) (*AndroidAttributes, error) {
	// Get the base set of Android attributes used in all crossdevice tests.
	basicAttributes, err := crossdevice.GetAndroidAttributes(ctx, a.Device)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get base set of crossdevice Android attributes for reporting")
	}

	// Add nearby specific attributes.
	metadata := AndroidAttributes{
		BasicAttributes: basicAttributes,
	}

	displayName, err := a.GetDeviceName(ctx)
	if err != nil {
		return nil, err
	}
	metadata.DisplayName = displayName

	dataUsage, err := a.GetDataUsage(ctx)
	if err != nil {
		return nil, err
	}
	if val, ok := DataUsageStrings[dataUsage]; ok {
		metadata.DataUsage = val
	} else {
		return nil, errors.Errorf("undefined dataUsage: %v", dataUsage)
	}

	visibility, err := a.GetVisibility(ctx)
	if err != nil {
		return nil, err
	}
	if val, ok := VisibilityStrings[visibility]; ok {
		metadata.Visibility = val
	} else {
		return nil, errors.Errorf("undefined visibility: %v", visibility)
	}

	nearbyVersion, err := a.GetNearbySharingVersion(ctx)
	if err != nil {
		return nil, err
	}
	metadata.NearbyShareVersion = nearbyVersion

	return &metadata, nil
}
