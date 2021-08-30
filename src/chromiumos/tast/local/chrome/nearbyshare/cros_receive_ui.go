// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbyshare is used to control Nearby Share functionality.
package nearbyshare

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
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

	ui := uiauto.New(tconn).WithTimeout(10 * time.Second)
	receiveWindow := nodewith.Role(role.RootWebArea).Name("Settings - Nearby Share")
	if err := ui.WaitUntilExists(receiveWindow)(ctx); err != nil {
		return errors.Wrap(err, "failed to find Nearby Share receiving window")
	}

	// Check for the text in the dialog that shows the displayed device name and that we're visible to nearby devices.
	// This text includes a countdown for remaining high-visibility time that changes dynamically, so we'll match a substring.
	r, err := regexp.Compile(fmt.Sprintf("Visible to nearby devices as %v", deviceName))
	if err != nil {
		return errors.Wrap(err, "failed to compile regexp for visibility and device name text")
	}
	visibleText := nodewith.NameRegex(r).Role(role.StaticText).Ancestor(receiveWindow)
	if err := ui.WaitUntilExists(visibleText)(ctx); err != nil {
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
	// TODO(crbug/1201855): Get rid of the regex and just use "ACCEPT" once we don't have to worry about version skew for CrOS<->CrOS sharing.
	// Since we don't have multi-DUT support in the lab yet, sender and receiver devices are often running different OS versions, and thus have different strings.
	r := regexp.MustCompile("(ACCEPT|RECEIVE)")
	ui := uiauto.New(tconn)
	btn := nodewith.ClassName("NotificationTextButton").NameRegex(r)
	if err := ui.LeftClick(btn)(ctx); err != nil {
		return errors.Wrap(err, "failed to click sharing notification's receive button")
	}
	return nil
}

// IncomingShareNotificationExists checks if the incoming share notification is present.
func IncomingShareNotificationExists(ctx context.Context, tconn *chrome.TestConn, senderName string) (bool, error) {
	notifications, err := ash.Notifications(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "failed to get notifications")
	}
	for _, n := range notifications {
		if strings.Contains(n.Title, "Nearby Share") && strings.Contains(n.Message, senderName) {
			return true, nil
		}
	}
	return false, nil
}

// WaitForReceivingCompleteNotification waits for the notification indicating that the incoming share has completed.
func WaitForReceivingCompleteNotification(ctx context.Context, tconn *chrome.TestConn, senderName string, timeout time.Duration) error {
	if _, err := ash.WaitForNotification(ctx, tconn, timeout,
		ash.WaitTitleContains("received"),
		ash.WaitTitleContains(senderName),
	); err != nil {
		return errors.Wrap(err, "failed to wait for receiving complete notification")
	}
	return nil
}
