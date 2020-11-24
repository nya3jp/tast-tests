// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testutil provides utilities to setup testing environment for camera
// tests.
package testutil

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

var (
	// ID is the app id of CCA.
	ID = "hfhhnacclhffhdffklopdkcgdhifgngh"
	// BackgroundURL is the url of the CCA background page.
	BackgroundURL     = fmt.Sprintf("chrome-extension://%s/views/background.html", ID)
	fakeCameraOptions = chrome.ExtraArgs(
		// The default fps of fake device is 20, but CCA requires fps >= 24.
		// Set the fps to 30 to avoid OverconstrainedError.
		"--use-fake-device-for-media-stream=fps=30")
	chromeWithSWA                      = chrome.NewPrecondition("cca_swa", chrome.EnableFeatures("CameraSystemWebApp"))
	chromeWithSWAAndFakeCamera         = chrome.NewPrecondition("cca_fake_camera_swa", chrome.EnableFeatures("CameraSystemWebApp"), fakeCameraOptions)
	arcWithSWA                         = arc.NewPrecondition("arc_cca_swa", nil, "--enable-features=CameraSystemWebApp")
	chromeWithPlatformApp              = chrome.NewPrecondition("cca_platform_app", chrome.DisableFeatures("CameraSystemWebApp"))
	chromeWithPlatformAppAndFakeCamera = chrome.NewPrecondition("cca_fake_camera_platform_app", chrome.DisableFeatures("CameraSystemWebApp"), fakeCameraOptions)
	arcWithPlatformApp                 = arc.NewPrecondition("arc_cca_platform_app", nil, "--disable-features=CameraSystemWebApp")
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

// ChromeWithSWAAndFakeCamera returns a precondition that Chrome is already logged in with fake camera and CCA is installed as an SWA when a test is run.
func ChromeWithSWAAndFakeCamera() testing.Precondition { return chromeWithSWAAndFakeCamera }

// ARCWithSWA returns a precondition that ARC Container has already booted and CCA is installed as an SWA when a test is run.
func ARCWithSWA() testing.Precondition { return arcWithSWA }

// ChromeWithPlatformApp returns a precondition that Chrome is already logged in and CCA is installed as a platform app when a test is run.
func ChromeWithPlatformApp() testing.Precondition { return chromeWithPlatformApp }

// ChromeWithPlatformAppAndFakeCamera returns a precondition that Chrome is already logged in with fake camera and CCA is installed as a platform app when a test is run.
func ChromeWithPlatformAppAndFakeCamera() testing.Precondition {
	return chromeWithPlatformAppAndFakeCamera
}

// ARCWithPlatformApp returns a precondition that ARC Container has already booted and CCA is installed as a platform app when a test is run.
func ARCWithPlatformApp() testing.Precondition { return arcWithPlatformApp }

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

// GetUSBCamerasFromV4L2Test returns a list of usb camera paths.
func GetUSBCamerasFromV4L2Test(ctx context.Context) ([]string, error) {
	cmd := testexec.CommandContext(ctx, "media_v4l2_test", "--list_usbcam")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}
