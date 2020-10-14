// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/ui/nearbyshare"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: NearbyShare,
		Desc: "Checks that settings can be opened from Quick Settings",
		Contacts: []string{
			"kyleshima@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// NearbyShare tests that we can receive a file from an out-of-contacts Android device in high-visibility mode.
func NearbyShare(ctx context.Context, s *testing.State) {
	const (
		// Delay duration for manually sharing from the Android phone.
		manualDelay = 60 * time.Second

		// Timeout for completing the share.
		sharingTimeout = 60 * time.Second

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

	receiveUI, err := nearbyshare.EnterHighVisibility(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to enter high-visibility mode: ", err)
	}
	defer receiveUI.Release(ctx)

	s.Log("Share from the Android phone now")

	if err := receiveUI.WaitForShare(ctx, tconn, manualDelay); err != nil {
		s.Fatal("No incoming share detected: ", err)
	}

	// Add your own device name to check if it is displayed in the receiving UI.
	senderName := "Kyle's Phone"
	if err := receiveUI.Root.WaitUntilDescendantExists(ctx, ui.FindParams{Role: ui.RoleTypeStaticText, Name: senderName}, 10*time.Second); err != nil {
		s.Errorf("Sender name %v not found: %v", senderName, err)
	}

	// Accept the share.
	if err := receiveUI.Accept(ctx, tconn); err != nil {
		s.Fatal("Failed to accept the share: ", err)
	}

	// Check that file was received by checking the notification.
	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to open quick settings: ", err)
	}
	if err := nearbyshare.WaitForReceiptNotification(ctx, tconn, nearbyshare.SharingContentFile, sharingTimeout); err != nil {
		s.Fatal("Failed waiting for sharing receipt notification: ", err)
	}

	// View the file in the Files app.
	if err := nearbyshare.ReceivedFollowUp(ctx, tconn, nearbyshare.SharingContentFile); err != nil {
		s.Fatal("Failed to click the follow-up action in the receipt notification: ", err)
	}

	filesApp, err := filesapp.WaitForLaunched(ctx, tconn)
	if err != nil {
		s.Fatal("Failed waiting for Files app to launch: ", err)
	}
	defer filesApp.Release(ctx)

	if err := filesApp.WaitForFile(ctx, filename, 10*time.Second); err != nil {
		s.Fatal("Failed to find the received file: ", err)
	}
}
