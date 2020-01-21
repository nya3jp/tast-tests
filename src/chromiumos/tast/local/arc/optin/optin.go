// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package optin provides set of util functions used to control ARC provisioning.
package optin

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// playStoreAppID is app id of Play Store app.
const playStoreAppID = "cnbgggchhmkkdmeppjobngjoejnihlei"

// SetPlayStoreEnabled is a wrapper for chrome.autotestPrivate.setPlayStoreEnabled.
func SetPlayStoreEnabled(ctx context.Context, tconn *chrome.Conn, enabled bool) error {
	expr := fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.setPlayStoreEnabled)(%t)`, enabled)
	return tconn.EvalPromise(ctx, expr, nil)
}

// Perform steps through opt-in flow and wait for it to complete.
func Perform(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn) error {
	SetPlayStoreEnabled(ctx, tconn, true)

	bgURL := chrome.ExtensionBackgroundPageURL(playStoreAppID)
	conn, err := cr.NewConnForTarget(ctx, func(t *chrome.Target) bool {
		return t.URL == bgURL
	})
	if err != nil {
		return errors.Wrapf(err, "failed to find %v", bgURL)
	}
	defer conn.Close()

	for _, condition := range []string{
		"port != null",
		"termsPage != null",
		"termsPage.isManaged_ || termsPage.state_ == LoadState.LOADED",
	} {
		if err := conn.WaitForExpr(ctx, condition); err != nil {
			return errors.Wrapf(err, "failed to wait for %v", condition)
		}
	}

	if err := conn.Exec(ctx, "termsPage.onAgree()"); err != nil {
		return errors.Wrap(err, "failed to execute 'termsPage.onAgree()'")
	}

	if err := conn.WaitForExpr(ctx, "!appWindow"); err != nil {
		return errors.Wrap(err, "failed to wait for '!appWindow'")
	}

	// TODO(niwa): Check if we still need to handle non-tos_needed case.
	return nil
}

// WaitForPlayStoreShown waits for Play Store window to be shown.
func WaitForPlayStoreShown(ctx context.Context, tconn *chrome.Conn) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var appShown bool
		expr := fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.isAppShown)('%s')`, playStoreAppID)
		if err := tconn.EvalPromise(ctx, expr, &appShown); err != nil {
			return testing.PollBreak(err)
		}
		if !appShown {
			return errors.New("Play Store is not shown yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for Play Store window to be shown")
	}
	return nil
}
