// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testutil provides utilities to setup testing environment for camera
// tests.
package testutil

import (
	"context"
	"fmt"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// TestBridge is used to comminicate with CCA for test specific logic, such as test environment set-up/tear-down flow, performance/error monitoring.
type TestBridge struct {
	cr       *chrome.Chrome
	pageConn *chrome.Conn
	bridge   *chrome.JSObject
	isSWA    bool
}

// NewTestBridge returns a new test bridge instance.
func NewTestBridge(ctx context.Context, cr *chrome.Chrome, isSWA bool) (*TestBridge, error) {
	pageConn, bridge, err := constructTestBridge(ctx, cr, isSWA)
	if err != nil {
		return nil, err
	}
	return &TestBridge{cr, pageConn, bridge, isSWA}, nil
}

func constructTestBridge(ctx context.Context, cr *chrome.Chrome, isSWA bool) (*chrome.Conn, *chrome.JSObject, error) {
	var pageConn *chrome.Conn
	var err error
	if isSWA {
		pageConn, err = cr.NewConn(ctx, "chrome://camera-app/src/views/test.html")
	} else {
		pageConn, err = cr.NewConnForTarget(ctx, chrome.MatchTargetURL(BackgroundURL))
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
	`, JSPath(isSWA))
	var testBridge chrome.JSObject
	if err := pageConn.Call(ctx, &testBridge, codes); err != nil {
		if err2 := tearDownBridgePageConnection(ctx, cr, pageConn, isSWA); err2 != nil {
			testing.ContextLog(ctx, "Failed to tear down bridge page connection", err2)
		}
		return nil, nil, errors.Wrap(err, "failed to get test bridge")
	}
	return pageConn, &testBridge, nil
}

func tearDownBridgePageConnection(ctx context.Context, cr *chrome.Chrome, conn *chrome.Conn, isSWA bool) error {
	// For platform app, it does not make sense to close background page.
	if isSWA {
		checkTestPage := func(t *target.Info) bool {
			return t.URL == "chrome://camera-app/src/views/test.html"
		}
		if testPageAlive, err := cr.IsTargetAvailable(ctx, checkTestPage); err == nil {
			if testPageAlive {
				if err := conn.CloseTarget(ctx); err != nil {
					return errors.Wrap(err, "failed to call CloseTarget() on the bridge page connection")
				}
			}
		} else {
			testing.ContextLog(ctx, "Failed to check if test page is alive or not: ", err)
		}
	}
	if err := conn.Close(); err != nil {
		return errors.Wrap(err, "failed to call Close() on the bridge page connection")
	}
	return nil
}

// AppWindow registers and returns the app window which is used to communicate with the foreground window of CCA instance.
func (t *TestBridge) AppWindow(ctx context.Context) (*AppWindow, error) {
	var appWindow chrome.JSObject
	if err := t.bridge.Call(ctx, &appWindow, "function() { return this.registerUnboundWindow(); }"); err != nil {
		return nil, errors.Wrap(err, "failed to register app window")
	}
	return &AppWindow{&appWindow}, nil
}

// Reset reconstructs the connection to test bridge.
func (t *TestBridge) Reset(ctx context.Context) error {
	if err := t.TearDown(ctx); err != nil {
		return err
	}

	pageConn, bridge, err := constructTestBridge(ctx, t.cr, t.isSWA)
	if err != nil {
		return errors.Wrap(err, "failed to reconstruct test bridge")
	}
	t.pageConn = pageConn
	t.bridge = bridge
	return nil
}

// TearDown tears down the connection of test bridge.
func (t *TestBridge) TearDown(ctx context.Context) error {
	t.bridge.Release(ctx)
	return tearDownBridgePageConnection(ctx, t.cr, t.pageConn, t.isSWA)
}
