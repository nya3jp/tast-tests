// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cca provides utilities to interact with Chrome Camera App.
package cca

import (
	"context"
	"fmt"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/common/policy/fakedms"
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
	UseFakeDMS              bool
	ARCEnabled              bool
}

// TestEnvironment includes things we need to understand and manipulate
// current environment when testing.
type TestEnvironment struct {
	Chrome         *chrome.Chrome
	Config         ChromeConfig
	BridgePageConn *chrome.Conn
	TestBridge     *chrome.JSObject
	FakeDMS        *fakedms.FakeDMS
}

// AppLauncher is used during the launch process of CCA. We could launch CCA
// by launchApp event, camera intent or any other ways.
type AppLauncher func(ctx context.Context, tconn *chrome.TestConn) error

func constructTestBridge(ctx context.Context, cr *chrome.Chrome, isSWA bool) (*chrome.Conn, *chrome.JSObject, error) {
	var bridgePageConn *chrome.Conn
	var err error
	if isSWA {
		bridgePageConn, err = cr.NewConn(ctx, "chrome://camera-app/src/views/test.html")
	} else {
		bridgePageConn, err = cr.NewConnForTarget(ctx, chrome.MatchTargetURL(BackgroundURL))
	}
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to construct bridge page connection")
	}

	codes := fmt.Sprintf(`
	  async function() {
		let workerPath;
		if (window.location.href.startsWith('chrome-extension://')) {
		  workerPath = '/js/test_bridge.js';
		} else {
		  workerPath = 'chrome://camera-app/src/js/test_bridge.js';
		}

		const sharedWorker = new SharedWorker(workerPath, {type: 'module'});
		const Comlink = await import('%s/lib/comlink.js');
		return Comlink.wrap(sharedWorker.port);
	  }
	`, jsPath(isSWA))
	var testBridge chrome.JSObject
	if err := bridgePageConn.Call(ctx, &testBridge, codes); err != nil {
		if err2 := tearDownBridgePageConnection(ctx, cr, bridgePageConn, isSWA); err2 != nil {
			testing.ContextLog(ctx, "Failed to tear down bridge page connection", err2)
		}
		return nil, nil, errors.Wrap(err, "failed to get test bridge")
	}
	return bridgePageConn, &testBridge, nil
}

func jsPath(isSWA bool) string {
	if isSWA {
		return systemWebAppJSPath
	}
	return platformAppJSPath
}

func launchApp(ctx context.Context, env *TestEnvironment, appLauncher AppLauncher) (*chrome.Conn, *chrome.JSObject, error) {
	var appWindow chrome.JSObject
	if err := env.TestBridge.Call(ctx, &appWindow, "function() { return this.registerUnboundWindow(); }"); err != nil {
		return nil, nil, errors.Wrap(err, "failed to register app window")
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

// tearDownBridgePageConnection tears down the connection to the bridge page.
func tearDownBridgePageConnection(ctx context.Context, cr *chrome.Chrome, conn *chrome.Conn, isSWA bool) error {
	// For platform app, it does not make sense to close background page.
	if isSWA {
		checkTestPage := func(t *target.Info) bool {
			return t.URL == "chrome://camera-app/src/views/test.html"
		}
		if testPageAlive, err := cr.IsTargetAvailable(ctx, checkTestPage); testPageAlive && err == nil {
			if err := conn.CloseTarget(ctx); err != nil {
				return errors.Wrap(err, "failed to call CloseTarget() on the bridge page connection")
			}
		}
	}
	if err := conn.Close(); err != nil {
		return errors.Wrap(err, "failed to call Close() on the bridge page connection")
	}
	return nil
}

// tearDownTestBridge tears down the connection to the test bridge.
func (e *TestEnvironment) tearDownTestBridge(ctx context.Context) error {
	e.TestBridge.Release(ctx)
	e.TestBridge = nil

	if err := tearDownBridgePageConnection(ctx, e.Chrome, e.BridgePageConn, e.Config.InstallSWA); err != nil {
		return errors.Wrap(err, "failed to tear dwon bridge page connection")
	}
	e.BridgePageConn = nil
	return nil
}

// tearDown tears down the whole test environment.
func (e *TestEnvironment) tearDown(ctx context.Context) error {
	if err := e.tearDownTestBridge(ctx); err != nil {
		return errors.Wrap(err, "failed to tear down test bridge")
	}

	if e.FakeDMS != nil {
		e.FakeDMS.Stop(ctx)
		e.FakeDMS = nil
	}

	if err := e.Chrome.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close Chrome")
	}
	e.Chrome = nil

	return nil
}

// ResetTestBridge tears down the original connection to the test bridge and
// reconstructs one.
func (e *TestEnvironment) ResetTestBridge(ctx context.Context) error {
	if err := e.tearDownTestBridge(ctx); err != nil {
		return errors.Wrap(err, "failed to tear down test bridge")
	}
	bridgePageConn, testBridge, err := constructTestBridge(ctx, e.Chrome, e.Config.InstallSWA)
	if err != nil {
		return errors.Wrap(err, "failed to reconstruct test bridge")
	}
	e.BridgePageConn = bridgePageConn
	e.TestBridge = testBridge
	return nil
}
