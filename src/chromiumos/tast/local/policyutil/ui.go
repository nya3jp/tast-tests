// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// VerifyNotExists checks if the element does not appear during timeout.
// The function first waits until the element disappears.
// Note: this waits for the full timeout to check that the element does not appear.
func VerifyNotExists(ctx context.Context, tconn *chrome.TestConn, finder *nodewith.Finder, timeout time.Duration) error {
	start := time.Now()
	// Wait for element to disappear.
	ui := uiauto.New(tconn)
	if err := uiauto.Combine("node still exists",
		ui.WithTimeout(timeout).WaitUntilGone(finder),
		ui.EnsureGoneFor(finder, timeout-time.Since(start)),
	)(ctx); err != nil {
		return err
	}
	return nil
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

// EnsureMaximized will ensure that the browser window is maximized.
func EnsureMaximized(ctx context.Context, tconn *chrome.TestConn) error {
	uia := uiauto.New(tconn)
	frameCaptionButton := nodewith.Role(role.Button).ClassName("FrameCaptionButton").Nth(1)
	maximizeButton := nodewith.Role(role.Button).Name("Maximize").ClassName("FrameCaptionButton")
	restoreButton := nodewith.Role(role.Button).Name("Restore").ClassName("FrameCaptionButton")
	if err := uia.WaitUntilExists(frameCaptionButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for frame caption button button")
	}
	midButton, err := uia.Info(ctx, frameCaptionButton)
	if err != nil {
		return errors.Wrap(err, "failed to find frame caption button button")
	}
	if midButton.Name == "Maximize" {
		if err := uiauto.Combine("Maximize the browser window",
			uia.LeftClick(maximizeButton),
			uia.WaitUntilExists(restoreButton),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to click Maximize button")
		}
	}
	return nil
}
