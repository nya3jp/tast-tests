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

// Facing is camera facing from JavaScript VideoFacingModeEnum.
type Facing string

const (
	// FacingBack is the constant string from JavaScript VideoFacingModeEnum.
	FacingBack Facing = "environment"
	// FacingFront is the constant string from JavaScript VideoFacingModeEnum.
	FacingFront = "user"
)

// App represents a CCA (Chrome Camera App) instance.
type App struct {
	conn        *chrome.Conn
	cr          *chrome.Chrome
	scriptPaths []string
}

// New launches a CCA instance and evaluates the helper script within it. The
// scriptPath should be the data path to the helper script cca_ui.js.
func New(ctx context.Context, cr *chrome.Chrome, scriptPaths []string) (*App, error) {
	const ccaID = "hfhhnacclhffhdffklopdkcgdhifgngh"

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	launchApp := fmt.Sprintf(`
		new Promise((resolve, reject) => {
		  chrome.management.launchApp(%q, () => {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve();
		    }
		  })
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
	defer bconn.Close()
	if err := bconn.WaitForExpr(ctx, "cca.bg.appWindowCreated"); err != nil {
		return nil, err
	}

	ccaURL := fmt.Sprintf("chrome-extension://%s/views/main.html", ccaID)
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(ccaURL))
	if err != nil {
		return nil, err
	}

	// Let CCA perform some one-time initialization after launched.  Otherwise
	// the first CheckVideoActive() might timed out because it's still
	// initializing, especially on low-end devices and when the system is busy.
	waitIdle := "new Promise(resolve => requestIdleCallback(resolve, {timeout: 30000}))"
	if err := conn.EvalPromise(ctx, waitIdle, nil); err != nil {
		return nil, err
	}

	for _, scriptPath := range scriptPaths {
		script, err := ioutil.ReadFile(scriptPath)
		if err != nil {
			return nil, err
		}
		if err := conn.Eval(ctx, string(script), nil); err != nil {
			return nil, err
		}
	}

	testing.ContextLog(ctx, "CCA launched")
	return &App{conn, cr, scriptPaths}, nil
}

// Close closes the App and the associated connection.
func (a *App) Close(ctx context.Context) error {
	if a.conn == nil {
		// It's already closed. Do nothing.
		return nil
	}
	var firstErr error
	if err := a.conn.CloseTarget(ctx); err != nil {
		firstErr = errors.Wrap(err, "failed to CloseTarget()")
	}
	if err := a.conn.Close(); err != nil && firstErr == nil {
		firstErr = errors.Wrap(err, "failed to Conn.Close()")
	}
	a.conn = nil
	return firstErr
}

// Restart restarts the App and resets the associated connection.
func (a *App) Restart(ctx context.Context) error {
	if err := a.Close(ctx); err != nil {
		return err
	}
	newApp, err := New(ctx, a.cr, a.scriptPaths)
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
	if err := testing.Sleep(ctx, duration); err != nil {
		return err
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

// GetNumOfCameras returns number of camera devices.
func (a *App) GetNumOfCameras(ctx context.Context) (int, error) {
	var numCameras int
	err := a.conn.EvalPromise(ctx, "CCAUIMultiCamera.getNumOfCameras()", &numCameras)
	return numCameras, err
}

// CheckFacing returns an error if the active camera facing is not expected.
func (a *App) CheckFacing(ctx context.Context, expected Facing) error {
	checkFacing := fmt.Sprintf("CCAUIMultiCamera.checkFacing(%q)", expected)
	return a.conn.EvalPromise(ctx, checkFacing, nil)
}

// CheckSwitchDeviceButtonExist returns an error if whether switch button exists is not expected.
func (a *App) CheckSwitchDeviceButtonExist(ctx context.Context, expected bool) error {
	var actual bool
	err := a.conn.Eval(ctx, "CCAUIMultiCamera.switchCameraButtonExist()", &actual)
	if err != nil {
		return err
	} else if actual != expected {
		return errors.Errorf("unexpected switch button existence: got %v, want %v", actual, expected)
	}
	return nil
}

// ToggleGridOption toggles the grid option and returns whether it's enabled after toggling.
func (a *App) ToggleGridOption(ctx context.Context) (bool, error) {
	var grid bool
	err := a.conn.EvalPromise(ctx, "CCAUIMultiCamera.toggleGrid()", &grid)
	return grid, err
}

// SwitchCamera switches to next camera device.
func (a *App) SwitchCamera(ctx context.Context) error {
	return a.conn.EvalPromise(ctx, "CCAUIMultiCamera.switchCamera()", nil)
}

// CheckGridOption checks whether grid option enable state is as expected.
func (a *App) CheckGridOption(ctx context.Context, expected bool) error {
	var actual bool
	err := a.conn.Eval(ctx, "cca.state.get('grid')", &actual)
	if err != nil {
		return err
	} else if actual != expected {
		return errors.Errorf("unexpected grid option enablement: got %v, want %v", actual, expected)
	}
	return nil
}
