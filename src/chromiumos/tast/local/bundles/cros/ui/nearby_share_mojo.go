// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/ui/ossettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: NearbyShareMojo,
		Desc: "Nearby Share JS control testing",
		Contacts: []string{
			"kyleshima@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// NearbyShareMojo tests high-vis mode controlled by JS.
func NearbyShareMojo(ctx context.Context, s *testing.State) {
	const (
		// Name of the file to be received from the Android phone.
		filename = "nearby-share-test.txt"
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

	// Launch settings.
	if err := ossettings.Launch(ctx, tconn); err != nil {
		s.Fatal("Failed to launch OS settings: ", err)
	}

	// Get the settings conn.
	settingsConn, err := ossettings.ChromeConn(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get the OS settings Chrome conn: ", err)
	}
	defer settingsConn.Close()

	// Set up the observer object.
	var observer chrome.JSObject
	if err := settingsConn.Call(ctx, &observer,
		`function() {return {
			inHighVis: null,
			shareTarget: null,
			metadata: null,
			onHighVisibilityChanged: function(inHighVisibility) {
				this.inHighVis = inHighVisibility;
				console.log(this.inHighVis);
			},
			onTransferUpdate: function(shareTarget, metadata) {
				this.shareTarget = shareTarget;
				this.metadata = metadata;
				console.log(this.shareTarget);
				console.log(this.metadata);
			},
		}}`,
	); err != nil {
		s.Fatal("Failed to set up the observer: ", err)
	}

	// Add the observer to the ReceiveManager.
	if err := settingsConn.WaitForExpr(ctx, `nearby_share !== undefined`); err != nil {
		s.Fatal("Failed waiting for nearby_share to load: ", err)
	}
	if err := settingsConn.Call(ctx, nil, `nearby_share.observeReceiveManager`, &observer); err != nil {
		s.Fatal("Failed to add observer to ReceiveManager: ", err)
	}

	// Start high-vis mode.
	var receiveManager chrome.JSObject
	if err := settingsConn.Call(ctx, &receiveManager, `nearby_share.getReceiveManager`); err != nil {
		s.Fatal("Failed to get receive manager: ", err)
	}
	if err := receiveManager.Call(ctx, nil, `async function() {await this.registerForegroundReceiveSurface()}`); err != nil {
		s.Fatal("Failed to enter high-vis mode: ", err)
	}

	// Manually send from Android. Wait until detected.
	s.Log("Send now")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var targetExists bool
		if err := observer.Call(ctx, &targetExists, `function() {return this.shareTarget != null}`); err != nil {
			return testing.PollBreak(err)
		}

		if !targetExists {
			return errors.New("share target not found yet")
		}

		return nil
	}, nil); err != nil {
		s.Fatal("Failed waiting to find the share target: ", err)
	}

	if err := settingsConn.Call(ctx, nil, `(observer) => nearby_share.getReceiveManager().accept(observer.shareTarget.id)`, &observer); err != nil {
		s.Fatal("Failed to accept the incoming share: ", err)
	}

	f, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Files app: ", err)
	}
	defer f.Release(ctx)

	if err := f.OpenDownloads(ctx); err != nil {
		s.Fatal("Failed to open downloads folder: ", err)
	}

	if err := f.WaitForFile(ctx, filename, 10*time.Second); err != nil {
		s.Fatal("Failed to find the received file: ", err)
	}
}
