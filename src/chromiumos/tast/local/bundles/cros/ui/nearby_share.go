// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strconv"
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
		Vars:         []string{"sharingTimeout", "manualDelay", "senderName", "filename"},
	})
}

// NearbyShare tests that we can receive a file from an out-of-contacts Android device in high-visibility mode.
func NearbyShare(ctx context.Context, s *testing.State) {
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

	var manualDelay time.Duration
	if val, ok := s.Var("manualDelay"); ok {
		seconds, err := strconv.Atoi(val)
		if err != nil {
			s.Fatalf("Failed to convert manualDelay argument (%v) to integer: %v", val, err)
		}
		manualDelay = time.Duration(seconds) * time.Second
	} else {
		manualDelay = 60 * time.Second
	}

	s.Logf("Share from the Android phone within %v", manualDelay)
	if err := receiveUI.WaitForShare(ctx, tconn, manualDelay); err != nil {
		s.Fatal("No incoming share detected: ", err)
	}

	// Add your own device name to check if it is displayed in the receiving UI.
	senderName, ok := s.Var("senderName")
	if !ok {
		s.Fatal("Need to provide a senderName argument to 'tast run' command")
	}
	s.Logf("Looking for shares from %v", senderName)
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

	var sharingTimeout time.Duration
	if val, ok := s.Var("sharingTimeout"); ok {
		seconds, err := strconv.Atoi(val)
		if err != nil {
			s.Fatalf("Failed to convert sharingTimeout argument (%v) to integer: %v", val, err)
		}
		sharingTimeout = time.Duration(seconds) * time.Second
	} else {
		sharingTimeout = 60 * time.Second
	}
	s.Logf("Waiting up to %v to receive the share", sharingTimeout)
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

	filename, ok := s.Var("filename")
	if !ok {
		s.Fatal("Need to provide a filename argument to 'tast run' command")
	}
	s.Logf("Looking for file %v", filename)
	if err := filesApp.WaitForFile(ctx, filename, 10*time.Second); err != nil {
		s.Fatal("Failed to find the received file: ", err)
	}
}
