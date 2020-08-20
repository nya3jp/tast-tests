// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

// ClickElement executes the default action of the first node found with the
// given params.
func ClickElement(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams) error {
	node, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return err
	}
	defer node.Release(ctx)
	return node.LeftClick(ctx)
}

// ShelfVisible sets the shelf behavior to "Never Hide" temporary if it's
// not shown. It sets the behavior to original state once the given function
// is done and returns the fn's error.
func ShelfVisible(ctx context.Context, tconn *chrome.TestConn, fn func() error) error {
	if info, err := display.GetInternalInfo(ctx, tconn); err == nil {
		b, err := ash.GetShelfBehavior(ctx, tconn, info.ID)
		if err == nil && b != ash.ShelfBehaviorNeverAutoHide {
			if err = ash.SetShelfBehavior(ctx, tconn, info.ID, ash.ShelfBehaviorNeverAutoHide); err == nil {
				defer ash.SetShelfBehavior(ctx, tconn, info.ID, b)
				testing.Sleep(ctx, time.Second) // wait for the shelf shows up
			}
		}
	}
	return fn()
}
