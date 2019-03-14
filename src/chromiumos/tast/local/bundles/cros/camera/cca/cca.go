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
	*chrome.Conn
}

func launchApp(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn) (*App, error) {
	const ccaID = "hfhhnacclhffhdffklopdkcgdhifgngh"
	// TODO(shik): Remove autotestPrivate by changing to chrome.management.launchApp().
	const codeTemplate = `
new Promise((resolve) => {
  chrome.autotestPrivate.launchApp(%q, resolve)
})`
	code := fmt.Sprintf(codeTemplate, ccaID)
	if err := tconn.EvalPromise(ctx, code, nil); err != nil {
		return nil, err
	}

	// TODO(shik): Unknown race, if we connect too fast then the window will
	// disappear. Not sure what's the correct thing to poll here.
	select {
	case <-time.After(time.Second):
	case <-ctx.Done():
		return nil, errors.New("timed out while sleeping before connecting to CCA")
	}

	ccaURL := fmt.Sprintf("chrome-extension://%s/views/main.html", ccaID)
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(ccaURL))
	if err != nil {
		return nil, err
	}
	return &App{conn}, nil
}

// TestFunc is the test body function to run in RunUITest.
type TestFunc func(cr *chrome.Chrome, tconn *chrome.Conn, app *App)

// RunUITest runs the test body function after the browser and CCA are setup.
func RunUITest(ctx context.Context, s *testing.State, tf TestFunc) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open test api: ", err)
	}

	app, err := launchApp(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to connect to CCA: ", err)
	}
	defer app.Close()

	code, err := ioutil.ReadFile(s.DataPath("cca_ui.js"))
	if err != nil {
		s.Fatal("Failed to read cca_ui.js: ", err)
	}

	if err := app.Eval(ctx, string(code), nil); err != nil {
		s.Fatal("Failed to eval cca_ui.js: ", err)
	}

	s.Log("CCA UI test initialized")
	tf(cr, tconn, app)
}

func (a *App) checkVideoState(ctx context.Context, active bool, duration time.Duration) error {
	codeTemplate := "Tast.isVideoActive() === %t"
	code := fmt.Sprintf(codeTemplate, active)

	wait := func() error {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return a.WaitForExpr(ctx, code)
	}
	if err := wait(); err != nil {
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
	if err := a.Eval(ctx, code, &ok); err != nil {
		return err
	}
	if !ok {
		return errors.Errorf("video state changed after %v", duration.Round(time.Millisecond))
	}
	return nil
}

// VideoActive checks the video is active for 1 second.
func (a *App) VideoActive(ctx context.Context) error {
	return a.checkVideoState(ctx, true, time.Second)
}

// VideoInactive checks the video is inactive for 1 second.
func (a *App) VideoInactive(ctx context.Context) error {
	return a.checkVideoState(ctx, false, time.Second)
}

// RestoreWindow restores the window, exiting a maximized, minimized, or fullscreen state.
func (a *App) RestoreWindow(ctx context.Context) error {
	return a.EvalPromise(ctx, "Tast.restoreWindow()", nil)
}

// MinimizeWindow minimizes the window.
func (a *App) MinimizeWindow(ctx context.Context) error {
	return a.EvalPromise(ctx, "Tast.minimizeWindow()", nil)
}

// MaximizeWindow maximizes the window.
func (a *App) MaximizeWindow(ctx context.Context) error {
	return a.EvalPromise(ctx, "Tast.maximizeWindow()", nil)
}

// FullscreenWindow fullscreens the window.
func (a *App) FullscreenWindow(ctx context.Context) error {
	return a.EvalPromise(ctx, "Tast.fullscreenWindow()", nil)
}
