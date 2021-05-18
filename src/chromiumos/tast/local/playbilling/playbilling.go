// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package playbilling

import (
	"context"
	"fmt"
	"net/http"
	"time"

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
func (p *TestPWA) Close(ctx context.Context) error {
	if err := p.shutdownServer(ctx); err != nil {
		return errors.Wrap(err, "failed to shutdown server")
	}

	return nil
}

func (p *TestPWA) shutdownServer(ctx context.Context) error {
	return p.server.Shutdown(ctx)
}
