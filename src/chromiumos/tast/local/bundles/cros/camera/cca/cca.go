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
	ccaURLPrefix       = fmt.Sprintf("chrome-extension://%s/views/main.html", ID)
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
	// ExpertModeButton is button used for opening expert mode setting menu.
	ExpertModeButton = UIComponent{"expert mode button", []string{"#settings-expert"}}
	// MirrorButton is button used for toggling preview mirroring option.
	MirrorButton = UIComponent{"mirror button", []string{"#toggle-mirror"}}
	// ModeSelector is selection bar for different capture modes.
	ModeSelector = UIComponent{"mode selector", []string{"#modes-group"}}
	// SettingsButton is button for opening master setting menu.
	SettingsButton = UIComponent{"settings", []string{"#open-settings"}}
	// SwitchDeviceButton is button for switching camera device.
	SwitchDeviceButton = UIComponent{"switch device button", []string{"#switch-device"}}

	// SettingsBackButton is back button for closing master setting menu.
	SettingsBackButton = UIComponent{"settings back button", []string{
		"#settings .menu-header button", "#view-settings .menu-header button"}}
	// GridSettingBackButton is back button for closing grid setting menu.
	GridSettingBackButton = UIComponent{"grid setting back button", []string{
		"#gridsettings .menu-header button", "#view-grid-settings .menu-header button"}}
	// TimerSettingBackButton is back button for closing timer setting menu.
	TimerSettingBackButton = UIComponent{"timer setting back button", []string{
		"#timersettings .menu-header button", "#view-timer-settings .menu-header button"}}
	// ResolutionSettingBackButton is back button for closing resolution setting menu.
	ResolutionSettingBackButton = UIComponent{"resolution setting back button", []string{
		"#resolutionsettings .menu-header button",
		"#view-resolution-settings .menu-header button"}}
	// PhotoResolutionSettingBackButton is back button for closing photo resolution setting menu.
	PhotoResolutionSettingBackButton = UIComponent{"photo resolution setting back button",
		[]string{
			"#photoresolutionsettings .menu-header button",
			"#view-photo-resolution-settings .menu-header button"}}
	// VideoResolutionSettingBackButton is back button for closing video resolution setting menu.
	VideoResolutionSettingBackButton = UIComponent{"video resolution setting back button",
		[]string{
			"#videoresolutionsettings .menu-header button",
			"#view-video-resolution-settings .menu-header button"}}

	// PhotoResolutionOption is option for each available photo capture resolution.
	PhotoResolutionOption = UIComponent{"photo resolution option", []string{
		"#photoresolutionsettings input", "#view-photo-resolution-settings input"}}
	// VideoResolutionOption is option for each available video capture resolution.
	VideoResolutionOption = UIComponent{"video resolution option", []string{
		"#videoresolutionsettings input", "#view-video-resolution-settings input"}}
)

// App represents a CCA (Chrome Camera App) instance.
type App struct {
	conn        *chrome.Conn
	cr          *chrome.Chrome
	scriptPaths []string
	outDir      string // Output directory to save the execution result
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

// AppLauncher is used during the launch process of CCA. We could launch CCA
// by launchApp event, camera intent or any other ways.
type AppLauncher func(tconn *chrome.TestConn) error

func isMatchCCAPrefix(t *target.Info) bool {
	return strings.HasPrefix(t.URL, ccaURLPrefix)
}

// Init launches a CCA instance by appLauncher, evaluates the helper script
// within it and waits until its AppWindow interactable. The scriptPath should
// be the data path to the helper script cca_ui.js. The returned App instance
// must be closed when the test is finished.
func Init(ctx context.Context, cr *chrome.Chrome, scriptPaths []string, outDir string, appLauncher AppLauncher) (*App, error) {
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

	prepareCCA := fmt.Sprintf(`
		CCAReady = tast.promisify(chrome.runtime.sendMessage)(
			%q, {action: 'SET_WINDOW_CREATED_CALLBACK'}, null);`, ID)
	if err := tconn.Exec(ctx, prepareCCA); err != nil {
		return nil, err
	}

	if err := appLauncher(tconn); err != nil {
		return nil, err
	}

	var windowURL string
	if err := tconn.EvalPromise(ctx, `CCAReady`, &windowURL); err != nil {
		return nil, err
	}
	// The expected windowURL is returned as:
	//		views/main.html
	// Or:
	//		views/main.html?...
	// And the CCA's URL used in chrome package should be something like:
	//		chrome-extension://.../views/main.html...
	// As a result, we should add the "chrome-extension://.../" as prefix.
	if !strings.HasPrefix(windowURL, "views/main.html") {
		return nil, errors.Errorf("unexpected window URL is returned: %q", windowURL)
	}
	windowURL = ccaURLPrefix + strings.TrimPrefix(windowURL, "views/main.html")

	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(windowURL))
	if err != nil {
		return nil, err
	}

	conn.StartProfiling(ctx)

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
		if err := conn.EvalPromise(ctx, string(script), nil); err != nil {
			return nil, err
		}
	}
	testing.ContextLog(ctx, "CCA launched")

	app := &App{conn, cr, scriptPaths, outDir}
	waitForWindowReady := func() error {
		if err := app.WaitForVideoActive(ctx); err != nil {
			return err
		}
		return app.WaitForState(ctx, "view-camera", true)
	}
	if err := waitForWindowReady(); err != nil {
		app.Close(ctx)
		return nil, errors.Wrap(err, "CCA window is not ready after launching app")
	}
	testing.ContextLog(ctx, "CCA window is ready")

	return app, nil
}

// New launches a CCA instance by launchApp event and initialize it. The
// returned App instance must be closed when the test is finished.
func New(ctx context.Context, cr *chrome.Chrome, scriptPaths []string, outDir string) (*App, error) {
	return Init(ctx, cr, scriptPaths, outDir, func(tconn *chrome.TestConn) error {
		launchApp := fmt.Sprintf(`tast.promisify(chrome.management.launchApp)(%q);`, ID)
		if err := tconn.EvalPromise(ctx, launchApp, nil); err != nil {
			return err
		}
		return nil
	})
}

// InstanceExists checks if there is any running CCA instance.
func InstanceExists(ctx context.Context, cr *chrome.Chrome) (bool, error) {
	return cr.IsTargetAvailable(ctx, isMatchCCAPrefix)
}

// Close closes the App and the associated connection.
func (a *App) Close(ctx context.Context) error {
	if a.conn == nil {
		// It's already closed. Do nothing.
		return nil
	}

	// TODO(b/144747002): Some tests (e.g. CCUIIntent) might trigger auto closing of CCA before
	// calling Close(). We should handle it gracefully to get the coverage report for them.
	err := a.OutputCodeCoverage(ctx)
	if err != nil {
		return err
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
	newApp, err := New(ctx, a.cr, a.scriptPaths, a.outDir)
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
func (a *App) WaitForFileSaved(ctx context.Context, dir string, pat *regexp.Regexp, ts time.Time) (os.FileInfo, error) {
	const timeout = 5 * time.Second
	var result os.FileInfo
	seen := make(map[string]struct{})
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		files, err := ioutil.ReadDir(dir)
		if err != nil {
			return errors.Wrap(err, "failed to read the directory where media files are saved")
		}
		for _, file := range files {
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
	err := a.conn.EvalPromise(ctx, "Tast.getNumOfCameras()", &numCameras)
	return numCameras, err
}

// GetFacing returns the active camera facing.
func (a *App) GetFacing(ctx context.Context) (Facing, error) {
	var facing Facing
	if err := a.conn.EvalPromise(ctx, "Tast.getFacing()", &facing); err != nil {
		return "", err
	}
	return facing, nil
}

// GetPreviewResolution returns resolution of preview video.
func (a *App) GetPreviewResolution(ctx context.Context) (Resolution, error) {
	r := Resolution{-1, -1}
	if err := a.conn.EvalPromise(ctx, "Tast.getPreviewResolution()", &r); err != nil {
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
	if err := a.conn.EvalPromise(ctx, "Tast.getPhotoResolutions()", &rs); err != nil {
		return nil, errors.Wrap(err, "failed to get photo resolution")
	}
	return rs, nil
}

// GetVideoResolutions returns available video resolutions of active camera on HALv3 device.
func (a *App) GetVideoResolutions(ctx context.Context) ([]Resolution, error) {
	var rs []Resolution
	if err := a.conn.EvalPromise(ctx, "Tast.getVideoResolutions()", &rs); err != nil {
		return nil, errors.Wrap(err, "failed to get video resolution")
	}
	return rs, nil
}

// GetDeviceID returns the active camera device id.
func (a *App) GetDeviceID(ctx context.Context) (DeviceID, error) {
	var id DeviceID
	if err := a.conn.EvalPromise(ctx, "Tast.getDeviceId()", &id); err != nil {
		return "", err
	}
	return id, nil
}

// GetState returns whether a state is active in CCA.
func (a *App) GetState(ctx context.Context, state string) (bool, error) {
	var result bool
	if err := a.conn.Eval(ctx, fmt.Sprintf("Tast.getState(%q)", state), &result); err != nil {
		return false, errors.Wrapf(err, "failed to get state: %v", state)
	}
	return result, nil
}

// PortraitModeSupported returns whether portrait mode is supported by the current active video device.
func (a *App) PortraitModeSupported(ctx context.Context) (bool, error) {
	var result bool
	if err := a.conn.EvalPromise(ctx, "Tast.isPortraitModeSupported()", &result); err != nil {
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

	dir, err := a.SavedDir(ctx)
	if err != nil {
		return nil, err
	}

	var fileInfos []os.FileInfo
	for _, pattern := range patterns {
		info, err := a.WaitForFileSaved(ctx, dir, pattern, start)
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
		info, err := a.WaitForFileSaved(ctx, dir, pattern, start)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot find result metadata with regexp: %v", pattern)
		}

		if info.Size() == 0 {
			return nil, errors.Errorf("saved file %v is empty", info.Name())
		}

		if err != nil {
			return nil, err
		}
		var jsonString map[string]interface{}
		if content, err := ioutil.ReadFile(filepath.Join(dir, info.Name())); err != nil {
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
	return startTime, nil
}

// StopRecording stops recording a video.
func (a *App) StopRecording(ctx context.Context, timerState TimerState, startTime time.Time) (os.FileInfo, error) {
	testing.ContextLog(ctx, "Click on stop shutter")
	if err := a.ClickShutter(ctx); err != nil {
		return nil, err
	}
	if err := a.WaitForState(ctx, "taking", false); err != nil {
		return nil, errors.Wrap(err, "shutter is not ended")
	}
	dir, err := a.SavedDir(ctx)
	if err != nil {
		return nil, err
	}
	info, err := a.WaitForFileSaved(ctx, dir, VideoPattern, startTime)
	if err != nil {
		return nil, errors.Wrap(err, "cannot find result video")
	} else if elapsed := info.ModTime().Sub(startTime); timerState == TimerOn && elapsed < TimerDelay {
		return nil, errors.Errorf("the capture happen after elapsed time %v, should be after %v timer", elapsed, TimerDelay)
	}
	return info, nil
}

// RecordVideo records a video and save to default location.
func (a *App) RecordVideo(ctx context.Context, timerState TimerState, duration time.Duration) (os.FileInfo, error) {
	startTime, err := a.StartRecording(ctx, timerState)
	if err != nil {
		return nil, err
	}

	sleepDelay := duration
	if timerState == TimerOn {
		sleepDelay += TimerDelay
	}
	if err := testing.Sleep(ctx, sleepDelay); err != nil {
		return nil, err
	}

	return a.StopRecording(ctx, timerState, startTime)
}

// SavedDir returns the path to the folder where captured files are saved.
func (a *App) SavedDir(ctx context.Context) (string, error) {
	path, err := cryptohome.UserPath(ctx, a.cr.User())
	if err != nil {
		return "", err
	}
	return filepath.Join(path, "Downloads"), err
}

// CheckFacing returns an error if the active camera facing is not expected.
func (a *App) CheckFacing(ctx context.Context, expected Facing) error {
	checkFacing := fmt.Sprintf("Tast.checkFacing(%q)", expected)
	return a.conn.EvalPromise(ctx, checkFacing, nil)
}

// Mirrored returns whether mirroring is on.
func (a *App) Mirrored(ctx context.Context) (bool, error) {
	var actual bool
	err := a.conn.Eval(ctx, "Tast.getState('mirror')", &actual)
	return actual, err
}

func (a *App) selectorExist(ctx context.Context, selector string) (bool, error) {
	code := fmt.Sprintf("Tast.isExist(%q)", selector)
	var exist bool
	err := a.conn.Eval(ctx, code, &exist)
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
	code := fmt.Sprintf("Tast.isVisible(%q)", selector)
	if err := a.conn.Eval(ctx, code, &visible); err != nil {
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
	if err := a.conn.Eval(ctx, fmt.Sprintf("Tast.isVisible(%q)", reviewElementID), &visible); err != nil {
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
	if err := a.conn.Exec(ctx, expr); err != nil {
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
	return a.conn.EvalPromise(ctx, "Tast.switchCamera()", nil)
}

// SwitchMode switches to specified capture mode.
func (a *App) SwitchMode(ctx context.Context, mode Mode) error {
	if active, err := a.GetState(ctx, string(mode)); err != nil {
		return err
	} else if active {
		return nil
	}
	code := fmt.Sprintf("Tast.switchMode(%q)", mode)
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
	code := fmt.Sprintf("Tast.getState(%q) === %t", state, active)
	if err := a.conn.WaitForExpr(ctx, code); err != nil {
		return errors.Wrapf(err, "failed to wait for state %s to set to %v", state, active)
	}
	return nil
}

// CheckGridOption checks whether grid option enable state is as expected.
func (a *App) CheckGridOption(ctx context.Context, expected bool) error {
	var actual bool
	err := a.conn.Eval(ctx, "Tast.getState('grid')", &actual)
	if err != nil {
		return err
	} else if actual != expected {
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
	code := fmt.Sprintf("document.querySelector(%q).click()", selector)
	if err := a.conn.Eval(ctx, code, nil); err != nil {
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
	code := fmt.Sprintf("document.querySelectorAll(%q)[%d].click()", selector, index)
	if err := a.conn.Eval(ctx, code, nil); err != nil {
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
	code := fmt.Sprintf("document.querySelectorAll(%q)[%d].checked", selector, index)
	var checked bool
	if err := a.conn.Eval(ctx, code, &checked); err != nil {
		return false, errors.Wrapf(err, "failed to get checked state on %v(th) %v", index, ui.Name)
	}
	return checked, nil
}

// ClickWithSelector clicks an element with given selector.
func (a *App) ClickWithSelector(ctx context.Context, selector string) error {
	code := fmt.Sprintf("document.querySelector(%q).click()", selector)
	return a.conn.Eval(ctx, code, nil)
}

// RemoveCacheData removes the cached key value pair in local storage.
func (a *App) RemoveCacheData(ctx context.Context, keys []string) error {
	keyArray := "["
	for i, key := range keys {
		if i == 0 {
			keyArray += fmt.Sprintf("%q", key)
		} else {
			keyArray += fmt.Sprintf(", %q", key)
		}
	}
	keyArray += "]"
	code := fmt.Sprintf("Tast.removeCacheData(%v)", keyArray)
	if err := a.conn.EvalPromise(ctx, code, nil); err != nil {
		testing.ContextLogf(ctx, "Failed to remove cache (%q): %v", code, err)
		return err
	}
	return nil
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
	code := fmt.Sprintf("Tast.checkMojoConnection(%v)", upstart.JobExists(ctx, "cros-camera"))
	return a.conn.EvalPromise(ctx, code, nil)
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
	if err := a.conn.Exec(ctx, "CCAConfigurationReady = Tast.waitNextConfiguration()"); err != nil {
		return err
	}
	if err := trigger(); err != nil {
		return err
	}
	return a.conn.EvalPromise(ctx, "CCAConfigurationReady", nil)
}
