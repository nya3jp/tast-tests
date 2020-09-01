// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cca provides utilities to interact with Chrome Camera App.
package cca

import (
	"context"
	"fmt"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
)

// ChromeConfig is the config to set the desired features/switches when
// launching Chrome.
type ChromeConfig struct {
	InstallSWA              bool
	UseFakeCamera           bool
	UseFakeHumanFaceContent bool
	UseFakeDMS              bool
	ARCEnabled              bool
}

// TestEnvironment includes things we need to understand and manipulate
// current environment when testing.
type TestEnvironment struct {
	Chrome         *chrome.Chrome
	Config         ChromeConfig
	TestBridgeConn *chrome.Conn
	FakeDMS        *fakedms.FakeDMS
	appWindows     []*chrome.JSObject
}

// AppLauncher is used during the launch process of CCA. We could launch CCA
// by launchApp event, camera intent or any other ways.
type AppLauncher func(ctx context.Context, tconn *chrome.TestConn) error

func launchApp(ctx context.Context, env *TestEnvironment, appLauncher AppLauncher) (*chrome.Conn, *chrome.JSObject, error) {
	var jsPath string
	if env.Config.InstallSWA {
		jsPath = systemWebAppJSPath
	} else {
		jsPath = platformAppJSPath
	}
	codes := fmt.Sprintf(`
	  async function() {
	    const {registerUnboundWindow} = await import('%s/test_bridge.js');
	    return registerUnboundWindow();
	  }
	`, jsPath)
	var appWindow chrome.JSObject
	if err := env.TestBridgeConn.Call(ctx, &appWindow, codes); err != nil {
		return nil, nil, errors.Wrap(err, "failed to register test app window")
	}

	conn, err := func() (*chrome.Conn, error) {
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
	}()
	if err != nil {
		appWindow.Release(ctx)
		return nil, nil, err
	}
	env.appWindows = append(env.appWindows, &appWindow)
	return conn, &appWindow, nil
}

// launchSWA launches CCA as SWA.
func launchSWA(ctx context.Context, tconn *chrome.TestConn) error {
	return apps.LaunchSystemWebApp(ctx, tconn, "Camera",
		"chrome://camera-app/src/views/main.html")
}

// launchPlatformApp launches CCA as platform app.
func launchPlatformApp(ctx context.Context, tconn *chrome.TestConn) error {
	return tconn.Call(ctx, nil, `tast.promisify(chrome.management.launchApp)`, ID)
}

// ResetTestBridge tears down the original connection to the test bridge and
// reconstructs one.
func (e *TestEnvironment) ResetTestBridge(ctx context.Context) error {
	e.TearDownTestBridge(ctx)

	var err error
	if e.Config.InstallSWA {
		e.TestBridgeConn, err = e.Chrome.NewConn(ctx, "chrome://camera-app/src/views/test.html")
	} else {
		e.TestBridgeConn, err = e.Chrome.NewConnForTarget(ctx, chrome.MatchTargetURL(BackgroundURL))
	}
	return err
}

// TearDownTestBridge tears down the connection to the test bridge.
func (e *TestEnvironment) TearDownTestBridge(ctx context.Context) error {
	for _, appWindow := range e.appWindows {
		appWindow.Release(ctx)
	}

	// For platform app, it does not make sense to close background page.
	if e.Config.InstallSWA {
		if err := e.TestBridgeConn.CloseTarget(ctx); err != nil {
			return errors.Wrap(err, "failed to call CloseTarget() on test bridge connection")
		}
	}
	if err := e.TestBridgeConn.Close(); err != nil {
		return errors.Wrap(err, "failed to call Close() on test bridge connection")
	}

	return nil
}

// TearDown tears down the whole test environment.
func (e *TestEnvironment) TearDown(ctx context.Context) error {
	if err := e.TearDownTestBridge(ctx); err != nil {
		return errors.Wrap(err, "failed to tear down test bridge")
	}

	if e.FakeDMS != nil {
		e.FakeDMS.Stop(ctx)
	}

	if err := e.Chrome.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close Chrome")
	}

	return nil
}
