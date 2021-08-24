// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
)

// OldVerifyNotExists checks if the element does not appear during timeout.
// The function first waits until the element disappears.
// Note: this waits for the full timeout to check that the element does not appear.
func OldVerifyNotExists(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams, timeout time.Duration) error {
	start := time.Now()

	// Wait for element to disappear.
	if err := ui.WaitUntilGone(ctx, tconn, params, timeout); err != nil {
		return err
	}

	// Continue waiting for timeout.
	var after = time.After(timeout - time.Since(start))

	// Wait for the full timeout to see if the element shows up.
	// Check periodically if it shows up.
	for {
		if exists, err := ui.Exists(ctx, tconn, params); err != nil {
			return err
		} else if exists {
			return ui.ErrNodeExists
		}

		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		case <-after:
			// Node did not show up.
			return nil
		}
	}
}

// VerifyNotExists is OldVerifyNotExists with its ui dependency removed.
func VerifyNotExists(ctx context.Context, tconn *chrome.TestConn, finder *nodewith.Finder, timeout time.Duration) error {
	start := time.Now()

	// Wait for element to disappear.
	ui := uiauto.New(tconn)
	if err := ui.WithTimeout(timeout).WaitUntilGone(finder)(ctx); err != nil {
		return err
	}

	// Continue waiting for timeout.
	var after = time.After(timeout - time.Since(start))

	// Wait for the full timeout to see if the element shows up.
	// Check periodically if it shows up.
	for {
		if exists, err := ui.IsNodeFound(ctx, finder); err != nil {
			return err
		} else if exists {
			return errors.New("node still exists")
		}

		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		case <-after:
			// Node did not show up.
			return nil
		}
	}
}

// WaitUntilExistsStatus repeatedly checks the existence of a node
// until the desired status is found or the timeout is reached.
// If the JavaScript fails to execute, an error is returned.
func WaitUntilExistsStatus(ctx context.Context, tconn *chrome.TestConn, finder *nodewith.Finder, exists bool, timeout time.Duration) error {
	ui := uiauto.New(tconn)
	if exists {
		return ui.WithTimeout(timeout).WaitUntilExists(finder)(ctx)
	}

	return ui.WithTimeout(timeout).WaitUntilGone(finder)(ctx)
}

// VerifyNodeState repeatedly checks the existence of a node to make sure it
// reaches the desired status and stays in that state.
// Fully waits until the timeout expires to ensure non-existence.
// If the JavaScript fails to execute, an error is returned.
func VerifyNodeState(ctx context.Context, tconn *chrome.TestConn, finder *nodewith.Finder, exists bool, timeout time.Duration) error {
	if exists {
		ui := uiauto.New(tconn)
		return ui.WithTimeout(timeout).WaitUntilExists(finder)(ctx)
	}

	return VerifyNotExists(ctx, tconn, finder, timeout)
}
