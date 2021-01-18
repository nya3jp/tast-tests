// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testutil provides utilities to setup testing environment for camera
// tests.
package testutil

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

var (
	fakeCameraOptions = chrome.ExtraArgs(
		// The default fps of fake device is 20, but CCA requires fps >= 24.
		// Set the fps to 30 to avoid OverconstrainedError.
		"--use-fake-device-for-media-stream=fps=30")
	chromeWithFakeCamera = chrome.NewPrecondition("chrome_fake_camera", fakeCameraOptions)
)

// ChromeWithFakeCamera returns a precondition that Chrome is already logged in with fake camera.
func ChromeWithFakeCamera() testing.Precondition { return chromeWithFakeCamera }

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

// RefreshApp refreshes the camera app and rebuilds the communication flow between tests and app.
func RefreshApp(ctx context.Context, conn *chrome.Conn, tb *TestBridge) (*AppWindow, error) {
	appWindow, err := tb.AppWindow(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to register app window")
	}

	err = func() error {
		// Triggers refresh.
		if err := conn.Eval(ctx, "location.reload()", nil); err != nil {
			return errors.Wrap(err, "failed to trigger refresh")
		}

		_, err = appWindow.WaitUntilWindowBound(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to bind window")
		}

		if err := appWindow.NotifyReady(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for app window ready")
		}
		return nil
	}()
	if err != nil {
		if releaseErr := appWindow.Release(ctx); releaseErr != nil {
			testing.ContextLog(ctx, "Failed to release app window: ", releaseErr)
		}
		return nil, err
	}
	return appWindow, nil
}

// GetUSBCamerasFromV4L2Test returns a list of usb camera paths.
func GetUSBCamerasFromV4L2Test(ctx context.Context) ([]string, error) {
	cmd := testexec.CommandContext(ctx, "media_v4l2_test", "--list_usbcam")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}
