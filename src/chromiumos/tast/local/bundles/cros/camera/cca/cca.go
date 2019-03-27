// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cca provides utilities to interact with Chrome Camera App.
package cca

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// App represents a CCA (Chrome Camera App) instance.
type App struct {
	conn       *chrome.Conn
	cr         *chrome.Chrome
	scriptPath string
}

// New launches a CCA instance and evaluates the helper script within it. The
// scriptPath should be the data path to the helper script cca_ui.js.
func New(ctx context.Context, cr *chrome.Chrome, scriptPath string) (*App, error) {
	const ccaID = "hfhhnacclhffhdffklopdkcgdhifgngh"

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	launchApp := fmt.Sprintf(`
		new Promise((resolve) => {
		  chrome.management.launchApp(%q, resolve)
		})`, ccaID)
	if err := tconn.EvalPromise(ctx, launchApp, nil); err != nil {
		return nil, err
	}

	// Wait until the window is created before connecting to it, otherwise there
	// is a race that may make the window disappear.
	bgURL := chrome.ExtensionBackgroundPageURL(ccaID)
	bconn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		return nil, err
	}
	if err := bconn.WaitForExpr(ctx, "cca.bg.appWindowCreated"); err != nil {
		return nil, err
	}

	ccaURL := fmt.Sprintf("chrome-extension://%s/views/main.html", ccaID)
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(ccaURL))
	if err != nil {
		return nil, err
	}

	script, err := ioutil.ReadFile(scriptPath)
	if err != nil {
		return nil, err
	}
	if err := conn.Eval(ctx, string(script), nil); err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "CCA launched")
	return &App{conn, cr, scriptPath}, nil
}

// Close closes the App and the associated connection.
func (a *App) Close(ctx context.Context) error {
	if err := a.conn.CloseTarget(ctx); err != nil {
		return errors.Wrap(err, "failed to CloseTarget()")
	}
	if err := a.conn.Close(); err != nil {
		return errors.Wrap(err, "failed to Conn.Close()")
	}
	return nil
}

// Restart restarts the App and resets the associated connection.
func (a *App) Restart(ctx context.Context) error {
	if err := a.Close(ctx); err != nil {
		return err
	}
	newApp, err := New(ctx, a.cr, a.scriptPath)
	if err != nil {
		return err
	}
	*a = *newApp
	return nil
}

func (a *App) checkVideoState(ctx context.Context, active bool, duration time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	code := fmt.Sprintf("Tast.isVideoActive() === %t", active)
	if err := a.conn.WaitForExpr(ctx, code); err != nil {
		return err
	}

	// Due to the pipeline delay in camera stack, animation delay, and other
	// reasons, sometimes a bug would be triggered after several frames. Wait
	// |duration| here and check that the state does not change afterwards.
	select {
	case <-time.After(duration):
	case <-ctx.Done():
		return ctx.Err()
	}

	var ok bool
	if err := a.conn.Eval(ctx, code, &ok); err != nil {
		return err
	}
	if !ok {
		return errors.Errorf("video state changed after %v", duration.Round(time.Millisecond))
	}
	return nil
}

// CheckVideoActive checks the video is active for 1 second.
func (a *App) CheckVideoActive(ctx context.Context) error {
	return a.checkVideoState(ctx, true, time.Second)
}

// CheckVideoInactive checks the video is inactive for 1 second.
func (a *App) CheckVideoInactive(ctx context.Context) error {
	return a.checkVideoState(ctx, false, time.Second)
}

// RestoreWindow restores the window, exiting a maximized, minimized, or fullscreen state.
func (a *App) RestoreWindow(ctx context.Context) error {
	return a.conn.EvalPromise(ctx, "Tast.restoreWindow()", nil)
}

// MinimizeWindow minimizes the window.
func (a *App) MinimizeWindow(ctx context.Context) error {
	return a.conn.EvalPromise(ctx, "Tast.minimizeWindow()", nil)
}

// MaximizeWindow maximizes the window.
func (a *App) MaximizeWindow(ctx context.Context) error {
	return a.conn.EvalPromise(ctx, "Tast.maximizeWindow()", nil)
}

// FullscreenWindow fullscreens the window.
func (a *App) FullscreenWindow(ctx context.Context) error {
	return a.conn.EvalPromise(ctx, "Tast.fullscreenWindow()", nil)
}
