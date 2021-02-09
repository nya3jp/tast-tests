// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	connPkg = "org.chromium.arc.testapp.connectivity"
	// UI IDs in the app.
	connIDPrefix = connPkg + ":id/"
)

// RunArcConnectivityApp tests ARC++ app connectivity by installing a test ARC++ app and connecting to a URL in the app.
// Returns the default Android global proxy and an Activity associated with the app which should be closed by calling `GetArcConnectivityAppResult`.
// Callers must invoke `GetArcConnectivityAppResult` to read the network response or error.
// Note: if you require additional network configurations info for your test, please add the info to the ArcConnectivityTest app.
func RunArcConnectivityApp(ctx context.Context, a *arc.ARC, tconn *chrome.TestConn, url string) (proxy string, act *arc.Activity, err error) {
	const (
		activity      = ".ConnectivityActivity"
		urlID         = connIDPrefix + "url"
		proxyID       = connIDPrefix + "global_proxy"
		fetchButtonID = connIDPrefix + "network_request_button"
	)
	testing.ContextLog(ctx, "Installing ARC test app")
	err = a.Install(ctx, arc.APKPath("ArcConnectivityTest.apk"))
	if err != nil {
		err = errors.Wrap(err, "failed to install app")
		return
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		err = errors.Wrap(err, "failed initializing UI Automator")
		return
	}
	defer d.Close(ctx)

	act, err = arc.NewActivity(a, connPkg, connPkg+activity)
	if err != nil {
		err = errors.Wrap(err, "failed to create a new activity")
		return
	}
	defer func() {
		if err != nil {
			act.Close()
		}
	}()

	err = act.Start(ctx, tconn)
	if err != nil {
		err = errors.Wrap(err, "failed to start activity")
		return
	}

	defer func() {
		if err != nil {
			act.Stop(ctx, tconn)
		}
	}()
	// Read the global proxy which is displayed on the test app's UI.
	field := d.Object(ui.ID(proxyID))
	err = field.WaitForExists(ctx, 30*time.Second)
	if err != nil {
		err = errors.Wrap(err, "failed to find field")
		return
	}

	proxy, err = field.GetText(ctx)
	if err != nil {
		err = errors.Wrap(err, "failed to read global proxy value")
		return
	}

	field = d.Object(ui.ID(urlID))
	err = field.WaitForExists(ctx, 30*time.Second)
	if err != nil {
		err = errors.Wrap(err, "failed to find field")
		return
	}

	err = field.SetText(ctx, url)
	if err != nil {
		err = errors.Wrap(err, "failed to set url")
		return
	}
	// Do network request.
	field = d.Object(ui.ID(fetchButtonID))
	err = field.WaitForExists(ctx, 30*time.Second)
	if err != nil {
		err = errors.Wrap(err, "failed to find field")
		return
	}

	err = field.Click(ctx)
	if err != nil {
		err = errors.Wrap(err, "failed to click field")
		return
	}

	return
}

// GetArcConnectivityAppResult wait for the network request started by calling `RunArcConnectivityApp` to finish,
// reads HTTP response code and stops the app. In case of success it returns the response code. If the app fails to connect
// to the url, it will return the network error message. If the test fails, it returns an error.
func GetArcConnectivityAppResult(ctx context.Context, a *arc.ARC, tconn *chrome.TestConn, act *arc.Activity) (string, error) {
	const (
		waitButtonID = connIDPrefix + "await_result_button"
		resultID     = connIDPrefix + "result"
	)
	defer act.Close()
	defer act.Stop(ctx, tconn)
	d, err := a.NewUIDevice(ctx)

	field := d.Object(ui.ID(waitButtonID))
	if err := field.WaitForExists(ctx, 30*time.Second); err != nil {
		return "", errors.Wrap(err, "failed to find field")
	}
	// Wait for the network request result.
	if err := field.Click(ctx); err != nil {
		return "", errors.Wrap(err, "failed to click field")
	}

	field = d.Object(ui.ID(resultID))
	if err := field.WaitForExists(ctx, 30*time.Second); err != nil {
		return "", errors.Wrap(err, "failed to find field")
	}

	result, err := field.GetText(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get result")
	}

	return result, nil
}
