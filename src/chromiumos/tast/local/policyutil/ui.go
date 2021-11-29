// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
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

// ConsentCookiesIfExists checks if there are cookies consent page, then it clicks on agree button.
func ConsentCookiesIfExists(ctx context.Context, tconn *chrome.TestConn) error {
	uia := uiauto.New(tconn)
	agreeButton := nodewith.Role(role.Button).NameRegex(regexp.MustCompile(`(I agree|Agree to the use of cookies and other data for the purposes described|Ich stimme zu)`)).First()
	cookiesConsent := false
	errCookies := errors.New("cookies consent not found")

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if consent, err := uia.IsNodeFound(ctx, agreeButton); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to check agree button"))
		} else if consent {
			cookiesConsent = true
			return nil
		}

		return errCookies
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil && !strings.HasSuffix(err.Error(), errCookies.Error()) {
		return err
	}

	if cookiesConsent {
		if err := uiauto.Combine("Focus and Left Click on Agree button",
			uia.FocusAndWait(agreeButton),
			uia.LeftClick(agreeButton),
			uia.WaitUntilGone(agreeButton),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to click Agree button")
		}
	}
	return nil
}
