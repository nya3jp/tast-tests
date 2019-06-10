// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cca provides utilities to interact with Chrome Camera App.
package cca

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
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

// Mode is capture mode in CCA.
type Mode string

const (
	// Video is the mode used to record video.
	Video Mode = "video-mode"
	// Photo is the mode used to take photo.
	Photo Mode = "photo-mode"
	// Square is the mode used to take square photo.
	Square Mode = "square-mode"
	// Portrait is the mode used to take portrait photo.
	Portrait Mode = "portrait-mode"
)

var (
	// PhotoPattern is the filename format of photoes taken by CCA.
	PhotoPattern = regexp.MustCompile(`^IMG_\d{8}_\d{6}[^.]*\.jpg$`)
	// VideoPattern is the filename format of videos recorded by CCA.
	VideoPattern = regexp.MustCompile(`^VID_\d{8}_\d{6}[^.]*\.mkv$`)
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

// WaitForVideoActive waits for the video to become active for 1 second.
func (a *App) WaitForVideoActive(ctx context.Context) error {
	return a.checkVideoState(ctx, true, time.Second)
}

// WaitForSavedFile waits for captured file to be saved.
func (a *App) WaitForSavedFile(ctx context.Context, pat *regexp.Regexp, ts time.Time) (os.FileInfo, error) {
	path, err := a.GetSavedDir(ctx)
	if err != nil {
		return nil, err
	}

	const timeout = 5 * time.Second
	var result os.FileInfo
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			return errors.Wrap(err, "failed to read the directory for saving media files")
		}
		curTs := ts
		for _, file := range files {
			if file.ModTime().Before(curTs) {
				continue
			}
			if file.ModTime().After(ts) {
				ts = file.ModTime()
			}
			testing.ContextLog(ctx, "New file found: ", file.Name())
			if pat.MatchString(file.Name()) {
				testing.ContextLog(ctx, "Found a match: ", file.Name())
				result = file
				return nil
			}
		}
		return errors.New("no matching output file found")
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		err = errors.Wrapf(err, "no matching output file found after %v", timeout)
	}
	return result, err
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

// GetState gets value of option from its classname.
func (a *App) GetState(ctx context.Context, state string) (bool, error) {
	var result bool
	err := a.conn.Eval(ctx, fmt.Sprintf("cca.state.get('%s')", state), &result)
	if err != nil {
		err = errors.Wrapf(err, "failed to get state: %v", state)
	}
	return result, err
}

// GetTimerDelay gets duration of timer delay.
func (a *App) GetTimerDelay(ctx context.Context) (time.Duration, error) {
	if delay10, err := a.GetState(ctx, "_10sec"); err != nil {
		return 0, errors.Wrap(err, "failed to get 10 seconds timer state")
	} else if delay10 {
		return 10 * time.Second, nil
	}
	return 3 * time.Second, nil
}

// GetSavedDir gets path where captured files are saved.
func (a *App) GetSavedDir(ctx context.Context) (string, error) {
	path, err := cryptohome.UserPath(ctx, a.cr.User())
	if err == nil {
		path = filepath.Join(path, "Downloads")
	}
	return path, err
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

func (a *App) toggleOption(ctx context.Context, option string, toggleSelector string) (bool, error) {
	prev, err := a.GetState(ctx, option)
	if err != nil {
		return false, err
	}
	code := fmt.Sprintf("document.querySelector('%s').click()", toggleSelector)
	if err := a.conn.Eval(ctx, code, nil); err != nil {
		return false, errors.Wrapf(err, "failed to click on toggle button of selector: %v", toggleSelector)
	}
	code = fmt.Sprintf("cca.state.get('%s') !== %t", option, prev)
	if err := a.conn.WaitForExpr(ctx, code); err != nil {
		return false, errors.Wrapf(err, "failed to wait for toggling option: %v", option)
	}
	return a.GetState(ctx, option)
}

// ToggleGridOption toggles the grid option and returns whether it's enabled after toggling.
func (a *App) ToggleGridOption(ctx context.Context) (bool, error) {
	return a.toggleOption(ctx, "grid", "#toggle-grid")
}

// ToggleTimerOption toggles the timer option and returns whether it's enabled after toggling.
func (a *App) ToggleTimerOption(ctx context.Context) (bool, error) {
	return a.toggleOption(ctx, "timer", "#toggle-timer")
}

// ClickShutter clicks the shutter button.
func (a *App) ClickShutter(ctx context.Context) error {
	err := a.conn.Eval(ctx, "CCAUICapture.clickShutter()", nil)
	if err != nil {
		err = errors.Wrap(err, "failed to click shutter button")
	}
	return err
}

// SwitchCamera switches to next camera device.
func (a *App) SwitchCamera(ctx context.Context) error {
	return a.conn.EvalPromise(ctx, "CCAUIMultiCamera.switchCamera()", nil)
}

// SwitchMode switches to specified capture mode.
func (a *App) SwitchMode(ctx context.Context, mode Mode) error {
	if active, err := a.GetState(ctx, string(mode)); err != nil {
		return err
	} else if active {
		return nil
	}
	code := fmt.Sprintf("CCAUICapture.switchMode('%s')", mode)
	if err := a.conn.Eval(ctx, code, nil); err != nil {
		return errors.Wrapf(err, "failed to switch to mode: %v", mode)
	}
	if err := a.WaitForVideoActive(ctx); err != nil {
		return errors.Wrapf(err, "Preview is inactive after switch to mode: %v", mode)
	}
	return nil
}

// WaitForState waits until state become active/inactive.
func (a *App) WaitForState(ctx context.Context, state string, active bool) error {
	code := fmt.Sprintf("cca.state.get('%s') === %t", state, active)
	if err := a.conn.WaitForExpr(ctx, code); err != nil {
		return errors.Wrapf(err, "failed to wait for activation of state: %v, %v", state, active)
	}
	return nil
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
