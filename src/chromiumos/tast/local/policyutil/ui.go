// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
)

// VerifyNotExists checks if the element does not appear during timeout.
// The function first waits until the element disappears.
// Note: this waits for the full timeout to check that the element does not appear.
func VerifyNotExists(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams, timeout time.Duration) error {
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

// WaitUntilExistsStatus repeatedly checks the existence of a node
// until the desired status is found or the timeout is reached.
// If the JavaScript fails to execute, an error is returned.
func WaitUntilExistsStatus(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams, exists bool, timeout time.Duration) error {
	if exists {
		return ui.WaitUntilExists(ctx, tconn, params, timeout)
	}

	return ui.WaitUntilGone(ctx, tconn, params, timeout)
}

// VerifyNodeState repeatedly checks the existence of a node to make sure it
// reaches the desired status and stays in that state.
// Fully waits until the timeout expires to ensure non-existence.
// If the JavaScript fails to execute, an error is returned.
func VerifyNodeState(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams, exists bool, timeout time.Duration) error {
	if exists {
		return ui.WaitUntilExists(ctx, tconn, params, timeout)
	}

	return VerifyNotExists(ctx, tconn, params, timeout)
}
