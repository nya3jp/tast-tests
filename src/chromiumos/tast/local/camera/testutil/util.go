// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testutil provides utilities to setup testing environment for camera
// tests.
package testutil

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
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

// USBCamerasFromV4L2Test returns a list of usb camera paths.
func USBCamerasFromV4L2Test(ctx context.Context) ([]string, error) {
	cmd := testexec.CommandContext(ctx, "media_v4l2_test", "--list_usbcam")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run media_v4l2_test")
	}
	return strings.Fields(string(out)), nil
}

// CaptureDevicesFromV4L2Test returns a list of usb camera paths.
func CaptureDevicesFromV4L2Test(ctx context.Context) ([]string, error) {
	cmd := testexec.CommandContext(ctx, "media_v4l2_test", "--list_capture_devices")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run media_v4l2_test")
	}
	return strings.Fields(string(out)), nil
}

// MIPICamerasFromCrOSCameraTool returns a list of MIPI camera information outputted from cros-camera-tool.
func MIPICamerasFromCrOSCameraTool(ctx context.Context) ([]map[string]string, error) {
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

// WaitForCameraSocket returns when the camera socket is ready
func WaitForCameraSocket(ctx context.Context) (*chrome.Chrome, error) {
	const exec = "cros_camera_connector_test"
	const socket = "/run/camera/camera3.sock"

	// TODO(b/151270948): Temporarily disable ARC.
	// The cros-camera service would kill itself when running the test if
	// arc_setup.cc is triggered at that time, which will fail the test.
	cr, err := chrome.New(ctx, chrome.ARCDisabled(), chrome.NoLogin())
	if err != nil {
		return nil, errors.Wrap(err, "failed to start chrome")
	}

	if err := upstart.EnsureJobRunning(ctx, "cros-camera"); err != nil {
		cr.Close(ctx)
		return nil, errors.Wrap(err, "failed to start cros-camera")
	}

	arcCameraGID, err := sysutil.GetGID("arc-camera")
	if err != nil {
		cr.Close(ctx)
		return nil, errors.Wrap(err, "failed to get gid of arc-camera")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		info, err := os.Stat(socket)
		if err != nil {
			return err
		}
		perm := info.Mode().Perm()
		if perm != 0660 {
			return errors.Errorf("perm %04o (want %04o)", perm, 0660)
		}
		st := info.Sys().(*syscall.Stat_t)
		if st.Gid != arcCameraGID {
			return errors.Errorf("gid %04o (want %04o)", st.Gid, arcCameraGID)
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil {
		cr.Close(ctx)
		return nil, errors.Wrap(err, "invalid camera socket")
	}

	return cr, nil
}
