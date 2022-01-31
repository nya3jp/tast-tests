// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testutil provides utilities to setup testing environment for camera
// tests.
package testutil

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
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
type AppLauncher struct {
	LaunchApp    func(ctx context.Context, tconn *chrome.TestConn) error
	UseSWAWindow bool
}

// LaunchApp launches the camera app and handles the communication flow between tests and app.
func LaunchApp(ctx context.Context, cr *chrome.Chrome, tb *TestBridge, appLauncher AppLauncher) (*chrome.Conn, *AppWindow, error) {
	appWindow, err := tb.AppWindow(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to register app window")
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	conn, err := func() (*chrome.Conn, error) {
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return nil, err
		}
		if err := appLauncher.LaunchApp(ctx, tconn); err != nil {
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
		if releaseErr := appWindow.Release(cleanupCtx); releaseErr != nil {
			testing.ContextLog(cleanupCtx, "Failed to release app window: ", releaseErr)
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

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()
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
		if releaseErr := appWindow.Release(cleanupCtx); releaseErr != nil {
			testing.ContextLog(cleanupCtx, "Failed to release app window: ", releaseErr)
		}
		return nil, err
	}
	return appWindow, nil
}

// CloseApp closes the camera app and ensure the window is closed via autotest private API.
func CloseApp(ctx context.Context, cr *chrome.Chrome, appConn *chrome.Conn, useSWAWindow bool) error {
	if err := appConn.CloseTarget(ctx); err != nil {
		return errors.Wrap(err, "failed to close target")
	}
	if !useSWAWindow {
		return nil
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get test API connection")
	}
	return testing.Poll(ctx, func(ctx context.Context) error {
		var isOpen bool
		if err := tconn.Call(ctx, &isOpen, `tast.promisify(chrome.autotestPrivate.isSystemWebAppOpen)`, apps.Camera.ID); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to check if camera app is open via autotestPrivate.isSystemWebAppOpen API"))
		}
		if isOpen {
			return errors.New("unexpected result returned by autotestPrivate.isSystemWebAppOpen API: got true; want false")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
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

// GetCaptureDevicesFromV4L2Test returns a list of usb camera paths.
func GetCaptureDevicesFromV4L2Test(ctx context.Context) ([]string, error) {
	cmd := testexec.CommandContext(ctx, "media_v4l2_test", "--list_capture_devices")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}

// GetMIPICamerasFromCrOSCameraTool returns a list of MIPI camera information outputted from cros-camera-tool.
func GetMIPICamerasFromCrOSCameraTool(ctx context.Context) ([]map[string]string, error) {
	cmd := testexec.CommandContext(ctx, "cros-camera-tool", "modules", "list")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run cros-camera-tool")
	}
	var cams []map[string]string
	if err := json.Unmarshal(out, &cams); err != nil {
		return nil, errors.Wrap(err, "failed to parse cros-camera-tool output")
	}
	return cams, nil
}

// IsVividDriverLoaded returns whether vivid driver is loaded on the device.
func IsVividDriverLoaded(ctx context.Context) bool {
	cmd := testexec.CommandContext(ctx, "sh", "-c", "lsmod | grep -q vivid")
	return cmd.Run() == nil
}
