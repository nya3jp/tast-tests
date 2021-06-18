// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package systemproxy contains utility functions to authenticate to the system-proxy daemon.
package systemproxy

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// DoSystemProxyAuthentication authenticates to system-proxy with `username` and `password` by clicking on the system-proxy notification
// which informs the user that system-proxy requires credentials and entering the proxy credentials in the system-proxy dialog.
// If system-proxy is not asking for credentials or in case of failure, returns an error.
func DoSystemProxyAuthentication(ctx context.Context, tconn *chrome.TestConn, username, password string) error {
	const (
		notificationTitle = "Sign in"
		uiTimeout         = 10 * time.Second
	)

	if err := quicksettings.Show(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to show system tray")
	}

	if _, err := ash.WaitForNotification(ctx, tconn, uiTimeout, ash.WaitTitle(notificationTitle)); err != nil {
		return errors.Wrapf(err, "failed waiting %v for system-proxy notification", uiTimeout)
	}

	pollOpts := testing.PollOptions{Interval: 2 * time.Second, Timeout: uiTimeout}
	if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
		Name: notificationTitle,
		Role: ui.RoleTypeStaticText,
	}, &pollOpts); err != nil {
		return errors.Wrap(err, "failed finding notification and clicking it")
	}

	// Introduce Credentials in the system-proxy dialog.
	dialog, err := ui.StableFind(ctx, tconn, ui.FindParams{ClassName: "RequestSystemProxyCredentialsView"}, &pollOpts)
	if err != nil {
		return errors.Wrap(err, "failed to find system-proxy dialog")
	}
	defer dialog.Release(ctx)

	// Set up keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	if err := kb.Type(ctx, username); err != nil {
		return errors.Wrap(err, "failed to type username")
	}
	// Move focus to password text field.
	if err := kb.Accel(ctx, "Tab"); err != nil {
		return errors.Wrap(err, "failed to navigate via tab to the password text field")
	}
	if err := kb.Type(ctx, password); err != nil {
		return errors.Wrap(err, "failed to type password")
	}

	okButton, err := dialog.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeButton, Name: "Sign in"}, uiTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find field")
	}
	defer okButton.Release(ctx)

	if err := okButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click the sign in button")
	}
	return nil
}
