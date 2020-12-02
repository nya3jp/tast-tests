// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

// Click executes the default action of the first node found with the
// given params. If the node doesn't exist in a second, an error is returned.
func Click(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams) error {
	return WaitAndClick(ctx, tconn, params, time.Second)
}

// WaitAndClick executes the default action of a node found with the
// given params. If the timeout is hit, an error is returned.
func WaitAndClick(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams, timeout time.Duration) error {
	node, err := ui.FindWithTimeout(ctx, tconn, params, timeout)
	if err != nil {
		return err
	}
	defer node.Release(ctx)
	return node.LeftClick(ctx)
}

// ClickDescendant finds the first descendant of the parent node using the params
// and clicks it. If the node doesn't exist in a second, an error is returned.
func ClickDescendant(ctx context.Context, parent *ui.Node, params ui.FindParams) error {
	return WaitAndClickDescendant(ctx, parent, params, time.Second)
}

// WaitAndClickDescendant finds a descendant of the parent node using the params
// and clicks it. If the timeout is hit, an error is returned.
func WaitAndClickDescendant(ctx context.Context, parent *ui.Node, params ui.FindParams, timeout time.Duration) error {
	node, err := parent.DescendantWithTimeout(ctx, params, timeout)
	if err != nil {
		return err
	}
	defer node.Release(ctx)
	return node.LeftClick(ctx)
}

// EnsureShelfVisible sets the shelf behavior to "Never Hide" if it's not shown,
// and returns a function which reverts back to the original state.
func EnsureShelfVisible(ctx context.Context, tconn *chrome.TestConn) (func(context.Context) error, error) {
	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return nil, err
	}

	b, err := ash.GetShelfBehavior(ctx, tconn, info.ID)
	if err != nil {
		return nil, err
	}

	if b == ash.ShelfBehaviorNeverAutoHide {
		return func(c context.Context) error { return nil }, nil
	}

	if err = ash.SetShelfBehavior(ctx, tconn, info.ID, ash.ShelfBehaviorNeverAutoHide); err != nil {
		return nil, err
	}

	testing.Sleep(ctx, time.Second) // wait for the shelf shows up
	return func(c context.Context) error {
		return ash.SetShelfBehavior(ctx, tconn, info.ID, b)
	}, nil
}
