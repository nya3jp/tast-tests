// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/ui/ossettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: NearbyShareMojoSend,
		Desc: "Nearby Share JS control testing",
		Contacts: []string{
			"kyleshima@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"files_app_test.png"},
	})
}

// NearbyShareMojoSend tests the s ending flow, controlled by JS.
func NearbyShareMojoSend(ctx context.Context, s *testing.State) {
	const (
		// Name of the receiving Android phone.
		phoneName = "Kyle's Phone"
	)

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

	// Set up an object to control the discovery flow.
	var testNearbySender chrome.JSObject
	if err := nearbyConn.Call(ctx, &testNearbySender,
		`function() {return {
			shareTargetNameMap: new Map(), 

			confirmationManager: null,

			transferUpdateListener: null,

			currentTransferStatus: null,

			transferUpdateListenerReceiver_: null,

			mojoEventTarget_: null,

			listenerIDs_: null,

			getDiscoveryManager() {
				discoveryManager = nearbyShare.mojom.DiscoveryManager.getRemote();
				discoveryManager.onConnectionError.addListener(() => discoveryManager = null);
				return discoveryManager;
			},

			startDiscovery() {
				this.mojoEventTarget_ =
					new nearbyShare.mojom.ShareTargetListenerCallbackRouter();

				this.listenerIds_ = [
				this.mojoEventTarget_.onShareTargetDiscovered.addListener(
					this.onShareTargetDiscovered_.bind(this)),
				this.mojoEventTarget_.onShareTargetLost.addListener(
					this.onShareTargetLost_.bind(this)),
				];

				this.getDiscoveryManager()
					.startDiscovery(this.mojoEventTarget_.$.bindNewPipeAndPassRemote())
					.then(response => {
					if (!response.success) {
						// TODO(crbug.com/1123934): Show error.
						return;
					}
					});
			},

			stopDiscovery() {
				if (!this.mojoEventTarget_) {
				  return;
				}

				this.shareTargetNameMap.clear();
				this.listenerIds_.forEach(
					id => assert(this.mojoEventTarget_.removeListener(id)));
				this.mojoEventTarget_.$.close();
				this.mojoEventTarget_ = null;
			},

			selectShareTargetRes: null,

			selectShareTarget(targetName) {
				var shareTarget = this.shareTargetNameMap.get(targetName);
				if (!shareTarget) {
				  this.selectShareTargetRes = "no_share_target";
				  return;
				};
				import('./discovery_manager.js')
				  .then(m => m.getDiscoveryManager().selectShareTarget(shareTarget.id))
				  .then(response => {
					const {result, transferUpdateListener, confirmationManager} =
						response;
					if (result !== nearbyShare.mojom.SelectShareTargetResult.kOk) {
					  this.selectShareTargetRes = "bad_res"
					  return;
				    }

				    this.confirmationManager = confirmationManager;
				    this.transferUpdateListenerReceiver_ =
				        new nearbyShare.mojom.TransferUpdateListenerReceiver(this);
				    this.transferUpdateListenerReceiver_.$.bindHandle(
					    transferUpdateListener.handle);
				  });
			},

			onShareTargetDiscovered_(shareTarget) {
				this.shareTargetNameMap.set(shareTarget.name, shareTarget);
			},

			onShareTargetLost_(shareTarget) {
				this.shareTargetNameMap.delete(shareTarget.name);
			},

			onTransferUpdate(status, token) {
				if (token) {
				  this.confirmationToken_ = token;
				}
				this.currentTransferStatus = status;
				console.log('token: ', token)
				console.log('currentTransferStatus: ', status)
			},

			accept() {
				this.confirmationManager.accept();
			},

			reject() {
				this.confirmationManager.reject();
			},

			cancel() {
				this.confirmationManager.cancel();
			},
		}}`,
	); err != nil {
		s.Fatal("Failed to set up the discovery test object: ", err)
	}

	// Start discovery.
	if err := testNearbySender.Call(ctx, nil, `async function() {await this.startDiscovery()}`); err != nil {
		s.Fatal("Failed to start discovery: ", err)
	}

	// Manually send from Android. Wait until detected.
	s.Log("Start receiving now")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var targetExists bool
		if err := testNearbySender.Call(ctx, &targetExists, `function(name) {return this.shareTargetNameMap.get(name) != undefined}`, phoneName); err != nil {
			return testing.PollBreak(err)
		}

		if !targetExists {
			return errors.New("share target not found yet")
		}

		return nil
	}, nil); err != nil {
		s.Fatal("Failed waiting to find the share target: ", err)
	}

	// Confirm the share.
	if err := testNearbySender.Call(ctx, nil, `async function(name) {await this.selectShareTarget(name)}`, phoneName); err != nil {
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
}
