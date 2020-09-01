// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testutil provides utilities to setup testing environment for camera
// tests.
package testutil

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
)

var (
	// ID is the app id of CCA.
	ID = "hfhhnacclhffhdffklopdkcgdhifgngh"
	// BackgroundURL is the url of the CCA background page.
	BackgroundURL = fmt.Sprintf("chrome-extension://%s/views/background.html", ID)
	// platformAppJSPath is the js path for Platform app version of CCA.
	platformAppJSPath = fmt.Sprintf("chrome-extension://%s/js", ID)
	// systemWebAppJSPath is the js path for SWA version of CCA.
	systemWebAppJSPath = "chrome://camera-app/src/js"
)

// AppLauncher is used during the launch process of CCA. We could launch CCA
// by launchApp event, camera intent or any other ways.
type AppLauncher func(ctx context.Context, tconn *chrome.TestConn) error

// JSPath returns the path for JS files according to |isSWA|.
func JSPath(isSWA bool) string {
	if isSWA {
		return systemWebAppJSPath
	}
	return platformAppJSPath
}

// LaunchApp launches the camera app and handle the communication flow between tests and app.
func LaunchApp(ctx context.Context, cr *chrome.Chrome, tb *TestBridge, appLauncher AppLauncher) (*chrome.Conn, *AppWindow, error) {
	appWindow, err := tb.AppWindow(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to register app window")
	}

	conn, err := func() (*chrome.Conn, error) {
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return nil, err
		}
		if err := appLauncher(ctx, tconn); err != nil {
			return nil, err
		}

		windowURL, err := appWindow.WaitUntilWindowBound(ctx)
		if err != nil {
			return nil, err
		}

		var conn *chrome.Conn
		if conn, err = cr.NewConnForTarget(ctx, chrome.MatchTargetURL(windowURL)); err != nil {
			return nil, err
		}
		conn.StartProfiling(ctx)
		if err := appWindow.NotifyReady(ctx); err != nil {
			return nil, err
		}
		return conn, nil
	}()
	if err != nil {
		appWindow.Release(ctx)
		return nil, nil, err
	}
	return conn, appWindow, nil
}

// LaunchSWA launches CCA as SWA.
func LaunchSWA(ctx context.Context, tconn *chrome.TestConn) error {
	return apps.LaunchSystemWebApp(ctx, tconn, "Camera",
		"chrome://camera-app/src/views/main.html")
}

// LaunchPlatformApp launches CCA as platform app.
func LaunchPlatformApp(ctx context.Context, tconn *chrome.TestConn) error {
	return tconn.Call(ctx, nil, `tast.promisify(chrome.management.launchApp)`, ID)
}
