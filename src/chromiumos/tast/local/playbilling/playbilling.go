// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package playbilling

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/mafredri/cdp/protocol/input"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	appID              = "dlbmfdiobcnhnfocmenonncepnmhpckd"
	localServerPort    = 80
	localServerAddress = "http://127.0.0.1/"
)

type elementCoordinates struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// TestPWA holds references to the http.Server and the underlying
// PWA CDP connection.
type TestPWA struct {
	server      *http.Server
	pbConn      *chrome.Conn
	uiAutomator *ui.Device
}

// NewTestPWA sets up a local HTTP server to serve the PWA for which the test
// Android app points to.
func NewTestPWA(ctx context.Context, cr *chrome.Chrome, arcDevice *arc.ARC, pwaDir string) (pwa *TestPWA, retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	uiAutomator, err := arcDevice.NewUIDevice(ctx)
	if err != nil {
		if err := arcDevice.Close(cleanupCtx); err != nil {
			testing.ContextLog(cleanupCtx, "Failed to close ARC device: ", err)
		}
		return nil, errors.Wrap(err, "failed to initialize UI automator")
	}

	if err := arcDevice.WaitIntentHelper(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to wait for intent helper")
	}

	fs := http.FileServer(http.Dir(pwaDir))
	server := &http.Server{Addr: fmt.Sprintf(":%v", localServerPort), Handler: fs}
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			testing.ContextLog(ctx, "Failed to create local server: ", err)
		}
	}()
	defer func() {
		if retErr != nil {
			server.Shutdown(cleanupCtx)
		}
	}()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed getting Test API connection")
	}
	defer tconn.Close()

	if err := apps.Launch(ctx, tconn, appID); err != nil {
		return nil, errors.Wrapf(err, "failed launching app ID %q", appID)
	}

	pbConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(localServerAddress))
	if err != nil {
		return nil, errors.Wrapf(err, "failed getting connection for target: %q", localServerAddress)
	}

	return &TestPWA{
		server:      server,
		pbConn:      pbConn,
		uiAutomator: uiAutomator,
	}, nil
}

// Close performs cleanup actions for the PWA.
func (p *TestPWA) Close(ctx context.Context) {
	if err := p.shutdownServer(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to shutdown the PWA server: ", err)
	}

	if err := p.uiAutomator.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close the UIAutomator: ", err)
	}
}

func (p *TestPWA) shutdownServer(ctx context.Context) error {
	return p.server.Shutdown(ctx)
}

// BuySKU clicks a button on the test PWA where the SKU is the element ID.
func (p *TestPWA) BuySKU(ctx context.Context, sku string) error {
	return p.clickElementByID(ctx, sku)
}

func (p *TestPWA) waitForStableElementByID(ctx context.Context, id string) (elementCoordinates, error) {
	jsExpr := fmt.Sprintf("document.getElementById('%s')", id)
	if err := p.pbConn.WaitForExprFailOnErrWithTimeout(ctx, fmt.Sprintf("%s != undefined", jsExpr), 30*time.Second); err != nil {
		return elementCoordinates{}, errors.Wrapf(err, "failed to wait for %q to be defined", jsExpr)
	}

	var previousLocation, currentLocation elementCoordinates
	start := time.Now()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := p.pbConn.Eval(ctx, fmt.Sprintf(`new Promise(resolve => {
			const domRect = %s.getBoundingClientRect();
			resolve({
				x: domRect.x,
				y: domRect.y,
			});
		})`, jsExpr), &currentLocation); err != nil {
			previousLocation = elementCoordinates{}
			return err
		}
		if currentLocation != previousLocation {
			previousLocation = currentLocation
			elapsed := time.Since(start)
			return errors.Errorf("element has not stopped changing location after %s", elapsed)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}); err != nil {
		return elementCoordinates{}, errors.Wrapf(err, "failed to wait for %q to stabilize", jsExpr)
	}

	return currentLocation, nil
}

func (p *TestPWA) clickElementByID(ctx context.Context, id string) error {
	currentLocation, err := p.waitForStableElementByID(ctx, id)
	if err != nil {
		return err
	}

	// Dispatch a CDP mouse event to click and release relative to the PWA window.
	// Can't call the JS API as require gesture to initiate mouse click.
	mousePressed := input.NewDispatchMouseEventArgs("mousePressed", currentLocation.X, currentLocation.Y).SetClickCount(1).SetButton(input.MouseButtonLeft)
	if err := p.pbConn.DispatchMouseEvent(ctx, mousePressed); err != nil {
		return errors.Wrapf(err, "failed to mouse down at %v", currentLocation)
	}
	mousePressed.Type = "mouseReleased"
	if err := p.pbConn.DispatchMouseEvent(ctx, mousePressed); err != nil {
		return errors.Wrapf(err, "failed to mouse up at %v", currentLocation)
	}

	return nil
}
