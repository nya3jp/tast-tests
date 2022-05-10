// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/network/systemproxy"
	"chromiumos/tast/testing"
)

// RunArcConnectivityApp tests ARC++ app connectivity through the system-proxy ChromeOS daemon using the following steps:
// - installs a test ARC++ app;
// - connects to `url` in the app;
// - clicks on the system-proxy notification which informs the user that system-proxy requires credentials;
// - enters proxy credentials in the system-proxy dialog;
// - reads the network request's HTTP response code.
// In case of success it returns the global proxy. If the app fails to connect to the url or if the test fails, it returns an error.
// Note: If you require additional network configurations info for your test, please add the info to the ArcConnectivityTest app.
func RunArcConnectivityApp(ctx context.Context, a *arc.ARC, tconn *chrome.TestConn, url string, useSystemProxy bool, username, password string) (string, error) {
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
		errorID       = idPrefix + "error"
	)

	testing.ContextLog(ctx, "Installing ARC test app")
	if err := a.Install(ctx, arc.APKPath("ArcConnectivityTest.apk")); err != nil {
		return "", errors.Wrap(err, "failed to install app")
	}
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed initializing UI Automator")
	}

	defer d.Close(ctx)
	act, err := arc.NewActivity(a, pkg, pkg+activity)
	if err != nil {
		return "", errors.Wrap(err, "failed to create a new activity")
	}
	defer act.Close()
	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		return "", errors.Wrap(err, "failed to start activity")
	}
	defer act.Stop(ctx, tconn)

	// Read the global proxy which is displayed on the test app's UI.
	field := d.Object(ui.ID(proxyID))
	if err := field.WaitForExists(ctx, 30*time.Second); err != nil {
		return "", errors.Wrap(err, "failed to find field")
	}
	proxy, err := field.GetText(ctx)
	if err != nil {
		return proxy, errors.Wrap(err, "failed to read global proxy value")
	}
	field = d.Object(ui.ID(urlID))
	if err := field.WaitForExists(ctx, 30*time.Second); err != nil {
		return proxy, errors.Wrap(err, "failed to find field")
	}
	if err := field.SetText(ctx, url); err != nil {
		return proxy, errors.Wrap(err, "failed to set url")
	}

	// Do network request.
	field = d.Object(ui.ID(fetchButtonID))
	if err := field.WaitForExists(ctx, 30*time.Second); err != nil {
		return proxy, errors.Wrap(err, "failed to find field")
	}
	if err := field.Click(ctx); err != nil {
		return proxy, errors.Wrap(err, "failed to click field")
	}

	if useSystemProxy {
		// Wait for the system-proxy daemon to ask for proxy credentials and authenticate in the system dialog.
		if err := systemproxy.DoSystemProxyAuthentication(ctx, tconn, username, password); err != nil {
			return proxy, errors.Wrap(err, "system-proxy authentication failed")
		}
	}
	field = d.Object(ui.ID(waitButtonID))
	if err := field.WaitForExists(ctx, 30*time.Second); err != nil {
		return proxy, errors.Wrap(err, "failed to find field")
	}
	// Wait for the network request result.
	if err := field.Click(ctx); err != nil {
		return proxy, errors.Wrap(err, "failed to click field")
	}

	const httpOk = "200"
	field = d.Object(ui.ID(resultID))
	if err := field.WaitForText(ctx, httpOk, 30*time.Second); err != nil {
		// If there was a networking error displayed in the app, return it instead of the initial error.
		netErr := func() string {
			field = d.Object(ui.ID(errorID))
			if e := field.WaitForExists(ctx, 30*time.Second); e != nil {
				return ""
			}
			text, _ := field.GetText(ctx)
			return text
		}()

		if netErr != "" {
			// Return the network error from the app.
			return proxy, errors.Wrapf(err, "app reported network error %s", netErr)
		}
		// Return the initial error.
		return proxy, errors.Wrap(err, "fail wait for text")
	}

	return proxy, nil
}
