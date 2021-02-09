// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package systemproxy contains utility functions to authenticate to the system-proxy daemon.
package systemproxy

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// RunArcConnectivityApp tests ARC++ app connectivity through the system-proxy Chrome OS daemon using the following steps:
// - installs a test ARC++ app;
// - connects to `url` in the app;
// - clicks on the system-proxy notification which informs the user that system-proxy requires credentials;
// - enters proxy credentials in the system-proxy dialog;
// - reads the network request's HTTP response code.
// In case of success it returns the HTTP response code and the global proxy. If the app fails to connect
// to the url, it will return the network error along with the global proxy. If the test fails, it returns an error.
func RunArcConnectivityApp(ctx context.Context, a *arc.ARC, tconn *chrome.TestConn, url, username, password string) (string, string, error) {
	const (
		pkg      = "org.chromium.arc.testapp.connectivity"
		activity = ".ConnectivityActivity"
		// UI IDs in the app.
		idPrefix      = pkg + ":id/"
		urlID         = idPrefix + "url"
		proxyID       = idPrefix + "global_proxy"
		fetchButtonID = idPrefix + "network_request_button"
		waitButtonID  = idPrefix + "await_result_button"
		resultID      = idPrefix + "result"
	)
	testing.ContextLog(ctx, "Installing ARC test app")
	if err := a.Install(ctx, arc.APKPath("ArcConnectivityTest.apk")); err != nil {
		return "", "", errors.Wrap(err, "failed to install app")
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return "", "", errors.Wrap(err, "failed initializing UI Automator")
	}
	defer d.Close(ctx)

	act, err := arc.NewActivity(a, pkg, pkg+activity)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to create a new activity")
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		return "", "", errors.Wrap(err, "failed to start activity")
	}
	defer act.Stop(ctx, tconn)

	// Get the global proxy.
	field := d.Object(ui.ID(proxyID))
	if err := field.WaitForExists(ctx, 30*time.Second); err != nil {
		return "", "", errors.Wrap(err, "failed to find field")
	}

	proxy, err := field.GetText(ctx)
	if err != nil {
		return "", proxy, errors.Wrap(err, "failed to read global proxy value")
	}

	field = d.Object(ui.ID(urlID))
	if err := field.WaitForExists(ctx, 30*time.Second); err != nil {
		return "", proxy, errors.Wrap(err, "failed to find field")
	}

	if err := field.SetText(ctx, url); err != nil {
		return "", proxy, errors.Wrap(err, "failed to set url")
	}
	// Do network request.
	field = d.Object(ui.ID(fetchButtonID))
	if err := field.WaitForExists(ctx, 30*time.Second); err != nil {
		return "", proxy, errors.Wrap(err, "failed to find field")
	}

	if err := field.Click(ctx); err != nil {
		return "", proxy, errors.Wrap(err, "failed to click field")
	}

	// Wait for the system-proxy daemon to ask for proxy credentials and authenticate in the system dialog.
	if err := DoSystemProxyAuthentication(ctx, tconn, username, password); err != nil {
		return "", proxy, errors.Wrap(err, "system-proxy authentication failed")
	}

	field = d.Object(ui.ID(waitButtonID))
	if err := field.WaitForExists(ctx, 30*time.Second); err != nil {
		return "", proxy, errors.Wrap(err, "failed to find field")
	}
	// Wait for the network request result.
	if err := field.Click(ctx); err != nil {
		return "", proxy, errors.Wrap(err, "failed to click field")
	}

	field = d.Object(ui.ID(resultID))
	if err := field.WaitForExists(ctx, 30*time.Second); err != nil {
		return "", proxy, errors.Wrap(err, "failed to find field")
	}

	result, err := field.GetText(ctx)
	if err != nil {
		return "", proxy, errors.Wrap(err, "failed to get result")
	}

	return result, proxy, nil
}

// DoSystemProxyAuthentication authenticates to system-proxy with `username` and `password`. If system-proxy is not asking for
// credentials or in case of failure, returns an error.
func DoSystemProxyAuthentication(ctx context.Context, tconn *chrome.TestConn, username, password string) error {
	const (
		notificationTitle = "Sign in"
		uiTimeout         = 20 * time.Second
	)
	pollOpts := testing.PollOptions{Interval: 2 * time.Second, Timeout: uiTimeout}

	_, err := ash.WaitForNotification(ctx, tconn, uiTimeout, ash.WaitTitle(notificationTitle))
	if err != nil {
		return errors.Wrapf(err, "failed waiting %v for system-proxy notification", uiTimeout)
	}

	if err := chromeui.StableFindAndClick(ctx, tconn, chromeui.FindParams{
		Name: notificationTitle,
		Role: chromeui.RoleTypeStaticText,
	}, &pollOpts); err != nil {
		return errors.Wrap(err, "failed finding notification and clicking it")
	}

	// Introduce Credentials in the system-proxy dialog.
	dialog, err := chromeui.StableFind(ctx, tconn, chromeui.FindParams{ClassName: "DialogDelegateView"}, &pollOpts)
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
	}
	if err := kb.Type(ctx, password); err != nil {
		return errors.Wrap(err, "failed to type password")
	}

	okButton, err := dialog.DescendantWithTimeout(ctx, chromeui.FindParams{Role: chromeui.RoleTypeButton, Name: "Sign in"}, uiTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find field")
	}
	defer okButton.Release(ctx)

	if err := okButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click the sign in button")
	}
	return nil
}
