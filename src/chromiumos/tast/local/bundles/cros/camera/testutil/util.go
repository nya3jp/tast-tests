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
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

var (
	// ID is the app id of CCA.
	ID = "hfhhnacclhffhdffklopdkcgdhifgngh"
	// BackgroundURL is the url of the CCA background page.
	BackgroundURL = fmt.Sprintf("chrome-extension://%s/views/background.html", ID)
	chromeWithSWA = chrome.NewPrecondition("cca_swa", chrome.EnableFeatures("CameraSystemWebApp"))
	arcWithSWA    = arc.NewPrecondition("cca_swa", nil, "--enable-features=CameraSystemWebApp")
)

// CCAAppType determines whether CCA is a platform app or an SWA.
type CCAAppType int

const (
	// SWA represents the System web app version of CCA
	SWA CCAAppType = iota
	// PlatformApp represents the platform app version of CCA
	PlatformApp
)

// ChromeWithSWA returns a precondition that Chrome is already logged in and CCA is installed as an SWA when a test is run.
func ChromeWithSWA() testing.Precondition { return chromeWithSWA }

// ARCWithSWA returns a precondition that ARC Container has already booted and CCA is installed as an SWA when a test is run.
func ARCWithSWA() testing.Precondition { return arcWithSWA }

// AppLauncher is used during the launch process of CCA. We could launch CCA
// by launchApp event, camera intent or any other ways.
type AppLauncher func(ctx context.Context, tconn *chrome.TestConn) error

// LaunchApp launches the camera app and handles the communication flow between tests and app.
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

		conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(windowURL))
		if err != nil {
			return nil, err
		}
		conn.StartProfiling(ctx)
		if err := appWindow.NotifyReady(ctx); err != nil {
			if closeErr := conn.CloseTarget(ctx); closeErr != nil {
				testing.ContextLog(ctx, "Failed to close app: ", closeErr)
			}
			if closeErr := conn.Close(); closeErr != nil {
				testing.ContextLog(ctx, "Failed to close app connection: ", closeErr)
			}
			return nil, err
		}
		return conn, nil
	}()
	if err != nil {
		if releaseErr := appWindow.Release(ctx); releaseErr != nil {
			testing.ContextLog(ctx, "Failed to release app window: ", releaseErr)
		}
		return nil, nil, err
	}
	return conn, appWindow, nil
}

// SWALauncher returns an app launcher which launches CCA as SWA.
func SWALauncher() AppLauncher {
	return func(ctx context.Context, tconn *chrome.TestConn) error {
		return apps.LaunchSystemWebApp(ctx, tconn, "Camera", "chrome://camera-app/views/main.html")
	}
}

// PlatformAppLauncher returns an app launcher which launches CCA as platform app.
func PlatformAppLauncher() AppLauncher {
	return func(ctx context.Context, tconn *chrome.TestConn) error {
		return tconn.Call(ctx, nil, `tast.promisify(chrome.management.launchApp)`, ID)
	}
}
