// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbyshare is used to control Nearby Share functionality.
package nearbyshare

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
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
	if err := quicksettings.Show(ctx, tconn); err != nil {
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
