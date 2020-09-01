// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cca provides utilities to interact with Chrome Camera App.
package cca

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// ChromeConfig is the config to set the desired features/switches when
// launching Chrome.
type ChromeConfig struct {
	InstallSWA              bool
	UseFakeCamera           bool
	UseFakeHumanFaceContent bool
	UseFakeDms              bool
	ARCEnabled              bool
	FileForFakeVideoCapture string
	FakeDmsURL              string
}

// TestEnvironment includes things we need to understand and manipulate
// current environment when testing.
type TestEnvironment struct {
	Chrome         *chrome.Chrome
	Config         ChromeConfig
	TestBridgeConn *chrome.Conn
	appWindows     []*chrome.JSObject
}

// AppLauncher is used during the launch process of CCA. We could launch CCA
// by launchApp event, camera intent or any other ways.
type AppLauncher func(ctx context.Context, tconn *chrome.TestConn) error

// SetupTestEnvironment sets up the test environment by given config.
func SetupTestEnvironment(ctx context.Context, config ChromeConfig) (*TestEnvironment, error) {
	var opts []chrome.Option
	opts = append(opts, chrome.ExtraArgs("--tast-test"))
	if config.InstallSWA {
		opts = append(opts, chrome.ExtraArgs("--enable-features=CameraSystemWebApp"))
	}
	if config.UseFakeCamera {
		opts = append(opts, chrome.ExtraArgs(
			"--use-fake-ui-for-media-stream",
			// The default fps of fake device is 20, but CCA requires fps >= 24.
			// Set the fps to 30 to avoid OverconstrainedError.
			"--use-fake-device-for-media-stream=fps=30"))
	}
	if config.UseFakeHumanFaceContent {
		opts = append(opts, chrome.ExtraArgs(
			"--use-file-for-fake-video-capture="+config.FileForFakeVideoCapture))
	}
	if config.ARCEnabled {
		opts = append(opts, chrome.ARCEnabled())
	}
	if config.UseFakeDms {
		opts = append(opts, chrome.Auth("tast-user@managedchrome.com", "test0000", "gaia-id"),
			chrome.DMSPolicy(config.FakeDmsURL))
	}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	var testBridgeConn *chrome.Conn
	if config.InstallSWA {
		testBridgeConn, err = cr.NewConn(ctx, "chrome://camera-app/src/views/test.html")
	} else {
		testBridgeConn, err = cr.NewConnForTarget(ctx, chrome.MatchTargetURL(BackgroundURL))
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to test bridge")
	}
	return &TestEnvironment{cr, config, testBridgeConn, nil}, nil
}

func launchApp(ctx context.Context, env *TestEnvironment, appLauncher AppLauncher, appWindow *chrome.JSObject) (*chrome.Conn, error) {
	// We need to wait until the test bridge is ready for connection.
	if err := env.TestBridgeConn.WaitForExpr(ctx, "window.registerUnboundWindow !== undefined"); err != nil {
		return nil, errors.Wrap(err, "failed to wait for test bridge ready")
	}

	if err := env.TestBridgeConn.Eval(ctx, "window.registerUnboundWindow()", appWindow); err != nil {
		return nil, errors.Wrap(err, "failed to get app window")
	}
	env.appWindows = append(env.appWindows, appWindow)

	cr := env.Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}
	if err := appLauncher(ctx, tconn); err != nil {
		return nil, err
	}

	var windowURL string
	if appWindow.Call(ctx, &windowURL, "function() { return this.waitUntilWindowBound(); }"); err != nil {
		return nil, err
	}

	var conn *chrome.Conn
	if conn, err = cr.NewConnForTarget(ctx, chrome.MatchTargetURL(windowURL)); err != nil {
		return nil, err
	}
	conn.StartProfiling(ctx)
	if appWindow.Call(ctx, nil, "function() { this.notifyReadyOnTastSide(); }"); err != nil {
		return nil, err
	}
	return conn, nil
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

// ResetTestBridge tears down the original connection to the test bridge and
// reconstructs one.
func (e *TestEnvironment) ResetTestBridge(ctx context.Context) error {
	e.TearDown(ctx)

	var err error
	if e.Config.InstallSWA {
		e.TestBridgeConn, err = e.Chrome.NewConn(ctx, "chrome://camera-app/src/views/test.html")
	} else {
		e.TestBridgeConn, err = e.Chrome.NewConnForTarget(ctx, chrome.MatchTargetURL(BackgroundURL))
	}
	return err
}

// TearDown tears down the connection to the test bridge.
func (e *TestEnvironment) TearDown(ctx context.Context) {
	for _, appWindow := range e.appWindows {
		appWindow.Release(ctx)
	}

	// For platform app, it does not make sense to close background page.
	if e.Config.InstallSWA {
		if err := e.TestBridgeConn.CloseTarget(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to CloseTarget() when tearing down test environments")
		}
	}
	if err := e.TestBridgeConn.Close(); err != nil {
		testing.ContextLog(ctx, "Failed to Close() when tearing down test environments")
	}
}
