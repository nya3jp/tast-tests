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
	"chromiumos/tast/local/upstart"
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
	Photo = "photo-mode"
	// Square is the mode used to take square photo.
	Square = "square-mode"
	// Portrait is the mode used to take portrait photo.
	Portrait = "portrait-mode"
)

var (
	// PhotoPattern is the filename format of photoes taken by CCA.
	PhotoPattern = regexp.MustCompile(`^IMG_\d{8}_\d{6}[^.]*\.jpg$`)
	// VideoPattern is the filename format of videos recorded by CCA.
	VideoPattern = regexp.MustCompile(`^VID_\d{8}_\d{6}[^.]*\.mkv$`)
)

// TimerDelay is default timer delay of CCA.
const TimerDelay time.Duration = 3 * time.Second

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

	// The cros-camera job exists only on boards that use the new camera stack.
	if upstart.JobExists(ctx, "cros-camera") {
		// Ensure that cros-camera service is running, because the service
		// might stopped due to the errors from some previous tests, and failed
		// to restart for some reasons.
		if err := upstart.EnsureJobRunning(ctx, "cros-camera"); err != nil {
			return nil, err
		}
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	bgURL := chrome.ExtensionBackgroundPageURL(ccaID)
	bconn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		return nil, err
	}
	defer bconn.Close()

	// TODO(shik): Remove the else branch after CCA get updated.
	const prepareForTesting = `
		window.readyForTesting = new Promise((resolve) => {
		  if (typeof cca.bg.onAppWindowCreatedForTesting !== 'undefined') {
		    cca.bg.onAppWindowCreatedForTesting = resolve;
		  } else {
		    const interval = setInterval(() => {
		      if (cca.bg.appWindowCreated) {
		        clearInterval(interval);
		        resolve();
		      }
		    }, 100);
		  }
		});`
	if err := bconn.Exec(ctx, prepareForTesting); err != nil {
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
		  });
		})`, ccaID)
	if err := tconn.EvalPromise(ctx, launchApp, nil); err != nil {
		return nil, err
	}

	// Wait until the window is created before connecting to it, otherwise there
	// is a race that may make the window disappear.
	if err := bconn.EvalPromise(ctx, "readyForTesting", nil); err != nil {
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
	// Fail the test early if it's timed out to make it easier to figure out
	// the real reason of a test failure.
	const waitIdle = `
		new Promise((resolve, reject) => {
		  const idleCallback = ({didTimeout}) => {
		    if (didTimeout) {
		      reject(new Error('Timed out initializing CCA'));
		    } else {
		      resolve();
		    }
		  };
		  requestIdleCallback(idleCallback, {timeout: 30000});
		});`
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
	testing.ContextLog(ctx, "CCA closed")
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

// WaitForFileSaved waits for the presence of the captured file with file name matching the specified
// pattern and modified time after the specified timestamp.
func (a *App) WaitForFileSaved(ctx context.Context, pat *regexp.Regexp, ts time.Time) (os.FileInfo, error) {
	path, err := a.getSavedDir(ctx)
	if err != nil {
		return nil, err
	}

	const timeout = 5 * time.Second
	var result os.FileInfo
	seen := make(map[string]struct{})
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			return errors.Wrap(err, "failed to read the directory where media files are saved")
		}
		for _, file := range files {
			if file.ModTime().Before(ts) {
				continue
			}
			if _, ok := seen[file.Name()]; ok {
				continue
			}
			seen[file.Name()] = struct{}{}
			testing.ContextLog(ctx, "New file found: ", file.Name())
			if pat.MatchString(file.Name()) {
				testing.ContextLog(ctx, "Found a match: ", file.Name())
				result = file
				return nil
			}
		}
		return errors.New("no matching output file found")
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return nil, errors.Wrapf(err, "no matching output file found after %v", timeout)
	}
	return result, nil
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

// GetFacing returns the active camera facing.
func (a *App) GetFacing(ctx context.Context) (Facing, error) {
	var facing Facing
	if err := a.conn.EvalPromise(ctx, "CCAUIPreviewOptions.getFacing()", &facing); err != nil {
		return "", err
	}
	return facing, nil
}

// GetState returns whether a state is active in CCA.
func (a *App) GetState(ctx context.Context, state string) (bool, error) {
	var result bool
	if err := a.conn.Eval(ctx, fmt.Sprintf("cca.state.get(%q)", state), &result); err != nil {
		return false, errors.Wrapf(err, "failed to get state: %v", state)
	}
	return result, nil
}

func (a *App) getSavedDir(ctx context.Context) (string, error) {
	path, err := cryptohome.UserPath(ctx, a.cr.User())
	if err != nil {
		return "", err
	}
	return filepath.Join(path, "Downloads"), err
}

// CheckFacing returns an error if the active camera facing is not expected.
func (a *App) CheckFacing(ctx context.Context, expected Facing) error {
	checkFacing := fmt.Sprintf("CCAUIMultiCamera.checkFacing(%q)", expected)
	return a.conn.EvalPromise(ctx, checkFacing, nil)
}

// Mirrored returns whether mirroring is on.
func (a *App) Mirrored(ctx context.Context) (bool, error) {
	var actual bool
	err := a.conn.Eval(ctx, "cca.state.get('mirror')", &actual)
	return actual, err
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

// MirrorButtonExists returns whether mirror button exists.
func (a *App) MirrorButtonExists(ctx context.Context) (bool, error) {
	var actual bool
	err := a.conn.Eval(ctx, "CCAUIPreviewOptions.mirrorButtonExist()", &actual)
	return actual, err
}

func (a *App) toggleOption(ctx context.Context, option string, toggleSelector string) (bool, error) {
	prev, err := a.GetState(ctx, option)
	if err != nil {
		return false, err
	}
	code := fmt.Sprintf("document.querySelector(%q).click()", toggleSelector)
	if err := a.conn.Eval(ctx, code, nil); err != nil {
		return false, errors.Wrapf(err, "failed to click on toggle button of selector %s", toggleSelector)
	}
	code = fmt.Sprintf("cca.state.get(%q) !== %t", option, prev)
	if err := a.conn.WaitForExpr(ctx, code); err != nil {
		return false, errors.Wrapf(err, "failed to wait for toggling option %s", option)
	}
	return a.GetState(ctx, option)
}

// ToggleGridOption toggles the grid option and returns whether it's enabled after toggling.
func (a *App) ToggleGridOption(ctx context.Context) (bool, error) {
	return a.toggleOption(ctx, "grid", "#toggle-grid")
}

// ToggleMirroringOption toggles the mirroring option.
func (a *App) ToggleMirroringOption(ctx context.Context) (bool, error) {
	return a.toggleOption(ctx, "mirror", "#toggle-mirror")
}

// SetTimerOption sets the timer option to active/inactive.
func (a *App) SetTimerOption(ctx context.Context, active bool) error {
	if cur, err := a.GetState(ctx, "timer"); err != nil {
		return err
	} else if cur != active {
		if _, err := a.toggleOption(ctx, "timer", "#toggle-timer"); err != nil {
			return err
		}
	}
	// Fix timer to 3 seconds for saving test time.
	if active {
		if delay3, err := a.GetState(ctx, "_3sec"); err != nil {
			return err
		} else if !delay3 {
			return errors.New("default timer is not set to 3 seconds")
		}
	}
	return nil
}

// ClickShutter clicks the shutter button.
func (a *App) ClickShutter(ctx context.Context) error {
	if err := a.conn.Eval(ctx, "CCAUICapture.clickShutter()", nil); err != nil {
		return errors.Wrap(err, "failed to click shutter button")
	}
	return nil
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
	code := fmt.Sprintf("CCAUICapture.switchMode(%q)", mode)
	if err := a.conn.Eval(ctx, code, nil); err != nil {
		return errors.Wrapf(err, "failed to switch to mode %s", mode)
	}
	if err := a.WaitForState(ctx, "mode-switching", false); err != nil {
		return errors.Wrap(err, "failed to wait for finishing of mode switching")
	}
	if err := a.WaitForVideoActive(ctx); err != nil {
		return errors.Wrapf(err, "preview is inactive after switching to mode %s", mode)
	}
	// Owing to the mode retry mechanism in CCA, it may fallback to other mode when failing to
	// switch to specified mode. Verify the mode value again after switching.
	if active, err := a.GetState(ctx, string(mode)); err != nil {
		return errors.Wrapf(err, "failed to get mode state after switching to mode %s", mode)
	} else if !active {
		return errors.Wrapf(err, "failed to switch to mode %s", mode)
	}
	return nil
}

// WaitForState waits until state become active/inactive.
func (a *App) WaitForState(ctx context.Context, state string, active bool) error {
	code := fmt.Sprintf("cca.state.get(%q) === %t", state, active)
	if err := a.conn.WaitForExpr(ctx, code); err != nil {
		return errors.Wrapf(err, "failed to wait for state %s to set to %v", state, active)
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
