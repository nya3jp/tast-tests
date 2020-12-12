// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/android/adb"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/ui/ossettings"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SendToAndroidMojo,
		Desc: "Send a file from CrOS to Android using Mojo APIs to control Nearby Share",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"files_app_test.png", "nearby_snippet.apk", "test_nearby_sender.js"},
	})
}

// SendToAndroidMojo tests sending an image to an Android device using Mojo APIs to control Nearby Share.
func SendToAndroidMojo(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(
		ctx,
		chrome.EnableFeatures("IntentHandlingSharing", "NearbySharing", "Sharesheet"),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Set up the Android device and start the nearby snippet apk.
	// Should change this name to something specific to the device for clearer identification.
	const androidName = "android_SendToAndroidMojo"
	snippet, err := androidInit(ctx, s, nearbyshare.SnippetDataUsageOffline, nearbyshare.SnippetVisibilityEveryone, androidName)
	if err != nil {
		s.Fatal(err)
	}
	defer snippet.StopSnippet(ctx)

	// Enable Nearby Share through the API exposed to OS Settings.
	if err := ossettings.Launch(ctx, tconn); err != nil {
		s.Fatal("Failed to launch OS settings: ", err)
	}
	settingsConn, err := ossettings.ChromeConn(ctx, cr)
	if err != nil {
		s.Fatal("Failed to establish conn to OS settings: ", err)
	}
	defer settingsConn.Close()
	if err := settingsConn.Call(ctx, nil, `function() {nearby_share.getNearbyShareSettings().setEnabled(true)}`); err != nil {
		s.Fatal("Failed to enable Nearby Share from OS settings: ", err)
	}

	// Set the device name as well. Should change this name to something specific to the device for clearer identification.
	const cbName = "cros_SendToAndroidMojo"
	if err := settingsConn.Call(ctx, nil, `async function(name) {nearby_share.getNearbyShareSettings().setDeviceName(name)}`, cbName); err != nil {
		s.Fatal("Failed to enable Nearby Share from OS settings: ", err)
	}

	// Setup the test file for sharing.
	const (
		previewImageFile = "files_app_test.png"
	)
	imageFileLocation := filepath.Join(filesapp.DownloadPath, previewImageFile)
	if err := fsutil.CopyFile(s.DataPath(previewImageFile), imageFileLocation); err != nil {
		s.Fatalf("Failed to copy the test image to %s: %s", imageFileLocation, err)
	}
	defer os.Remove(imageFileLocation)

	// Launch files.
	f, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Files app: ", err)
	}
	defer f.Release(ctx)

	// Find the file to share, right-click it, and choose the "Share" option to launch the share sheet.
	if err := f.OpenDownloads(ctx); err != nil {
		s.Fatal("Failed to open downloads folder: ", err)
	}
	if err := f.WaitForFile(ctx, previewImageFile, 10*time.Second); err != nil {
		s.Fatal("Waiting for test file failed: ", err)
	}
	if err := f.SelectContextMenu(ctx, previewImageFile, "Share"); err != nil {
		s.Fatal("Failed to click Share context menu option: ", err)
	}

	// Find and click the Nearby Share button on the share sheet.
	nsBtn, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeButton, Name: "Nearby Share"}, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find share sheet's Nearby Share button: ", err)
	}
	defer nsBtn.Release(ctx)

	if err := nsBtn.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click share sheet's Nearby Share button: ", err)
	}

	// Wait for Nearby Share menu to appear.
	if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{Role: ui.RoleTypeGenericContainer, Name: "Nearby Share"}, 10*time.Second); err != nil {
		s.Fatal("Failed waiting for Nearby Share window to open: ", err)
	}

	// Set up a conn to chrome://nearby where we can control the sending flow.
	nearbyConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://nearby/"))
	if err != nil {
		s.Fatal("Failed to set up Chrome conn to chrome://nearby/")
	}
	defer nearbyConn.Close()

	// Javascript to control the sending flow.
	if err := nearbyConn.WaitForExpr(ctx, `nearbyShare.mojom !== undefined`); err != nil {
		s.Fatal("Failed waiting for nearbyShare.mojom to load: ", err)
	}
	js, err := ioutil.ReadFile(s.DataPath("test_nearby_sender.js"))
	if err != nil {
		s.Fatal("Failed to load JS for NS sending: ", err)
	}
	// Set up an object to control the discovery flow.
	var testNearbySender chrome.JSObject
	if err := nearbyConn.Call(ctx, &testNearbySender, string(js)); err != nil {
		s.Fatal("Failed to set up the discovery test object: ", err)
	}

	// Start discovery.
	if err := testNearbySender.Call(ctx, nil, `async function() {await this.startDiscovery()}`); err != nil {
		s.Fatal("Failed to start discovery: ", err)
	}

	// Start receiving on the Android device.
	callbackID, err := snippet.ReceiveFile(cbName, androidName, 2*time.Minute)
	if err != nil {
		s.Fatal("Failed to start receiving on Android: ", err)
	}
	// Defer cancelling the share in case it fails.
	defer snippet.CancelReceivingFile()

	// Wait until the Android device is detected.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var targetExists bool
		if err := testNearbySender.Call(ctx, &targetExists, `function(name) {return this.shareTargetNameMap.get(name) != undefined}`, androidName); err != nil {
			return testing.PollBreak(err)
		}

		if !targetExists {
			var receivers string
			testNearbySender.Call(ctx, &receivers, `function() {return Array.from(this.shareTargetNameMap.keys())}`, androidName)
			s.Log("Receivers: ", receivers)
			return errors.New("share target not found yet")
		}

		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: 15 * time.Second}); err != nil {
		var receivers string
		testNearbySender.Call(ctx, &receivers, `function() {return Array.from(this.shareTargetNameMap.keys())+''}`, androidName)
		quicksettings.Show(ctx, tconn)
		testing.Sleep(ctx, 2*time.Second)
		s.Log("Receivers at time of failure: ", receivers)
		s.Fatal("Failed waiting to find the share target: ", err)
	}

	// Confirm the share.
	if err := testNearbySender.Call(ctx, nil, `async function(name) {await this.selectShareTarget(name)}`, androidName); err != nil {
		s.Fatal("Failed to accept the incoming share: ", err)
	}
	var res string
	if err := testNearbySender.Call(ctx, &res, `function() {return this.selectShareTargetRes}`); err != nil {
		s.Fatal("Failed to get selectShareTargetRes: ", err)
	}
	s.Logf("==== selectShareTarget res: %v ====", res)

	// Make sure the device selection succeeded without error.
	var confirmSuccess bool
	if err := testNearbySender.Call(ctx, &confirmSuccess, `function() {return this.confirmationManager != null}`); err != nil {
		s.Fatal("Failed to check if confirmation succeeded: ", err)
	}
	if !confirmSuccess {
		s.Fatal("Confirmation was not succesful")
	}

	// Wait for Android to be ready to receive the share.
	if err := snippet.EventWaitAndGet(callbackID, nearbyshare.SnippetEventOnLocalConfirmation, 2*time.Minute); err != nil {
		s.Fatal("Failed waiting for onLocalConfirmation event to know that Android is ready to receive share: ", err)
	}

	// Get the secure sharing token to confirm the share on Android.
	var token string
	if err := testNearbySender.Call(ctx, &token, `function() {return this.confirmationToken_}`); err != nil {
		s.Fatal("Failed to get confirmation token", err)
	}
	s.Log("Confirmation token: ", token)

	// Confirm the share.
	if err := snippet.AcceptTheSharing(token); err != nil {
		s.Fatal("Failed to accept the share on the Android device: ", err)
	}

	// Check the status until the transfer is complete.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var status int
		if err := testNearbySender.Call(ctx, &status, `function() {return this.currentTransferStatus}`); err != nil {
			return testing.PollBreak(err)
		}
		s.Logf("Transfer status: %v", status)
		if status != 6 {
			return errors.New("transfer not yet complete.")
		}
		return nil
	}, &testing.PollOptions{Timeout: 300 * time.Second, Interval: 250 * time.Millisecond}); err != nil {
		s.Fatal("Failed waiting for transfer to complete: ", err)
	}

	// Wait for Android to signal the sharing has completed.
	if err := snippet.EventWaitAndGet(callbackID, nearbyshare.SnippetEventOnStop, 2*time.Minute); err != nil {
		s.Fatal("Failed waiting for onStop to know that Android sharing has finished: ", err)
	}
}

func androidInit(ctx context.Context, s *testing.State, dataUsage nearbyshare.SnippetDataUsage, visibility nearbyshare.SnippetVisibility, displayName string) (*nearbyshare.SnippetDevice, error) {
	if err := adb.LaunchServer(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to launch adb server")
	}

	devices, err := adb.Devices(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list adb devices")
	}

	// Assuming just one device is connected for testing, otherwise can match the serial.
	if len(devices) != 1 {
		return nil, errors.Errorf("expected exactly 1 adb device, but found %v", len(devices))
	}
	testDevice := devices[0]

	// Launch and start the snippet server.
	// Note: Please notice the Android 11 permission issue, -g still can't grant "ALL FILE" access permission.
	snippet, err := nearbyshare.PrepareSnippetDevice(ctx, testDevice, s.DataPath(nearbyshare.NearbySnippetApk))
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up the snippet server")
	}

	if err := snippet.Initialize(); err != nil {
		return nil, errors.Wrap(err, "failed to initialize snippet server")
	}

	if err := snippet.SetupDevice(dataUsage, visibility, displayName); err != nil {
		return nil, errors.Wrap(err, "failed to configure Android Nearby Share settings")
	}

	return snippet, nil
}
