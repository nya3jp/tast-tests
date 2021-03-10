// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbyshare is used to control Nearby Share functionality.
package nearbyshare

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/testing"
)

// StartHighVisibilityMode enables Nearby Share's high visibility mode via Quick Settings.
func StartHighVisibilityMode(ctx context.Context, tconn *chrome.TestConn, deviceName string) error {
	if err := quicksettings.Show(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to open Quick Settings")
	}
	defer quicksettings.Hide(ctx, tconn)

	if err := quicksettings.ToggleSetting(ctx, tconn, quicksettings.SettingPodNearbyShare, true); err != nil {
		return errors.Wrap(err, "failed to enter Nearby Share high-visibility mode")
	}

	receiveWindow, err := ui.FindWithTimeout(ctx, tconn, ReceiveUIParams, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find Nearby Share receiving window")
	}
	defer receiveWindow.Release(ctx)

	// Check for the text in the dialog that shows the displayed device name and that we're visible to nearby devices.
	// This text includes a countdown for remaining high-visibility time that changes dynamically, so we'll match a substring.
	r, err := regexp.Compile(fmt.Sprintf("Visible to nearby devices as %v", deviceName))
	if err != nil {
		return errors.Wrap(err, "failed to compile regexp for visibility and device name text")
	}
	textParams := ui.FindParams{
		Role:       ui.RoleTypeStaticText,
		Attributes: map[string]interface{}{"name": r},
	}
	if err := receiveWindow.WaitUntilDescendantExists(ctx, textParams, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to find text with device name and visibility indication")
	}
	return nil
}

// AcceptIncomingShareNotification waits for the incoming share notification from an in-contacts device and then accepts the share.
func AcceptIncomingShareNotification(ctx context.Context, tconn *chrome.TestConn, senderName string, timeout time.Duration) error {
	if _, err := ash.WaitForNotification(ctx, tconn, timeout,
		ash.WaitTitleContains("Nearby Share"),
		ash.WaitMessageContains(senderName),
	); err != nil {
		return errors.Wrap(err, "failed to wait for incoming share notification")
	}
	if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{Name: "RECEIVE", ClassName: "NotificationMdTextButton"}, nil); err != nil {
		return errors.Wrap(err, "failed to click sharing notification's receive button")
	}
	return nil
}

// WaitForReceivingCompleteNotification waits for the notification indicating that the incoming share has completed.
func WaitForReceivingCompleteNotification(ctx context.Context, tconn *chrome.TestConn, senderName string, timeout time.Duration) error {
	// if _, err := ash.WaitForNotification(ctx, tconn, timeout,
	// 	ash.WaitTitleContains("received"),
	// 	ash.WaitTitleContains(senderName),
	// ); err != nil {
	// 	return errors.Wrap(err, "failed to wait for receiving complete notification")
	// }
	// return nil
	n, err := ash.WaitForNotification(ctx, tconn, timeout,
		ash.WaitTitleContains("received"),
		ash.WaitTitleContains(senderName),
	)
	if err != nil {
		return errors.Wrap(err, "failed to wait for receiving complete notification")
	}
	testing.ContextLog(ctx, "################ NOTIFICATION TITLE: ", n.Title)
	testing.ContextLog(ctx, "################ NOTIFICATION MESSAGE: ", n.Message)
	testing.ContextLog(ctx, "################ NOTIFICATION PROGRESS: ", n.Progress)
	return nil
}
