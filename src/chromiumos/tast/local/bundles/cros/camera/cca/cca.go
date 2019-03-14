// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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

func launchCCA(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn) (*chrome.Conn, error) {
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
		return nil, errors.New("Timed out while sleeping before connecting to CCA")
	}

	ccaURL := fmt.Sprintf("chrome-extension://%s/views/main.html", ccaID)
	return cr.NewConnForTarget(ctx, chrome.MatchTargetURL(ccaURL))
}

// Conn represents a connection to a CCA instance.
type Conn struct {
	*chrome.Conn
}

// TestFunc is the test body function to run in RunUITest.
type TestFunc func(
	ctx context.Context,
	s *testing.State,
	cr *chrome.Chrome,
	tconn *chrome.Conn,
	conn *Conn,
)

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
	defer tconn.Close()

	conn, err := launchCCA(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to connect to CCA: ", err)
	}
	defer conn.Close()

	code, err := ioutil.ReadFile(s.DataPath("cca_ui.js"))
	if err != nil {
		s.Fatal("Failed to read cca_ui.js: ", err)
	}

	if err := conn.Eval(ctx, string(code), nil); err != nil {
		s.Fatal("Failed to eval cca_ui.js: ", err)
	}

	s.Log("CCA UI test initialized")
	tf(ctx, s, cr, tconn, &Conn{conn})
}

func (c *Conn) checkVideoState(ctx context.Context, active bool, duration time.Duration) error {
	codeTemplate := "Tast.isVideoActive() === %t"
	code := fmt.Sprintf(codeTemplate, active)
	if err := c.WaitForExpr(ctx, code); err != nil {
		return err
	}

	select {
	case <-time.After(duration):
	case <-ctx.Done():
		return ctx.Err()
	}

	return c.WaitForExpr(ctx, code)
}

// CheckVideoIsActive checks the video is active for 1 second.
func (c *Conn) CheckVideoIsActive(ctx context.Context) error {
	return c.checkVideoState(ctx, true, time.Second)
}

// CheckVideoIsInactive checks the video is inactive for 1 second.
func (c *Conn) CheckVideoIsInactive(ctx context.Context) error {
	return c.checkVideoState(ctx, false, time.Second)
}

// RestoreWindow restores the window, exiting a maximized, minimized, or fullscreen state.
func (c *Conn) RestoreWindow(ctx context.Context) error {
	code := "Tast.restoreWindow()"
	return c.EvalPromise(ctx, code, nil)
}

// MinimizeWindow minimizes the window.
func (c *Conn) MinimizeWindow(ctx context.Context) error {
	code := "Tast.minimizeWindow()"
	return c.EvalPromise(ctx, code, nil)
}

// MaximizeWindow maximizes the window.
func (c *Conn) MaximizeWindow(ctx context.Context) error {
	code := "Tast.maximizeWindow()"
	return c.EvalPromise(ctx, code, nil)
}

// FullscreenWindow fullscreens the window.
func (c *Conn) FullscreenWindow(ctx context.Context) error {
	code := "Tast.fullscreenWindow()"
	return c.EvalPromise(ctx, code, nil)
}
