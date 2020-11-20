// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cca provides utilities to interact with Chrome Camera App.
package cca

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
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
	// FacingExternal is the constant string indicating external camera facing.
	FacingExternal = "external"
)

// DeviceID is video device id from JavaScript navigator.mediaDevices.enumerateDevices.
type DeviceID string

// Mode is capture mode in CCA.
type Mode string

const (
	// ID is the app id of CCA.
	ID string = "hfhhnacclhffhdffklopdkcgdhifgngh"

	// Video is the mode used to record video.
	Video Mode = "video"
	// Photo is the mode used to take photo.
	Photo = "photo"
	// Square is the mode used to take square photo.
	Square = "square"
	// Portrait is the mode used to take portrait photo.
	Portrait = "portrait"

	// Expert is the state used to indicate expert mode.
	Expert string = "expert"
	// SaveMetadata is the state used to indicate save metadata.
	SaveMetadata = "save-metadata"
)

// TimerState is the information of whether shutter timer is on.
type TimerState bool

const (
	// TimerOn means shutter timer is on.
	TimerOn TimerState = true
	// TimerOff means shutter timer is off.
	TimerOff = false
)

var (
	// PhotoPattern is the filename format of photos taken by CCA.
	PhotoPattern = regexp.MustCompile(`^IMG_\d{8}_\d{6}[^.]*\.jpg$`)
	// VideoPattern is the filename format of videos recorded by CCA.
	VideoPattern = regexp.MustCompile(`^VID_\d{8}_\d{6}[^.]*\.(mkv|mp4)$`)
	// PortraitPattern is the filename format of portrait-mode photos taken by CCA.
	PortraitPattern = regexp.MustCompile(`^IMG_\d{8}_\d{6}[^.]*\_BURST\d{5}_COVER.jpg$`)
	// PortraitRefPattern is the filename format of the reference photo captured in portrait-mode.
	PortraitRefPattern = regexp.MustCompile(`^IMG_\d{8}_\d{6}[^.]*\_BURST\d{5}.jpg$`)
	// ErrVideoNotActive indicates that video is not active.
	ErrVideoNotActive = "Video is not active within given time"
)

// Orientation is the screen orientation from JavaScript window.screen.orientation.type.
type Orientation string

const (
	// PortraitPrimary is the primary portrait orientation.
	PortraitPrimary Orientation = "portrait-primary"
	// PortraitSecondary is the secondary portrait orientation.
	PortraitSecondary Orientation = "portrait-secondary"
	// LandscapePrimary is the primary landscape orientation.
	LandscapePrimary Orientation = "landscape-primary"
	// LandscapeSecondary is the secondary landscape orientation.
	LandscapeSecondary Orientation = "landscape-secondary"
)

// TimerDelay is default timer delay of CCA.
const TimerDelay time.Duration = 3 * time.Second

// UIComponent represents a CCA UI component.
type UIComponent struct {
	Name      string
	Selectors []string
}

var (
	// CancelResultButton is button for canceling intent review result.
	CancelResultButton = UIComponent{"cancel result button", []string{"#cancel-result"}}
	// ConfirmResultButton is button for confirming intent review result.
	ConfirmResultButton = UIComponent{"confirm result button", []string{"#confirm-result"}}
	// MirrorButton is button used for toggling preview mirroring option.
	MirrorButton = UIComponent{"mirror button", []string{"#toggle-mirror"}}
	// ModeSelector is selection bar for different capture modes.
	ModeSelector = UIComponent{"mode selector", []string{"#modes-group"}}
	// SettingsButton is button for opening primary setting menu.
	SettingsButton = UIComponent{"settings", []string{"#open-settings"}}
	// SwitchDeviceButton is button for switching camera device.
	SwitchDeviceButton = UIComponent{"switch device button", []string{"#switch-device"}}
	// VideoSnapshotButton is button for taking video snapshot during recording.
	VideoSnapshotButton = UIComponent{"video snapshot button", []string{"#video-snapshot"}}
	// VideoPauseResumeButton is button for pausing or resuming video recording.
	VideoPauseResumeButton = UIComponent{"video pause/resume button", []string{"#pause-recordvideo"}}

	// SettingsBackButton is back button for closing primary setting menu.
	SettingsBackButton = UIComponent{"settings back button", []string{
		"#view-settings .menu-header button"}}
	// GridSettingBackButton is back button for closing grid setting menu.
	GridSettingBackButton = UIComponent{"grid setting back button", []string{
		"#view-grid-settings .menu-header button"}}
	// TimerSettingBackButton is back button for closing timer setting menu.
	TimerSettingBackButton = UIComponent{"timer setting back button", []string{
		"#view-timer-settings .menu-header button"}}
	// ResolutionSettingButton is button for opening resolution setting menu.
	ResolutionSettingButton = UIComponent{"resolution setting button", []string{"#settings-resolution"}}
	// ResolutionSettingBackButton is back button for closing resolution setting menu.
	ResolutionSettingBackButton = UIComponent{"resolution setting back button", []string{
		"#view-resolution-settings .menu-header button"}}
	// ExpertModeButton is button used for opening expert mode setting menu.
	ExpertModeButton = UIComponent{"expert mode button", []string{"#settings-expert"}}
	// PhotoResolutionSettingBackButton is back button for closing photo resolution setting menu.
	PhotoResolutionSettingBackButton = UIComponent{"photo resolution setting back button",
		[]string{"#view-photo-resolution-settings .menu-header button"}}
	// VideoResolutionSettingBackButton is back button for closing video resolution setting menu.
	VideoResolutionSettingBackButton = UIComponent{"video resolution setting back button",
		[]string{"#view-video-resolution-settings .menu-header button"}}
	// PhotoResolutionOption is option for each available photo capture resolution.
	PhotoResolutionOption = UIComponent{"photo resolution option", []string{
		"#view-photo-resolution-settings input"}}
	// VideoResolutionOption is option for each available video capture resolution.
	VideoResolutionOption = UIComponent{"video resolution option", []string{
		"#view-video-resolution-settings input"}}
)

// ResolutionType is different capture resolution type.
type ResolutionType string

const (
	// PhotoResolution represents photo resolution type.
	PhotoResolution ResolutionType = "photo"
	// VideoResolution represents video resolution type.
	VideoResolution = "video"
)

// App represents a CCA (Chrome Camera App) instance.
type App struct {
	conn        *chrome.Conn
	cr          *chrome.Chrome
	scriptPaths []string
	outDir      string // Output directory to save the execution result
	appLauncher testutil.AppLauncher
	appWindow   *testutil.AppWindow
}

// ErrJS represents an error occurs when executing JavaScript.
type ErrJS struct {
	msg string
}

// Error returns the wrapped message of a ErrJS.
func (e *ErrJS) Error() string {
	return e.msg
}

// Resolution represents dimension of video or photo.
type Resolution struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// AspectRatio returns width divided by height as the aspect ratio of the resolution.
func (r *Resolution) AspectRatio() float64 {
	return float64(r.Width) / float64(r.Height)
}

// Init launches a CCA instance, evaluates the helper script within it and waits
// until its AppWindow interactable. The scriptPath should be the data path to
// the helper script cca_ui.js. The returned App instance must be closed when
// the test is finished.
func Init(ctx context.Context, cr *chrome.Chrome, scriptPaths []string, outDir string, appLauncher testutil.AppLauncher, tb *testutil.TestBridge, useSWA bool) (*App, error) {
	// The cros-camera job exists only on boards that use the new camera stack.
	if upstart.JobExists(ctx, "cros-camera") {
		// Ensure that cros-camera service is running, because the service
		// might stopped due to the errors from some previous tests, and failed
		// to restart for some reasons.
		if err := upstart.EnsureJobRunning(ctx, "cros-camera"); err != nil {
			return nil, err
		}
	}

	conn, appWindow, err := testutil.LaunchApp(ctx, cr, tb, appLauncher)
	if err != nil {
		return nil, errors.Wrap(err, "failed when launching app")
	}

	if err := func() error {
		// Let CCA perform some one-time initialization after launched.  Otherwise
		// the first CheckVideoActive() might timed out because it's still
		// initializing, especially on low-end devices and when the system is busy.
		// Fail the test early if it's timed out to make it easier to figure out
		// the real reason of a test failure.
		if err := conn.Eval(ctx, `(async () => {
			const deadline = await new Promise(
				(resolve) => requestIdleCallback(resolve, {timeout: 30000}));
			if (deadline.didTimeout) {
			throw new Error('Timed out initializing CCA');
			}
		})()`, nil); err != nil {
			return err
		}

		for _, scriptPath := range scriptPaths {
			script, err := ioutil.ReadFile(scriptPath)
			if err != nil {
				return err
			}
			if err := conn.Eval(ctx, string(script), nil); err != nil {
				return err
			}
		}
		testing.ContextLog(ctx, "CCA launched")
		return nil
	}(); err != nil {
		if closeErr := conn.CloseTarget(ctx); closeErr != nil {
			testing.ContextLog(ctx, "Failed to close app: ", closeErr)
		}
		if closeErr := conn.Close(); closeErr != nil {
			testing.ContextLog(ctx, "Failed to close app connection: ", closeErr)
		}
		if releaseErr := appWindow.Release(ctx); releaseErr != nil {
			testing.ContextLog(ctx, "Failed to release app window: ", releaseErr)
		}
		return nil, err
	}

	app := &App{conn, cr, scriptPaths, outDir, appLauncher, appWindow}
	waitForWindowReady := func() error {
		if err := app.WaitForVideoActive(ctx); err != nil {
			return errors.Wrap(err, ErrVideoNotActive)
		}
		return app.WaitForState(ctx, "view-camera", true)
	}
	if err := waitForWindowReady(); err != nil {
		if err2 := app.Close(ctx); err2 != nil {
			testing.ContextLog(ctx, "Failed to close app: ", err2)
		}
		return nil, errors.Wrap(err, "CCA window is not ready after launching app")
	}
	testing.ContextLog(ctx, "CCA window is ready")
	return app, nil
}

// New launches a CCA instance. The returned App instance must be closed when the test is finished.
func New(ctx context.Context, cr *chrome.Chrome, scriptPaths []string, outDir string, tb *testutil.TestBridge, useSWA bool) (*App, error) {
	launcher := testutil.PlatformAppLauncher()
	if useSWA {
		launcher = testutil.SWALauncher()
	}
	return Init(ctx, cr, scriptPaths, outDir, launcher, tb, useSWA)
}

// InstanceExists checks if there is any running CCA instance.
func InstanceExists(ctx context.Context, cr *chrome.Chrome, useSWA bool) (bool, error) {
	checkPrefix := func(t *target.Info) bool {
		url := fmt.Sprintf("chrome-extension://%s/views/main.html", ID)
		if useSWA {
			url = "chrome://camera-app/views/main.html"
		}
		return strings.HasPrefix(t.URL, url)
	}
	return cr.IsTargetAvailable(ctx, checkPrefix)
}

// ClosingItself checks if CCA intends to close itself.
func (a *App) ClosingItself(ctx context.Context) (bool, error) {
	return a.appWindow.ClosingItself(ctx)
}

// checkJSError checks javascript error emitted by CCA error callback.
func (a *App) checkJSError(ctx context.Context) error {
	if a.appWindow == nil {
		// It might be closed already. Do nothing.
		return nil
	}
	errorInfos, err := a.appWindow.Errors(ctx)
	if err != nil {
		return err
	}

	jsErrors := make([]testutil.ErrorInfo, 0)
	jsWarnings := make([]testutil.ErrorInfo, 0)
	for _, err := range errorInfos {
		if err.Level == testutil.ErrorLevelWarning {
			jsWarnings = append(jsWarnings, err)
		} else if err.Level == testutil.ErrorLevelError {
			jsErrors = append(jsErrors, err)
		} else {
			return errors.Errorf("unknown error level: %v", err.Level)
		}
	}

	writeLogFile := func(lv testutil.ErrorLevel, errs []testutil.ErrorInfo) error {
		filename := fmt.Sprintf("CCA_JS_%v.log", lv)
		logPath := filepath.Join(a.outDir, filename)
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer f.Close()
		for _, err := range errs {
			t := time.Unix(0, err.Time*1e6).Format("2006/01/02 15:04:05 [15:04:05.000]")
			f.WriteString(fmt.Sprintf("%v %v:\n", t, err.ErrorType))
			f.WriteString(err.Stack + "\n")
		}
		return nil
	}

	if err := writeLogFile(testutil.ErrorLevelWarning, jsWarnings); err != nil {
		return err
	}
	if err := writeLogFile(testutil.ErrorLevelError, jsErrors); err != nil {
		return err
	}
	if len(jsErrors) > 0 {
		return &ErrJS{fmt.Sprintf("there are %d JS errors, first error: %v: %v",
			len(jsErrors), jsErrors[0].Level, jsErrors[0].ErrorType)}
	}
	return nil
}

// Close closes the App and the associated connection.gi
func (a *App) Close(ctx context.Context) (retErr error) {
	if a.conn == nil {
		// It's already closed. Do nothing.
		return nil
	}

	defer func(ctx context.Context) {
		reportOrLogError := func(err error) {
			if retErr == nil {
				retErr = err
			} else {
				testing.ContextLog(ctx, "Failed to close app: ", err)
			}
		}

		if err := a.conn.CloseTarget(ctx); err != nil {
			reportOrLogError(errors.Wrap(err, "failed to CloseTarget()"))
		}
		if err := a.conn.Close(); err != nil {
			reportOrLogError(errors.Wrap(err, "failed to Conn.Close()"))
		}
		if err := a.appWindow.WaitUntilClosed(ctx); err != nil {
			reportOrLogError(errors.Wrap(err, "failed to wait for appWindow close"))
		}
		if err := a.checkJSError(ctx); err != nil {
			reportOrLogError(errors.Wrap(err, "There are JS errors when running CCA"))
		}
		if err := a.appWindow.Release(ctx); err != nil {
			reportOrLogError(errors.Wrap(err, "failed to release app window"))
		}

		testing.ContextLog(ctx, "CCA closed")
		a.conn = nil
		a.appWindow = nil
	}(ctx)

	if err := a.conn.Eval(ctx, "Tast.removeCacheData()", nil); err != nil {
		return errors.Wrap(err, "failed to clear cached data in local storage")
	}

	// TODO(b/144747002): Some tests (e.g. CCUIIntent) might trigger auto closing of CCA before
	// calling Close(). We should handle it gracefully to get the coverage report for them.
	err := a.OutputCodeCoverage(ctx)
	if err != nil {
		return err
	}
	return nil
}

// Restart restarts the App and resets the associated connection.
func (a *App) Restart(ctx context.Context, tb *testutil.TestBridge, useSWA bool) error {
	if err := a.Close(ctx); err != nil {
		return err
	}
	newApp, err := Init(ctx, a.cr, a.scriptPaths, a.outDir, a.appLauncher, tb, useSWA)
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
	// duration here and check that the state does not change afterwards.
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

// IsWindowMinimized returns true if the current app window is minimized.
func (a *App) IsWindowMinimized(ctx context.Context) (bool, error) {
	var isMinimized bool
	err := a.conn.Eval(ctx, "Tast.isMinimized()", &isMinimized)
	return isMinimized, err
}

// WaitForVideoActive waits for the video to become active for 1 second.
func (a *App) WaitForVideoActive(ctx context.Context) error {
	return a.checkVideoState(ctx, true, time.Second)
}

// WaitForFileSaved waits for the presence of the captured file with file name matching the specified
// pattern, size larger than zero, and modified time after the specified timestamp.
func (a *App) WaitForFileSaved(ctx context.Context, dirs []string, pat *regexp.Regexp, ts time.Time) (os.FileInfo, error) {
	const timeout = 5 * time.Second
	var result os.FileInfo
	seen := make(map[string]struct{})
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var allFiles []os.FileInfo
		var lastErr error
		for _, dir := range dirs {
			files, err := ioutil.ReadDir(dir)
			if err != nil {
				lastErr = err
				continue
			}
			allFiles = append(allFiles, files...)
		}
		if lastErr != nil && len(allFiles) == 0 {
			return errors.Wrap(lastErr, "failed to read the directory where media files are saved")
		}

		for _, file := range allFiles {
			if file.Size() == 0 || file.ModTime().Before(ts) {
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
	return a.conn.Eval(ctx, "Tast.restoreWindow()", nil)
}

// MinimizeWindow minimizes the window.
func (a *App) MinimizeWindow(ctx context.Context) error {
	return a.conn.Eval(ctx, "Tast.minimizeWindow()", nil)
}

// MaximizeWindow maximizes the window.
func (a *App) MaximizeWindow(ctx context.Context) error {
	return a.conn.Eval(ctx, "Tast.maximizeWindow()", nil)
}

// FullscreenWindow fullscreens the window.
func (a *App) FullscreenWindow(ctx context.Context) error {
	return a.conn.Eval(ctx, "Tast.fullscreenWindow()", nil)
}

// GetNumOfCameras returns number of camera devices.
func (a *App) GetNumOfCameras(ctx context.Context) (int, error) {
	var numCameras int
	err := a.conn.Eval(ctx, "Tast.getNumOfCameras()", &numCameras)
	return numCameras, err
}

// GetFacing returns the active camera facing.
func (a *App) GetFacing(ctx context.Context) (Facing, error) {
	var facing Facing
	if err := a.conn.Eval(ctx, "Tast.getFacing()", &facing); err != nil {
		return "", err
	}
	return facing, nil
}

// GetPreviewResolution returns resolution of preview video.
func (a *App) GetPreviewResolution(ctx context.Context) (Resolution, error) {
	r := Resolution{-1, -1}
	if err := a.conn.Eval(ctx, "Tast.getPreviewResolution()", &r); err != nil {
		return r, errors.Wrap(err, "failed to get preview resolution")
	}
	return r, nil
}

// GetScreenOrientation returns screen orientation.
func (a *App) GetScreenOrientation(ctx context.Context) (Orientation, error) {
	var orientation Orientation
	if err := a.conn.Eval(ctx, "Tast.getScreenOrientation()", &orientation); err != nil {
		return "", errors.Wrap(err, "failed to get screen orientation")
	}
	return orientation, nil
}

// GetPhotoResolutions returns available photo resolutions of active camera on HALv3 device.
func (a *App) GetPhotoResolutions(ctx context.Context) ([]Resolution, error) {
	var rs []Resolution
	if err := a.conn.Eval(ctx, "Tast.getPhotoResolutions()", &rs); err != nil {
		return nil, errors.Wrap(err, "failed to get photo resolution")
	}
	return rs, nil
}

// GetVideoResolutions returns available video resolutions of active camera on HALv3 device.
func (a *App) GetVideoResolutions(ctx context.Context) ([]Resolution, error) {
	var rs []Resolution
	if err := a.conn.Eval(ctx, "Tast.getVideoResolutions()", &rs); err != nil {
		return nil, errors.Wrap(err, "failed to get video resolution")
	}
	return rs, nil
}

// GetDeviceID returns the active camera device id.
func (a *App) GetDeviceID(ctx context.Context) (DeviceID, error) {
	var id DeviceID
	if err := a.conn.Eval(ctx, "Tast.getDeviceId()", &id); err != nil {
		return "", err
	}
	return id, nil
}

// GetState returns whether a state is active in CCA.
func (a *App) GetState(ctx context.Context, state string) (bool, error) {
	var result bool
	if err := a.conn.Call(ctx, &result, "Tast.getState", state); err != nil {
		return false, errors.Wrapf(err, "failed to get state: %v", state)
	}
	return result, nil
}

// PortraitModeSupported returns whether portrait mode is supported by the current active video device.
func (a *App) PortraitModeSupported(ctx context.Context) (bool, error) {
	var result bool
	if err := a.conn.Eval(ctx, "Tast.isPortraitModeSupported()", &result); err != nil {
		return false, err
	}
	return result, nil
}

// TakeSinglePhoto takes a photo and save to default location.
func (a *App) TakeSinglePhoto(ctx context.Context, timerState TimerState) ([]os.FileInfo, error) {
	var patterns []*regexp.Regexp

	isPortrait, err := a.GetState(ctx, string(Portrait))
	if err != nil {
		return nil, err
	}
	if isPortrait {
		patterns = append(patterns, PortraitRefPattern, PortraitPattern)
	} else {
		patterns = append(patterns, PhotoPattern)
	}

	if err = a.SetTimerOption(ctx, timerState); err != nil {
		return nil, err
	}
	start := time.Now()

	testing.ContextLog(ctx, "Click on start shutter")
	if err = a.ClickShutter(ctx); err != nil {
		return nil, err
	}
	if err = a.WaitForState(ctx, "taking", false); err != nil {
		return nil, errors.Wrap(err, "capturing hasn't ended")
	}

	dirs, err := a.SavedDirs(ctx)
	if err != nil {
		return nil, err
	}

	var fileInfos []os.FileInfo
	for _, pattern := range patterns {
		info, err := a.WaitForFileSaved(ctx, dirs, pattern, start)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot find result picture with regexp: %v", pattern)
		}
		if elapsed := info.ModTime().Sub(start); timerState == TimerOn && elapsed < TimerDelay {
			return nil, errors.Errorf("the capture should happen after timer of %v, actual elapsed time %v", TimerDelay, elapsed)
		}
		fileInfos = append(fileInfos, info)
	}

	isExpert, err := a.GetState(ctx, Expert)
	if err != nil {
		return nil, err
	}
	isSaveMetadata, err := a.GetState(ctx, SaveMetadata)
	if err != nil {
		return nil, err
	}
	if !isExpert || !isSaveMetadata {
		return fileInfos, nil
	}

	metadataPatterns := getMetadataPatterns(fileInfos)
	for _, pattern := range metadataPatterns {
		info, err := a.WaitForFileSaved(ctx, dirs, pattern, start)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot find result metadata with regexp: %v", pattern)
		}

		if info.Size() == 0 {
			return nil, errors.Errorf("saved file %v is empty", info.Name())
		}

		if err != nil {
			return nil, err
		}

		path, err := a.FilePathInSavedDirs(ctx, info.Name())
		if err != nil {
			return nil, err
		}

		var jsonString map[string]interface{}
		if content, err := ioutil.ReadFile(path); err != nil {
			return nil, errors.Wrapf(err, "failed to read metadata file %v", info.Name())
		} else if err := json.Unmarshal(content, &jsonString); err != nil {
			return nil, errors.Wrapf(err, "not a valid json file %v", info.Name())
		}

		fileInfos = append(fileInfos, info)
	}

	return fileInfos, nil
}

func getMetadataPatterns(fileInfos []os.FileInfo) []*regexp.Regexp {
	// Matches the extension and the potential number suffix such as " (2).jpg".
	re := regexp.MustCompile(`( \(\d+\))?\.jpg$`)
	var patterns []*regexp.Regexp
	for _, info := range fileInfos {
		pattern := `^` + regexp.QuoteMeta(re.ReplaceAllString(info.Name(), "")) + `.*\.json$`
		patterns = append(patterns, regexp.MustCompile(pattern))
	}
	return patterns
}

// StartRecording starts recording a video.
func (a *App) StartRecording(ctx context.Context, timerState TimerState) (time.Time, error) {
	startTime := time.Now()
	if err := a.SetTimerOption(ctx, timerState); err != nil {
		return startTime, err
	}
	testing.ContextLog(ctx, "Click on start shutter")
	if err := a.ClickShutter(ctx); err != nil {
		return startTime, err
	}

	// Wait for end of timer and start of recording.
	if err := a.WaitForState(ctx, "recording", true); err != nil {
		return startTime, errors.Wrap(err, "recording is not started")
	}
	recordStartTime := time.Now()
	if timerState == TimerOn {
		// Assume that the delay between recording state being set and
		// time.Now() getting timestamp is small enough to be
		// neglected. Otherwise, it may miss the failure cases if
		// |autual recording start time|+ |this delay| > |startTime| +
		// |TimerDelay|.
		if delay := recordStartTime.Sub(startTime); delay < TimerDelay {
			return startTime, errors.Errorf("recording starts %v before timer finished", TimerDelay-delay)
		}
	}

	return startTime, nil
}

// StopRecording stops recording a video.
func (a *App) StopRecording(ctx context.Context, timerState TimerState, startTime time.Time) (os.FileInfo, time.Time, error) {
	testing.ContextLog(ctx, "Click on stop shutter")
	stopTime, err := a.TriggerStateChange(ctx, "recording", false, func() error {
		return a.ClickShutter(ctx)
	})
	if err != nil {
		return nil, time.Time{}, err
	}
	if err := a.WaitForState(ctx, "taking", false); err != nil {
		return nil, time.Time{}, errors.Wrap(err, "shutter is not ended")
	}
	dirs, err := a.SavedDirs(ctx)
	if err != nil {
		return nil, time.Time{}, err
	}
	info, err := a.WaitForFileSaved(ctx, dirs, VideoPattern, startTime)
	if err != nil {
		return nil, time.Time{}, errors.Wrap(err, "cannot find result video")
	} else if elapsed := info.ModTime().Sub(startTime); timerState == TimerOn && elapsed < TimerDelay {
		return nil, time.Time{}, errors.Errorf("the capture happen after elapsed time %v, should be after %v timer", elapsed, TimerDelay)
	}
	return info, stopTime, nil
}

// RecordVideo records a video with duration length and save to default location.
func (a *App) RecordVideo(ctx context.Context, timerState TimerState, duration time.Duration) (os.FileInfo, error) {
	startTime, err := a.StartRecording(ctx, timerState)
	if err != nil {
		return nil, err
	}

	if err := testing.Sleep(ctx, duration); err != nil {
		return nil, err
	}

	info, _, err := a.StopRecording(ctx, timerState, startTime)
	if err != nil {
		return nil, err
	}
	return info, err
}

// savedDirs returns the paths to the folder where captured files might be saved.
func savedDirs(ctx context.Context, cr *chrome.Chrome) ([]string, error) {
	path, err := cryptohome.UserPath(ctx, cr.User())
	if err != nil {
		return nil, err
	}
	myFiles := filepath.Join(path, "MyFiles")
	return []string{filepath.Join(myFiles, "Downloads"), filepath.Join(myFiles, "Camera")}, nil
}

// ClearSavedDirs clears all files in the folders where captured files might be saved.
func ClearSavedDirs(ctx context.Context, cr *chrome.Chrome) error {
	clearDir := func(ctx context.Context, cr *chrome.Chrome, dir string) error {
		files, err := ioutil.ReadDir(dir)
		if err != nil {
			return errors.Wrap(err, "failed to read saved directory")
		}

		// Pattern for metadata files of all different kinds of photos.
		metadataPattern := regexp.MustCompile(`^IMG_\d{8}_\d{6}.*\.json$`)
		capturedPatterns := []*regexp.Regexp{PhotoPattern, VideoPattern, PortraitPattern, PortraitRefPattern, metadataPattern}
		for _, file := range files {
			for _, pat := range capturedPatterns {
				if pat.MatchString(file.Name()) {
					path := filepath.Join(dir, file.Name())
					if err := os.Remove(path); err != nil {
						return errors.Wrapf(err, "failed to remove file %v from saved directory", path)
					}
					break
				}
			}
		}

		return nil
	}

	dirs, err := savedDirs(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to get saved directorys")
	}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); err != nil {
			if os.IsNotExist(err) {
				continue
			} else {
				return err
			}
		}

		if err := clearDir(ctx, cr, dir); err != nil {
			return err
		}
	}
	return nil
}

// SavedDirs returns the path to the folder where captured files are saved.
func (a *App) SavedDirs(ctx context.Context) ([]string, error) {
	return savedDirs(ctx, a.cr)
}

// FilePathInSavedDirs finds and returns the path of the target file in saved directories.
func (a *App) FilePathInSavedDirs(ctx context.Context, name string) (string, error) {
	dirs, err := savedDirs(ctx, a.cr)
	if err != nil {
		return "", err
	}

	for _, dir := range dirs {
		path := filepath.Join(dir, name)
		_, err := os.Stat(path)
		if err == nil {
			return path, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
	}
	return "", errors.New("file not found in saved path")
}

// CheckFacing returns an error if the active camera facing is not expected.
func (a *App) CheckFacing(ctx context.Context, expected Facing) error {
	return a.conn.Call(ctx, nil, "Tast.checkFacing", expected)
}

// Mirrored returns whether mirroring is on.
func (a *App) Mirrored(ctx context.Context) (bool, error) {
	var actual bool
	err := a.conn.Eval(ctx, "Tast.getState('mirror')", &actual)
	return actual, err
}

func (a *App) selectorExist(ctx context.Context, selector string) (bool, error) {
	var exist bool
	err := a.conn.Call(ctx, &exist, "Tast.isExist", selector)
	return exist, err
}

// resolveUISelector resolves ui to its correct selector.
func (a *App) resolveUISelector(ctx context.Context, ui UIComponent) (string, error) {
	for _, s := range ui.Selectors {
		if exist, err := a.selectorExist(ctx, s); err != nil {
			return "", err
		} else if exist {
			return s, nil
		}
	}
	return "", errors.Errorf("failed to resolved ui %v to its correct selector", ui.Name)
}

// Visible returns whether a UI component is visible on the screen.
func (a *App) Visible(ctx context.Context, ui UIComponent) (bool, error) {
	wrapError := func(err error) error {
		return errors.Wrapf(err, "failed to check visibility state of %v", ui.Name)
	}
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return false, wrapError(err)
	}
	var visible bool
	if err := a.conn.Call(ctx, &visible, "Tast.isVisible", selector); err != nil {
		return false, wrapError(err)
	}
	return visible, nil
}

// CheckVisible returns an error if visibility state of ui is not expected.
func (a *App) CheckVisible(ctx context.Context, ui UIComponent, expected bool) error {
	if visible, err := a.Visible(ctx, ui); err != nil {
		return err
	} else if visible != expected {
		return errors.Errorf("unexpected %v visibility state: got %v, want %v", ui.Name, visible, expected)
	}
	return nil
}

// CheckConfirmUIExists returns whether the confirm UI exists.
func (a *App) CheckConfirmUIExists(ctx context.Context, mode Mode) error {
	var reviewElementID string
	if mode == Photo {
		reviewElementID = "#review-photo-result"
	} else if mode == Video {
		reviewElementID = "#review-video-result"
	} else {
		return errors.Errorf("unrecognized mode: %s", mode)
	}
	var visible bool
	if err := a.conn.Call(ctx, &visible, "Tast.isVisible", reviewElementID); err != nil {
		return err
	} else if !visible {
		return errors.New("review result is not shown")
	}

	if visible, err := a.Visible(ctx, ConfirmResultButton); err != nil {
		return err
	} else if !visible {
		return errors.New("confirm button is not shown")
	}

	if visible, err := a.Visible(ctx, CancelResultButton); err != nil {
		return err
	} else if !visible {
		return errors.New("cancel button is not shown")
	}
	return nil
}

// CountUI returns the number of ui element.
func (a *App) CountUI(ctx context.Context, ui UIComponent) (int, error) {
	wrapError := func(err error) error {
		return errors.Wrapf(err, "failed to count number of %v", ui.Name)
	}
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return 0, wrapError(err)
	}
	var number int
	if err := a.conn.Call(ctx, &number, `(selector) => document.querySelectorAll(selector).length`, selector); err != nil {
		return 0, wrapError(err)
	}
	return number, nil
}

// AttributeWithIndex returns the attr attribute of the index th ui.
func (a *App) AttributeWithIndex(ctx context.Context, ui UIComponent, index int, attr string) (string, error) {
	wrapError := func(err error) error {
		return errors.Wrapf(err, "failed to get %v attribute of %v th %v", attr, index, ui.Name)
	}
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return "", wrapError(err)
	}
	var value string
	if err := a.conn.Call(
		ctx, &value,
		`(selector, index, attr) => document.querySelectorAll(selector)[index].getAttribute(attr)`,
		selector, index, attr); err != nil {
		return "", wrapError(err)
	}
	return value, nil
}

// ConfirmResult clicks the confirm button or the cancel button according to the given isConfirmed.
func (a *App) ConfirmResult(ctx context.Context, isConfirmed bool, mode Mode) error {
	if err := a.WaitForState(ctx, "review-result", true); err != nil {
		return errors.Wrap(err, "does not enter review result state")
	}
	if err := a.CheckConfirmUIExists(ctx, mode); err != nil {
		return errors.Wrap(err, "check confirm UI failed")
	}

	var expr string
	if isConfirmed {
		// TODO(b/144547749): Since CCA will close automatically after clicking the button, sometimes it
		// will report connection lost error when executing. Removed the setTimeout wrapping once the
		// flakiness got resolved.
		expr = "setTimeout(() => Tast.click('#confirm-result'), 0)"
	} else {
		expr = "Tast.click('#cancel-result')"
	}
	if err := a.conn.Eval(ctx, expr, nil); err != nil {
		return errors.Wrap(err, "failed to click confirm/cancel button")
	}
	return nil
}

func (a *App) toggleOption(ctx context.Context, option, toggleSelector string) (bool, error) {
	prev, err := a.GetState(ctx, option)
	if err != nil {
		return false, err
	}
	if err := a.ClickWithSelector(ctx, toggleSelector); err != nil {
		return false, errors.Wrapf(err, "failed to click on toggle button of selector %s", toggleSelector)
	}
	code := fmt.Sprintf("Tast.getState(%q) !== %t", option, prev)
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

// SetTimerOption sets the timer option to on/off.
func (a *App) SetTimerOption(ctx context.Context, state TimerState) error {
	active := state == TimerOn
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

// ToggleExpertMode toggles expert mode and returns whether it's enabled after toggling.
func (a *App) ToggleExpertMode(ctx context.Context) (bool, error) {
	prev, err := a.GetState(ctx, Expert)
	if err != nil {
		return false, err
	}
	if err := a.conn.Eval(ctx, "Tast.toggleExpertMode()", nil); err != nil {
		return false, errors.Wrap(err, "failed to toggle expert mode")
	}
	if err := a.WaitForState(ctx, "expert", !prev); err != nil {
		return false, errors.Wrap(err, "failed to wait for toggling expert mode")
	}
	return a.GetState(ctx, Expert)
}

// CheckMetadataVisibility checks if metadata is shown/hidden on screen given enabled.
func (a *App) CheckMetadataVisibility(ctx context.Context, enabled bool) error {
	code := fmt.Sprintf("Tast.isVisible('#preview-exposure-time') === %t", enabled)
	if err := a.conn.WaitForExpr(ctx, code); err != nil {
		return errors.Wrapf(err, "failed to wait for metadata visibility set to %v", enabled)
	}
	return nil
}

// ToggleShowMetadata toggles show metadata and returns whether it's enabled after toggling.
func (a *App) ToggleShowMetadata(ctx context.Context) (bool, error) {
	return a.toggleOption(ctx, "show-metadata", "#expert-show-metadata")
}

// ToggleSaveMetadata toggles save metadata and returns whether it's enabled after toggling.
func (a *App) ToggleSaveMetadata(ctx context.Context) (bool, error) {
	return a.toggleOption(ctx, "save-metadata", "#expert-save-metadata")
}

// ClickShutter clicks the shutter button.
func (a *App) ClickShutter(ctx context.Context) error {
	if err := a.conn.Eval(ctx, "Tast.click('.shutter')", nil); err != nil {
		return errors.Wrap(err, "failed to click shutter button")
	}
	return nil
}

// SwitchCamera switches to next camera device.
func (a *App) SwitchCamera(ctx context.Context) error {
	return a.conn.Eval(ctx, "Tast.switchCamera()", nil)
}

// SwitchMode switches to specified capture mode.
func (a *App) SwitchMode(ctx context.Context, mode Mode) error {
	if active, err := a.GetState(ctx, string(mode)); err != nil {
		return err
	} else if active {
		return nil
	}
	if err := a.conn.Call(ctx, nil, "Tast.switchMode", mode); err != nil {
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
	code := fmt.Sprintf("Tast.getState(%q) === %t", state, active)
	if err := a.conn.WaitForExpr(ctx, code); err != nil {
		return errors.Wrapf(err, "failed to wait for state %s to set to %v", state, active)
	}
	return nil
}

// WaitForMinimized waits for app window to be minimized/restored.
func (a *App) WaitForMinimized(ctx context.Context, minimized bool) error {
	const timeout = 5 * time.Second
	return testing.Poll(ctx, func(ctx context.Context) error {
		actual, err := a.IsWindowMinimized(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to check if window is minimized"))
		}
		if actual != minimized {
			return errors.New("failed to wait for window minimized/restored")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

// CheckGridOption checks whether grid option enable state is as expected.
func (a *App) CheckGridOption(ctx context.Context, expected bool) error {
	var actual bool
	if err := a.conn.Eval(ctx, "Tast.getState('grid')", &actual); err != nil {
		return err
	}
	if actual != expected {
		return errors.Errorf("unexpected grid option enablement: got %v, want %v", actual, expected)
	}
	return nil
}

// Click clicks on ui.
func (a *App) Click(ctx context.Context, ui UIComponent) error {
	wrapError := func(err error) error {
		return errors.Wrapf(err, "failed to click on %v", ui.Name)
	}
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return wrapError(err)
	}
	if err := a.ClickWithSelector(ctx, selector); err != nil {
		return wrapError(err)
	}
	return nil
}

// ClickWithIndex clicks nth ui.
func (a *App) ClickWithIndex(ctx context.Context, ui UIComponent, index int) error {
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return err
	}
	if err := a.conn.Call(ctx, nil, `(selector, index) => document.querySelectorAll(selector)[index].click()`, selector, index); err != nil {
		return errors.Wrapf(err, "failed to click on %v(th) %v", index, ui.Name)
	}
	return nil
}

// IsCheckedWithIndex gets checked state of nth ui.
func (a *App) IsCheckedWithIndex(ctx context.Context, ui UIComponent, index int) (bool, error) {
	selector, err := a.resolveUISelector(ctx, ui)
	if err != nil {
		return false, err
	}
	var checked bool
	if err := a.conn.Call(ctx, &checked, `(selector, index) => document.querySelectorAll(selector)[index].checked`, selector, index); err != nil {
		return false, errors.Wrapf(err, "failed to get checked state on %v(th) %v", index, ui.Name)
	}
	return checked, nil
}

// ClickWithSelector clicks an element with given selector.
func (a *App) ClickWithSelector(ctx context.Context, selector string) error {
	return a.conn.Call(ctx, nil, `(selector) => document.querySelector(selector).click()`, selector)
}

// RunThroughCameras runs function f in app after switching to each available camera.
// The f is called with paramter of the switched camera facing.
// The error returned by f is passed to caller of this function.
func (a *App) RunThroughCameras(ctx context.Context, f func(Facing) error) error {
	numCameras, err := a.GetNumOfCameras(ctx)
	if err != nil {
		return errors.Wrap(err, "can't get number of cameras")
	}
	devices := make(map[DeviceID]Facing)
	for cam := 0; cam < numCameras; cam++ {
		if cam != 0 {
			if err := a.SwitchCamera(ctx); err != nil {
				return errors.Wrap(err, "failed to switch camera")
			}
		}
		id, err := a.GetDeviceID(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get device id")
		}
		facing, err := a.GetFacing(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get facing")
		}
		if _, ok := devices[id]; ok {
			continue
		}
		devices[id] = facing
		testing.ContextLogf(ctx, "Run f() on camera facing %q", facing)
		if err := f(facing); err != nil {
			return err
		}
	}
	if numCameras > 1 {
		// Switch back to the original camera.
		if err := a.SwitchCamera(ctx); err != nil {
			return errors.Wrap(err, "failed to switch to next camera")
		}
	}
	if len(devices) != numCameras {
		return errors.Errorf("failed to switch to some camera (tested cameras: %v)", devices)
	}
	return nil
}

// CheckMojoConnection checks if mojo connection works.
func (a *App) CheckMojoConnection(ctx context.Context) error {
	return a.conn.Call(ctx, nil, "Tast.checkMojoConnection", upstart.JobExists(ctx, "cros-camera"))
}

// OutputCodeCoverage stops the profiling and output the code coverage information to the output
// directory.
func (a *App) OutputCodeCoverage(ctx context.Context) error {
	reply, err := a.conn.StopProfiling(ctx)
	if err != nil {
		return err
	}

	coverageData, err := json.Marshal(reply)
	if err != nil {
		return err
	}

	coverageDirPath := filepath.Join(a.outDir, fmt.Sprintf("coverage"))
	if _, err := os.Stat(coverageDirPath); os.IsNotExist(err) {
		if err := os.MkdirAll(coverageDirPath, 0755); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	for idx := 0; ; idx++ {
		coverageFilePath := filepath.Join(coverageDirPath, fmt.Sprintf("coverage-%d.json", idx))
		if _, err := os.Stat(coverageFilePath); os.IsNotExist(err) {
			if err := ioutil.WriteFile(coverageFilePath, coverageData, 0644); err != nil {
				return err
			}
			break
		} else if err != nil {
			return err
		}
	}
	return nil
}

// TriggerConfiguration triggers configuration by calling trigger() and waits for camera configuration finishing.
func (a *App) TriggerConfiguration(ctx context.Context, trigger func() error) error {
	// waitNextConfiguration() returns a Promise instance, so Eval waits for its settled state.
	// For its workaround, wrap by a closure.
	var waiting chrome.JSObject
	if err := a.conn.Eval(ctx, `(p => () => p)(Tast.waitNextConfiguration())`, &waiting); err != nil {
		return errors.Wrap(err, "failed to start watching congiruation update")
	}
	defer waiting.Release(ctx)
	if err := trigger(); err != nil {
		return err
	}
	// And then unwrap the promise to wait its settled state.
	if err := a.conn.Call(ctx, nil, `(p) => p()`, &waiting); err != nil {
		return errors.Wrap(err, "failed to waiting for the completion configuration update")
	}
	return nil
}

// TriggerStateChange triggers |state| change by calling |trigger()|, waits for
// its value changing from |!expected| to |expected| and returns when the
// change happens.
func (a *App) TriggerStateChange(ctx context.Context, state string, expected bool, trigger func() error) (time.Time, error) {
	var wrappedPromise chrome.JSObject
	if err := a.conn.Call(ctx, &wrappedPromise, `
	  (state, expected) => {
		const p = Tast.observeStateChange(state, expected);
		return () => p;
	  }
	  `, state, expected); err != nil {
		return time.Time{}, errors.Wrapf(err, "failed to observe %v state with %v expected value", state, expected)
	}
	defer wrappedPromise.Release(ctx)

	if err := trigger(); err != nil {
		return time.Time{}, err
	}

	var ts int64
	if err := a.conn.Call(ctx, &ts, `(p) => p()`, &wrappedPromise); err != nil {
		return time.Time{}, errors.Wrapf(err, "failed to wait for %v state changing to %v", state, expected)
	}
	return time.Unix(0, ts*1e6), nil
}

// EnsureTabletModeEnabled makes sure that the tablet mode states of both
// device and app are enabled, and returns a function which reverts back to the
// original state.
func (a *App) EnsureTabletModeEnabled(ctx context.Context, enabled bool) (func(ctx context.Context) error, error) {
	tconn, err := a.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get test api connection")
	}

	originallyEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tablet mode state")
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, enabled)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to ensure tablet mode enabled(%v)", enabled)
	}

	cleanupAll := func(ctx context.Context) error {
		if err := cleanup(ctx); err != nil {
			return errors.Wrap(err, "failed to clean up tablet mode state")
		}
		if err := a.WaitForState(ctx, "tablet", originallyEnabled); err != nil {
			return errors.Wrapf(err, "failed to wait for original tablet mode enabled(%v)", originallyEnabled)
		}
		return nil
	}

	if err := a.WaitForState(ctx, "tablet", enabled); err != nil {
		if err := cleanupAll(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to restore tablet mode state: ", err)
		}
		return nil, errors.Wrapf(err, "failed to wait for tablet mode enabled(%v)", enabled)
	}
	return cleanupAll, nil
}

// Focus sets focus on CCA App window.
func (a *App) Focus(ctx context.Context) error {
	return a.conn.Eval(ctx, "Tast.focusWindow()", nil)
}

// InnerResolutionSettingButton returns button for opening rt resolution setting of facing camera.
func (a *App) InnerResolutionSettingButton(ctx context.Context, facing Facing, rt ResolutionType) (*UIComponent, error) {
	fname, ok := (map[Facing]string{
		FacingBack:     "back",
		FacingFront:    "front",
		FacingExternal: "external",
	})[facing]
	if !ok {
		return nil, errors.Errorf("cannot get resolution of unsuppport facing %v", facing)
	}

	name := fmt.Sprintf("%v camera %v resolution settings button", fname, rt)
	ariaPrefix := fname
	if facing == FacingExternal {
		// Assumes already switched to target external camera.
		id, err := a.GetDeviceID(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get device id of external camera")
		}
		ariaPrefix = string(id)
	}
	selector := fmt.Sprintf("button[aria-describedby='%s-%sres-desc']", ariaPrefix, rt)

	return &UIComponent{name, []string{selector}}, nil
}
