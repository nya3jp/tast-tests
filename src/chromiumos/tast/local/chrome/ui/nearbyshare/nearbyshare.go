// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbyshare is for controlling Nearby Share through the UI.
package nearbyshare

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/quicksettings"
)

const uiTimeout = 10 * time.Second

// ReceiveUI represents the receiving UI that is shown during high-visibility mode.
type ReceiveUI struct {
	Root *ui.Node
}

// EnterHighVisibility enables high-visibility mode from Quick Settings and returns the receiving UI surface.
func EnterHighVisibility(ctx context.Context, tconn *chrome.TestConn) (*ReceiveUI, error) {
	// TODO(crbug/1099502): replace this with quicksettings.Show when retry is no longer needed.
	if err := quicksettings.ShowWithRetry(ctx, tconn, uiTimeout); err != nil {
		return nil, errors.Wrap(err, "failed to open Quick Settings")
	}
	defer quicksettings.Hide(ctx, tconn)

	if err := quicksettings.ToggleSetting(ctx, tconn, quicksettings.SettingPodNearbyShare, true); err != nil {
		return nil, errors.Wrap(err, "failed to enter Nearby Share high-visibility mode")
	}

	receiveWindow, err := ui.FindWithTimeout(ctx, tconn, receiveUIParams, uiTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Nearby Share receiving window")
	}

	return &ReceiveUI{Root: receiveWindow}, nil
}

// Release releases the root node held by the receiving UI.
func (r *ReceiveUI) Release(ctx context.Context) {
	r.Root.Release(ctx)
}

// Accept accepts the incoming share in the receiving UI.
func (r *ReceiveUI) Accept(ctx context.Context, tconn *chrome.TestConn) error {
	confirm, err := r.Root.DescendantWithTimeout(ctx, confirmBtnParams, uiTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find confirm button")
	}
	defer confirm.Release(ctx)
	if err := confirm.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click confirm button to receive the share")
	}
	return nil
}

// WaitForShare waits until an incoming share appears in the receiving UI.
func (r *ReceiveUI) WaitForShare(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) error {
	return r.Root.WaitUntilDescendantExists(ctx, incomingShareParams, timeout)
}

// WaitForReceiptNotification waits for the notification indicating that the transfer is complete.
func WaitForReceiptNotification(ctx context.Context, tconn *chrome.TestConn, content SharingContentType, timeout time.Duration) error {
	if _, err := ash.WaitForNotification(ctx, tconn, 10*time.Second,
		ash.WaitTitleContains(fmt.Sprintf("%v received", content)),
	); err != nil {
		return errors.Wrap(err, "failed waiting for receipt confirmation notification")
	}
	return nil
}

// ReceivedFollowUp clicks the button for the suggested follow-up action in the receipt notification.
// This returns immediately after clicking the button, so the caller should take care to wait for the follow-up to actually occur.
func ReceivedFollowUp(ctx context.Context, tconn *chrome.TestConn, content SharingContentType) error {
	followupText := ReceivedFollowUpMap[content]
	followupBtn, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "NotificationMdTextButton", Name: followupText}, uiTimeout)
	if err != nil {
		return errors.Wrapf(err, "failed to find %v button in the receipt notification", followupText)
	}
	defer followupBtn.Release(ctx)

	if err := followupBtn.LeftClick(ctx); err != nil {
		return errors.Wrapf(err, "failed to click %v button", followupText)
	}

	return nil
}
