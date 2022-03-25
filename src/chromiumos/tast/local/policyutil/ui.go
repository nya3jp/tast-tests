// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
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

// EnsureCookiesAccepted ensures that cookies page for the given url is accepted and gone.
// It will open the url and click on the button with the given ID acceptBtnLocator.
func EnsureCookiesAccepted(ctx context.Context, br *browser.Browser, url, acceptBtnLocator string) error {
	conn, err := br.NewConn(ctx, url)
	if err != nil {
		return errors.Wrap(err, "failed to open the browser")
	}
	defer conn.Close()

	var acceptButtonFound bool
	acceptBtnCheck := fmt.Sprintf("%v != null", acceptBtnLocator)
	if err := conn.Eval(ctx, acceptBtnCheck, &acceptButtonFound); err != nil {
		return errors.Wrapf(err, "failed to find cookies element by locator %q", acceptBtnLocator)
	}
	if acceptButtonFound {
		testing.ContextLog(ctx, "Found cookies page")
		clickAccept := fmt.Sprintf("%v.click()", acceptBtnLocator)
		if err := conn.Eval(ctx, clickAccept, nil); err != nil {
			return errors.Wrap(err, "failed to click on accept button")
		}
	}
	conn.CloseTarget(ctx)
	return nil
}

// EnsureGoogleCookiesAccepted ensures that google related cookies are accepted (i.e. search, translate and extensions).
// It will open google page then click on Accept button.
func EnsureGoogleCookiesAccepted(ctx context.Context, br *browser.Browser) error {
	url := "https://www.google.com/?hl=en"
	acceptButtonID := "L2AGLb"
	acceptBtnLocator := fmt.Sprintf("document.getElementById(%q)", acceptButtonID)
	return EnsureCookiesAccepted(ctx, br, url, acceptBtnLocator)
}

// MaximizeActiveWindow maximizes the browser active window.
func MaximizeActiveWindow(ctx context.Context, tconn *chrome.TestConn) error {
	// Get the current active window.
	activeWindow, err := ash.GetActiveWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the current active window")
	}
	winID := activeWindow.ID

	// Turn the window into maximized state.
	if err := ash.SetWindowStateAndWait(ctx, tconn, winID, ash.WindowStateMaximized); err != nil {
		return errors.Wrap(err, "failed to set the window state to maximized")
	}
	return nil
}
