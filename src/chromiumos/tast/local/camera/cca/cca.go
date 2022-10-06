// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cca provides utilities to interact with Chrome Camera App.
package cca

import (
	"context"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// Facing is camera facing from JavaScript VideoFacingModeEnum.
type Facing string

const (
	// FacingBack is the constant string from JavaScript VideoFacingModeEnum.
	FacingBack Facing = "environment"
	// FacingFront is the constant string from JavaScript VideoFacingModeEnum.
	FacingFront Facing = "user"
	// FacingExternal is the constant string indicating external camera facing.
	FacingExternal Facing = "external"
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
	Photo Mode = "photo"
	// Square is the mode used to take square photo.
	// TODO(b/215484798): Removed since there is no square mode in new UI.
	Square = "square"
	// Portrait is the mode used to take portrait photo.
	Portrait = "portrait"
	// Scan is the mode used to scan barcode/document.
	Scan = "scan"

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
	VideoPattern = regexp.MustCompile(`^VID_\d{8}_\d{6}[^.]*\.mp4$`)
	// gifPattern is the filename format of gif recorded by CCA.
	gifPattern = regexp.MustCompile(`^VID_\d{8}_\d{6}[^.]*\.gif$`)
	// portraitPattern is the filename format of portrait-mode photos taken by CCA.
	portraitPattern = regexp.MustCompile(`^IMG_\d{8}_\d{6}[^.]*\_BURST\d{5}_COVER.jpg$`)
	// portraitRefPattern is the filename format of the reference photo captured in portrait-mode.
	portraitRefPattern = regexp.MustCompile(`^IMG_\d{8}_\d{6}[^.]*\_BURST\d{5}.jpg$`)
	// DocumentPDFPattern is the filename format of the document PDF file.
	DocumentPDFPattern = regexp.MustCompile(`^SCN_\d{8}_\d{6}[^.]*\.pdf$`)
	// DocumentPhotoPattern is the filename format of the document photo file.
	DocumentPhotoPattern = regexp.MustCompile(`^SCN_\d{8}_\d{6}[^.]*\.jpg$`)
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

// Profile is type of encoder profile.
type Profile struct {
	Name  string
	Value int
}

var (
	// ProfileH264Baseline is h264 baseline profile.
	ProfileH264Baseline = Profile{"baseline", 66}
	// ProfileH264Main is h264 main profile.
	ProfileH264Main = Profile{"main", 77}
	// ProfileH264High is h264 high profile.
	ProfileH264High = Profile{"high", 100}
)

// Option returns the value of corresponding select-option.
func (p Profile) Option() string {
	return strconv.Itoa(int(p.Value))
}

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
	cameraType  testutil.UseCameraType
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

// Range represents valid range of range type input element.
type Range struct {
	Max int `json:"max"`
	Min int `json:"min"`
}

// AspectRatio returns width divided by height as the aspect ratio of the resolution.
func (r *Resolution) AspectRatio() float64 {
	return float64(r.Width) / float64(r.Height)
}

// Init launches a CCA instance, evaluates the helper script within it and waits
// until its AppWindow interactable. The scriptPath should be the data path to
// the helper script cca_ui.js. The returned App instance must be closed when
// the test is finished.
func Init(ctx context.Context, cr *chrome.Chrome, scriptPaths []string, outDir string, appLauncher testutil.AppLauncher, tb *testutil.TestBridge) (_ *App, retErr error) {
	// Since we don't use "cros-camera" service for fake camera, there is no need
	// to ensure it is running.
	if tb.CameraType != testutil.UseFakeCamera {
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

		return loadScripts(ctx, conn, scriptPaths)
	}(); err != nil {
		if closeErr := testutil.CloseApp(ctx, cr, conn, appLauncher.UseSWAWindow); closeErr != nil {
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

	testing.ContextLog(ctx, "CCA launched")
	app := &App{conn, cr, scriptPaths, outDir, appLauncher, appWindow, tb.CameraType}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	defer func(cleanupCtx context.Context) {
		if retErr == nil {
			return
		}

		if err := app.Close(cleanupCtx); err != nil {
			testing.ContextLog(cleanupCtx, "Failed to close app: ", err)
		}
	}(cleanupCtx)

	if err := app.WaitForVideoActive(ctx); err != nil {
		return nil, errors.Wrap(err, ErrVideoNotActive)
	}

	if err := app.WaitForState(ctx, "view-camera", true); err != nil {
		return nil, errors.Wrap(err, "failed to wait for view-camera becomes true")
	}
	testing.ContextLog(ctx, "CCA window is ready")
	return app, nil
}

func loadScripts(ctx context.Context, conn *chrome.Conn, scriptPaths []string) error {
	for _, scriptPath := range scriptPaths {
		script, err := ioutil.ReadFile(scriptPath)
		if err != nil {
			return err
		}
		if err := conn.Eval(ctx, string(script), nil); err != nil {
			return err
		}
	}
	return nil
}

// New launches a CCA instance. The returned App instance must be closed when the test is finished.
func New(ctx context.Context, cr *chrome.Chrome, scriptPaths []string, outDir string, tb *testutil.TestBridge) (*App, error) {
	return Init(ctx, cr, scriptPaths, outDir, testutil.AppLauncher{
		LaunchApp: func(ctx context.Context, tconn *chrome.TestConn) error {
			return apps.LaunchSystemWebApp(ctx, tconn, "Camera", "chrome://camera-app/views/main.html")
		},
		UseSWAWindow: true,
	}, tb)
}

// InstanceExists checks if there is any running CCA instance.
func InstanceExists(ctx context.Context, cr *chrome.Chrome) (bool, error) {
	checkPrefix := func(t *target.Info) bool {
		url := "chrome://camera-app/views/main.html"
		return strings.HasPrefix(t.URL, url)
	}
	return cr.IsTargetAvailable(ctx, checkPrefix)
}

// ClosingItself checks if CCA intends to close itself.
func (a *App) ClosingItself(ctx context.Context) (bool, error) {
	return a.appWindow.ClosingItself(ctx)
}

// checkJSError checks javascript error emitted by CCA error callback. If
// |saveCameraFolderWhenFail| is true, copies files in the camera folder to
// output directory if there is any JS errors found.
func (a *App) checkJSError(ctx context.Context, saveCameraFolderWhenFail bool) error {
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
			t := time.Unix(0, err.Time*1e6).UTC().Format("2006/01/02 15:04:05 [15:04:05.000]")
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
		if saveCameraFolderWhenFail {
			a.SaveCameraFolder(ctx)
		}
		return &ErrJS{fmt.Sprintf("there are %d JS errors, first error: type=%v. name=%v",
			len(jsErrors), jsErrors[0].ErrorType, jsErrors[0].ErrorName)}
	}
	return nil
}

// Close closes the App and the associated connection.
func (a *App) Close(ctx context.Context) error {
	return a.CloseWithDebugParams(ctx, DebugParams{})
}

// CloseWithDebugParams closes the App and the associated connection with the debug parameters.
func (a *App) CloseWithDebugParams(ctx context.Context, params DebugParams) (retErr error) {
	if a.conn == nil {
		// It's already closed. Do nothing.
		return nil
	}

	cleanupCtx := ctx
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	defer func(ctx context.Context) {
		reportOrLogError := func(err error) {
			if retErr == nil {
				retErr = err
			} else {
				testing.ContextLog(ctx, "Failed to close app: ", err)
			}
		}

		if err := testutil.CloseApp(ctx, a.cr, a.conn, a.appLauncher.UseSWAWindow); err != nil {
			reportOrLogError(errors.Wrap(err, "failed to close app"))
		}
		if err := a.conn.Close(); err != nil {
			reportOrLogError(errors.Wrap(err, "failed to Conn.Close()"))
		}
		if err := a.appWindow.WaitUntilClosed(ctx); err != nil {
			reportOrLogError(errors.Wrap(err, "failed to wait for appWindow close"))
		}
		if err := a.checkJSError(ctx, params.SaveCameraFolderWhenFail); err != nil {
			reportOrLogError(errors.Wrap(err, "There are JS errors when running CCA"))
		}
		if err := a.appWindow.Release(ctx); err != nil {
			reportOrLogError(errors.Wrap(err, "failed to release app window"))
		}

		testing.ContextLog(ctx, "CCA closed")
		a.conn = nil
		a.appWindow = nil
	}(cleanupCtx)

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
func (a *App) Restart(ctx context.Context, tb *testutil.TestBridge) error {
	if err := a.Close(ctx); err != nil {
		return err
	}
	newApp, err := Init(ctx, a.cr, a.scriptPaths, a.outDir, a.appLauncher, tb)
	if err != nil {
		return err
	}
	*a = *newApp
	return nil
}

func (a *App) checkVideoState(ctx context.Context, active bool, duration time.Duration) error {
	cleanupCtx := ctx
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	code := fmt.Sprintf("Tast.isVideoActive() === %t", active)
	if err := a.conn.WaitForExpr(ctx, code); err != nil {
		if a.cameraType != testutil.UseFakeCamera {
			if jobErr := upstart.CheckJob(cleanupCtx, "cros-camera"); jobErr != nil {
				return errors.Wrap(jobErr, err.Error())
			}
		}
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

// WaitForFileSaved calls WaitForFileSavedFor with 5 second timeout.
func (a *App) WaitForFileSaved(ctx context.Context, dir string, pat *regexp.Regexp, ts time.Time) (os.FileInfo, error) {
	return a.WaitForFileSavedFor(ctx, dir, pat, ts, 5*time.Second)
}

// WaitForFileSavedFor waits for the presence of the captured file with file name matching the specified
// pattern, size larger than zero, and modified time after the specified timestamp.
func (a *App) WaitForFileSavedFor(ctx context.Context, dir string, pat *regexp.Regexp, ts time.Time, timeout time.Duration) (os.FileInfo, error) {
	var result os.FileInfo
	seen := make(map[string]struct{})
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		files, err := ioutil.ReadDir(dir)
		if err != nil {
			return errors.Wrap(err, "failed to read the camera directory")
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

// GetPreviewViewportSize returns resolution of the preview view port.
func (a *App) GetPreviewViewportSize(ctx context.Context) (Resolution, error) {
	r := Resolution{-1, -1}
	if err := a.conn.Eval(ctx, "Tast.getPreviewViewportSize()", &r); err != nil {
		return r, errors.Wrap(err, "failed to get the size of preview viewport")
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

// State returns whether a state is active in CCA.
func (a *App) State(ctx context.Context, state string) (bool, error) {
	var result bool
	if err := a.conn.Call(ctx, &result, "Tast.getState", state); err != nil {
		return false, errors.Wrapf(err, "failed to get state: %v", state)
	}
	return result, nil
}

// PreviewFrame grabs a frame from preview. The caller should be responsible for releasing the frame.
func (a *App) PreviewFrame(ctx context.Context) (*Frame, error) {
	if err := a.WaitForVideoActive(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to wait for preview active")
	}
	var f chrome.JSObject
	if err := a.conn.Call(ctx, &f, "Tast.getPreviewFrame"); err != nil {
		return nil, errors.Wrap(err, "failed to get preview frame")
	}
	return &Frame{&f}, nil
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

	isPortrait, err := a.State(ctx, string(Portrait))
	if err != nil {
		return nil, err
	}
	if isPortrait {
		patterns = append(patterns, portraitRefPattern)
		patterns = append(patterns, portraitPattern)
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

	isExpert, err := a.State(ctx, Expert)
	if err != nil {
		return nil, err
	}
	isSaveMetadata, err := a.State(ctx, SaveMetadata)
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

		path, err := a.FilePathInSavedDir(ctx, info.Name())
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
	dir, err := a.SavedDir(ctx)
	if err != nil {
		return nil, time.Time{}, err
	}
	info, err := a.WaitForFileSaved(ctx, dir, VideoPattern, startTime)
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

// RecordGif records a gif with maximal duration and |save| specify whether to save result gif in review page.
func (a *App) RecordGif(ctx context.Context, save bool) (os.FileInfo, error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second*5)
	defer cancel()

	if visible, err := a.Visible(ctx, GifRecordingOption); err != nil {
		return nil, err
	} else if !visible {
		// TODO(b/191950622): Remove the legacy enabling logic after formally launched.
		testing.ContextLog(ctx, "No gif recording option present, try enabling from expert mode")
		if err := a.setEnableGifRecording(ctx, true); err != nil {
			return nil, errors.Wrap(err, "failed to enable gif recording")
		}
		defer a.setEnableGifRecording(cleanupCtx, false)
	}

	if err := a.Click(ctx, GifRecordingOption); err != nil {
		return nil, err
	}

	// Start recording and wait for record to maximal gif duration and review page opened.
	if err := a.ClickShutter(ctx); err != nil {
		return nil, err
	}
	if err := a.WaitForState(ctx, "recording", true); err != nil {
		return nil, errors.Wrap(err, "gif recording is not started")
	}
	if err := a.WaitForState(ctx, "view-review", true); err != nil {
		return nil, errors.Wrap(err, "review page not opened after recording gif")
	}
	if !save {
		if err := a.Click(ctx, GifReviewRetakeButton); err != nil {
			return nil, err
		}
		if err := a.WaitForState(ctx, "taking", false); err != nil {
			return nil, errors.Wrap(err, "shutter is ended after clicking retake")
		}

		return nil, nil
	}

	// Check saved file.
	beforeSaveTime := time.Now()
	if err := a.Click(ctx, GifReviewSaveButton); err != nil {
		return nil, err
	}
	dir, err := a.SavedDir(ctx)
	if err != nil {
		return nil, err
	}
	info, err := a.WaitForFileSaved(ctx, dir, gifPattern, beforeSaveTime)
	if err != nil {
		return nil, errors.Wrap(err, "cannot find result gif")
	}
	if err := a.WaitForState(ctx, "taking", false); err != nil {
		return nil, errors.Wrap(err, "shutter is ended after saving file")
	}

	return info, nil
}

// savedDir returns the path to the folder where captured files might be saved.
func savedDir(ctx context.Context, cr *chrome.Chrome) (string, error) {
	path, err := cryptohome.UserPath(ctx, cr.NormalizedUser())
	if err != nil {
		return "", err
	}
	return filepath.Join(path, "MyFiles", "Camera"), nil
}

// ClearSavedDir clears all files in the folder where captured files might be saved.
func ClearSavedDir(ctx context.Context, cr *chrome.Chrome) error {
	dir, err := savedDir(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to get saved directory")
	}

	// Since "MyFiles/Camera" folder is not deletable by users once it is
	// created by CCA, we have assumption in CCA that it won't be deleted during
	// user session. Therefore, instead of completely deleting the it, we clear
	// all the contents inside if it exists.
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrap(err, "failed to read saved directory")
	}
	for _, file := range files {
		path := filepath.Join(dir, file.Name())
		if err := os.RemoveAll(path); err != nil {
			return errors.Wrapf(err, "failed to remove file %v from saved directory", path)
		}
	}
	return nil
}

// SavedDir returns the path to the folder where captured files are saved.
func (a *App) SavedDir(ctx context.Context) (string, error) {
	return savedDir(ctx, a.cr)
}

// FilePathInSavedDir finds and returns the path of the target file in saved directory.
func (a *App) FilePathInSavedDir(ctx context.Context, name string) (string, error) {
	dir, err := savedDir(ctx, a.cr)
	if err != nil {
		return "", err
	}

	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "", errors.New("file not found in saved path")
		}
		return "", err
	}
	return path, nil
}

// SaveCameraFolder saves the camera folder to the output directory to make debug easier.
func (a *App) SaveCameraFolder(ctx context.Context) error {
	cameraFolderPath, err := savedDir(ctx, a.cr)
	if err != nil {
		return errors.Wrap(err, "failed to get camera folder path")
	}
	if _, err := os.Stat(cameraFolderPath); err != nil {
		if os.IsNotExist(err) {
			// Nothing to do.
			return nil
		}
		return errors.Wrap(err, "failed to stat camera folder")
	}

	targetFolderPath := filepath.Join(a.outDir, fmt.Sprintf("cameraFolder"))
	if err := os.MkdirAll(targetFolderPath, 0755); err != nil {
		return errors.Wrap(err, "failed to make folder to save camera folder")
	}

	files, err := ioutil.ReadDir(cameraFolderPath)
	if err != nil {
		return errors.Wrap(err, "failed to read camera folder")
	}
	for _, file := range files {
		srcFilePath := filepath.Join(cameraFolderPath, file.Name())
		dstFilePath := filepath.Join(targetFolderPath, file.Name())
		data, err := ioutil.ReadFile(srcFilePath)
		if err != nil {
			return errors.Wrapf(err, "failed to read file: %v", srcFilePath)
		}
		if err := ioutil.WriteFile(dstFilePath, data, 0644); err != nil {
			return errors.Wrapf(err, "failed to write file: %v", dstFilePath)
		}
	}
	return nil
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
	if err := a.conn.Call(ctx, &exist, "Tast.exist", selector); err != nil {
		return false, errors.Wrapf(err, "failed to check selector %v exist", selector)
	}
	return exist, nil
}

// CheckConfirmUIExists returns whether the confirm UI exists.
func (a *App) CheckConfirmUIExists(ctx context.Context, mode Mode) error {
	// Legacy UI use 'review-result' state to show the review page while new UI use review view.
	// TODO(b/209726472): Clean code path of legacy UI after crrev.com/c/3338157 fully landed.
	isLegacyUI, err := a.State(ctx, "review-result")
	if err != nil {
		return errors.Wrap(err, "failed to judge legacy/new UI")
	}

	if isLegacyUI {
		testing.ContextLog(ctx, "Using legacy review UI")
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
	} else {
		testing.ContextLog(ctx, "Using new review UI")
		if visible, err := a.Visible(ctx, ReviewView); err != nil {
			return err
		} else if !visible {
			return errors.New("review result is not shown")
		}
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
	if err := a.conn.WaitForExpr(ctx, "Tast.getState('review-result') || Tast.getState('view-review') === true"); err != nil {
		return errors.Wrap(err, "failed to wait for review ui showing up")
	}

	if err := a.CheckConfirmUIExists(ctx, mode); err != nil {
		return errors.Wrap(err, "check confirm UI failed")
	}

	button := CancelResultButton
	if isConfirmed {
		button = ConfirmResultButton
	}
	if err := a.Click(ctx, button); err != nil {
		return err
	}
	return nil
}

// ToggleOption toggles on/off of the |option|.
func (a *App) ToggleOption(ctx context.Context, option Option) (bool, error) {
	prev, err := a.State(ctx, option.state)
	if err != nil {
		return false, err
	}
	if err := a.Click(ctx, option.ui); err != nil {
		return false, err
	}
	code := fmt.Sprintf("Tast.getState(%q) !== %t", option.state, prev)
	if err := a.conn.WaitForExpr(ctx, code); err != nil {
		return false, errors.Wrapf(err, "failed to wait for toggling option %s", option.state)
	}
	return a.State(ctx, option.state)
}

func (a *App) setEnableOption(ctx context.Context, option Option, enabled bool) error {
	prev, err := a.State(ctx, option.state)
	if err != nil {
		return errors.Wrapf(err, "failed to get option state %v", option)
	}
	if prev == enabled {
		return nil
	}

	cur, err := a.ToggleOption(ctx, option)
	if err != nil {
		return errors.Wrapf(err, "failed to toggle option %v", option)
	}
	if cur != enabled {
		return errors.Errorf("unexpected state after toggled option %v", option)
	}
	return nil
}

// maybeToggleQRCodeOption toggle QR code option if current state is not |target|.
func (a *App) maybeToggleQRCodeOption(ctx context.Context, target bool) error {
	if current, err := a.State(ctx, ScanBarcodeOptionInPhotoMode.state); err != nil {
		return errors.Wrap(err, "failed to check scan barcode option state")
	} else if current != target {
		if after, err := a.ToggleOption(ctx, ScanBarcodeOptionInPhotoMode); err != nil {
			return errors.Wrap(err, "failed to toggle scan barcode option")
		} else if after != target {
			return errors.New("failed to toggle scan barcode option to target state")
		}
	}
	return nil
}

// EnableQRCodeDetection enables the QR code detection.
func (a *App) EnableQRCodeDetection(ctx context.Context) error {
	if visible, err := a.Visible(ctx, ScanModeButton); err != nil {
		return errors.Wrap(err, "failed to check visibility of scan mode button")
	} else if visible {
		if err := a.SwitchMode(ctx, Scan); err != nil {
			return errors.Wrap(err, "failed to switch to scan mode")
		}
		if err := a.Click(ctx, ScanBarcodeOption); err != nil {
			return errors.Wrap(err, "failed to click the scan barcode option")
		}
	} else {
		if err := a.SwitchMode(ctx, Photo); err != nil {
			return errors.Wrap(err, "failed to switch to photo mode")
		}
		if err := a.maybeToggleQRCodeOption(ctx, true); err != nil {
			return errors.Wrap(err, "failed to toggle QR code detection")
		}
	}
	return nil
}

// DisableQRCodeDetection disables the QR code detection.
func (a *App) DisableQRCodeDetection(ctx context.Context) error {
	visible, err := a.Visible(ctx, ScanModeButton)
	if err != nil {
		return errors.Wrap(err, "failed to check visibility of scan mode button")
	}
	if err := a.SwitchMode(ctx, Photo); err != nil {
		return errors.Wrap(err, "failed to switch to photo mode")
	}
	if visible {
		if err := a.maybeToggleQRCodeOption(ctx, false); err != nil {
			return errors.Wrap(err, "failed to toggle QR code detection")
		}
	}
	return nil
}

// SetTimerOption sets the timer option to on/off.
func (a *App) SetTimerOption(ctx context.Context, state TimerState) error {
	// TODO(b/215484798): Removed the logic for old UI once the new UI applied.
	useOldUI, err := a.OptionExist(ctx, TimerOption)
	if err != nil {
		return errors.Wrap(err, "failed to check the existence of the timer toggle")
	}

	active := state == TimerOn
	if useOldUI {
		if cur, err := a.State(ctx, "timer"); err != nil {
			return err
		} else if cur != active {
			if _, err := a.ToggleOption(ctx, TimerOption); err != nil {
				return err
			}
		}
		// Fix timer to 3 seconds for saving test time.
		if active {
			if delay3, err := a.State(ctx, "timer-3s"); err != nil {
				return err
			} else if !delay3 {
				return errors.New("default timer is not set to 3 seconds")
			}
		}
		return nil
	}

	// New UI
	if timerOn, err := a.State(ctx, "timer"); err != nil {
		return errors.Wrap(err, "failed to get state timer")
	} else if timerOn != active {
		if err := a.Click(ctx, OpenTimerPanelButton); err != nil {
			return errors.Wrap(err, "failed to open timer option panel")
		}
		if active {
			if err := a.ClickChildIfContain(ctx, OptionsContainer, "3 seconds"); err != nil {
				return errors.Wrap(err, "failed to click the 3s timer button")
			}
			if err := a.WaitForState(ctx, "timer-3s", true); err != nil {
				return errors.Wrap(err, "failed to wait for 3s-timer being active")
			}
		} else {
			if err := a.ClickChildIfContain(ctx, OptionsContainer, "Off"); err != nil {
				return errors.Wrap(err, "failed to click the off timer button")
			}
			if err := a.WaitForState(ctx, "timer", false); err != nil {
				return errors.Wrap(err, "failed to wait for timer being inactive")
			}
		}
	}
	return nil
}

// ToggleExpertMode toggles expert mode and returns whether it's enabled after toggling.
func (a *App) ToggleExpertMode(ctx context.Context) (bool, error) {
	prev, err := a.State(ctx, Expert)
	if err != nil {
		return false, err
	}
	if err := a.conn.Eval(ctx, "Tast.toggleExpertMode()", nil); err != nil {
		return false, errors.Wrap(err, "failed to toggle expert mode")
	}
	if err := a.WaitForState(ctx, "expert", !prev); err != nil {
		return false, errors.Wrap(err, "failed to wait for toggling expert mode")
	}
	return a.State(ctx, Expert)
}

// EnableExpertMode enables expert mode.
func (a *App) EnableExpertMode(ctx context.Context) error {
	prevEnabled, err := a.State(ctx, Expert)
	if err != nil {
		return errors.Wrap(err, "failed to get expert mode state")
	}
	if prevEnabled {
		return nil
	}

	enabled, err := a.ToggleExpertMode(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to toggle expert mode")
	}
	if !enabled {
		return errors.New("unexpected state after toggling expert mode")
	}
	return nil
}

// CheckMetadataVisibility checks if metadata is shown/hidden on screen given enabled.
func (a *App) CheckMetadataVisibility(ctx context.Context, enabled bool) error {
	code := fmt.Sprintf("Tast.isVisible('#preview-exposure-time') === %t", enabled)
	if err := a.conn.WaitForExpr(ctx, code); err != nil {
		return errors.Wrapf(err, "failed to wait for metadata visibility set to %v", enabled)
	}
	return nil
}

// EnableDocumentMode enables the document mode via expert mode.
func (a *App) EnableDocumentMode(ctx context.Context) error {
	if err := a.EnableExpertMode(ctx); err != nil {
		return errors.Wrap(err, "failed to enable expert mode")
	}

	if err := MainMenu.Open(ctx, a); err != nil {
		return errors.Wrap(err, "failed to open main menu")
	}
	defer MainMenu.Close(ctx, a)

	if err := ExpertMenu.Open(ctx, a); err != nil {
		return errors.Wrap(err, "failed to open expert menu")
	}
	defer ExpertMenu.Close(ctx, a)

	if err := a.setEnableOption(ctx, EnableDocumentModeOnAllCamerasOption, true); err != nil {
		return errors.Wrap(err, "failed to enable document mode")
	}

	if err := a.WaitForVisibleState(ctx, ScanModeButton, true); err != nil {
		return errors.Wrap(err, "failed to wait for scan mode button shows up")
	}

	return nil
}

// SetEnableMultiStreamRecording enables/disables recording videos with multiple streams via expert mode.
func (a *App) SetEnableMultiStreamRecording(ctx context.Context, enabled bool) error {
	if err := a.EnableExpertMode(ctx); err != nil {
		return errors.Wrap(err, "failed to enable expert mode")
	}

	if err := MainMenu.Open(ctx, a); err != nil {
		return errors.Wrap(err, "failed to open main menu")
	}
	defer MainMenu.Close(ctx, a)

	if err := ExpertMenu.Open(ctx, a); err != nil {
		return errors.Wrap(err, "failed to open expert menu")
	}
	defer ExpertMenu.Close(ctx, a)

	if err := a.setEnableOption(ctx, EnableMultistreamRecordingOption, enabled); err != nil {
		return errors.Wrap(err, "failed to enable multi-stream recording")
	}

	if err := a.WaitForVideoActive(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for video active")
	}

	return nil
}

// setEnableGifRecording enables/disables the gif recording via expert mode.
func (a *App) setEnableGifRecording(ctx context.Context, enabled bool) error {
	if err := a.EnableExpertMode(ctx); err != nil {
		return errors.Wrap(err, "failed to enable expert mode")
	}

	if err := MainMenu.Open(ctx, a); err != nil {
		return errors.Wrap(err, "failed to open main menu")
	}
	defer MainMenu.Close(ctx, a)

	if err := ExpertMenu.Open(ctx, a); err != nil {
		return errors.Wrap(err, "failed to open expert menu")
	}
	defer ExpertMenu.Close(ctx, a)

	if err := a.setEnableOption(ctx, ShowGifRecordingOption, enabled); err != nil {
		return err
	}

	if err := a.WaitForVideoActive(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for video active")
	}

	return nil
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
	if err := a.TriggerConfiguration(ctx, func() error {
		return a.Click(ctx, SwitchDeviceButton)
	}); err != nil {
		return errors.Wrap(err, "failed to switch camera")
	}
	return nil
}

// SwitchMode switches to specified capture mode.
func (a *App) SwitchMode(ctx context.Context, mode Mode) error {
	modeName := string(mode)
	if active, err := a.State(ctx, modeName); err != nil {
		return err
	} else if active {
		return nil
	}
	if err := a.conn.Call(ctx, nil, "Tast.switchMode", modeName); err != nil {
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
	if active, err := a.State(ctx, modeName); err != nil {
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

// ClickWithSelector clicks an element with given selector.
func (a *App) ClickWithSelector(ctx context.Context, selector string) error {
	return a.conn.Call(ctx, nil, `Tast.click`, selector)
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

// ReturnFocusedElementAriaLabel returns the aria-label of the focused element.
func (a *App) ReturnFocusedElementAriaLabel(ctx context.Context) (string, error) {
	var arialabel string

	if err := a.conn.Eval(ctx, "document.activeElement.ariaLabel", &arialabel); err != nil {
		return "", err
	}

	return arialabel, nil
}

// Focus sets focus on CCA App window.
func (a *App) Focus(ctx context.Context) error {
	return a.conn.Eval(ctx, "Tast.focusWindow()", nil)
}

// InnerResolutionSetting returns setting menu for toggle |rt| resolution of |facing| camera.
func (a *App) InnerResolutionSetting(ctx context.Context, facing Facing, rt ResolutionType) (*SettingMenu, error) {
	view := fmt.Sprintf("view-%s-resolution-settings", rt)

	fname, ok := (map[Facing]string{
		FacingBack:     "back",
		FacingFront:    "front",
		FacingExternal: "external",
	})[facing]
	if !ok {
		return nil, errors.Errorf("cannot get resolution of unsuppport facing %v", facing)
	}
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
	openUI := &UIComponent{
		Name:      fmt.Sprintf("%v camera %v resolution settings button", fname, rt),
		Selectors: []string{selector},
	}

	return &SettingMenu{view, openUI}, nil
}

// Refresh refreshes CCA.
func (a *App) Refresh(ctx context.Context, tb *testutil.TestBridge) error {
	newAppWindow, err := testutil.RefreshApp(ctx, a.conn, tb)
	if err != nil {
		return err
	}

	// Releases the previous app window.
	if err := a.appWindow.Release(ctx); err != nil {
		return errors.Wrap(err, "failed to release app window")
	}
	a.appWindow = newAppWindow

	if err := loadScripts(ctx, a.conn, a.scriptPaths); err != nil {
		return errors.Wrap(err, "failed to load scripts")
	}
	return nil
}

// SaveScreenshot saves a screenshot in the outDir.
func (a *App) SaveScreenshot(ctx context.Context) error {
	image, err := screenshot.CaptureChromeImage(ctx, a.cr)
	if err != nil {
		return errors.Wrap(err, "failed to capture Chrome image")
	}

	filename := fmt.Sprintf("screenshot_%d.jpg", time.Now().UnixNano())
	path := filepath.Join(a.outDir, filename)

	file, err := os.Create(path)
	if err != nil {
		return errors.Wrap(err, "failed to create screenshot")
	}
	defer file.Close()

	options := jpeg.Options{Quality: 80}
	err = jpeg.Encode(file, image, &options)
	if err != nil {
		return errors.Wrap(err, "failed to encode screenshot to jpeg")
	}
	return nil
}

// CheckMode checks whether CCA window is in correct capture mode.
func (a *App) CheckMode(ctx context.Context, mode Mode) error {
	if result, err := a.State(ctx, string(mode)); err != nil {
		return errors.Wrap(err, "failed to check state")
	} else if !result {
		return errors.Errorf("CCA is not in the expected mode: %s", mode)
	}
	return nil
}

// CheckCameraFacing checks whether CCA is in correct facing if there's a camera with that facing.
func (a *App) CheckCameraFacing(ctx context.Context, facing Facing) error {
	initialFacing, err := a.GetFacing(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get initial facing")
	}
	if initialFacing == facing {
		return nil
	}
	// It may fail to open desired facing camera on device without camera of
	// that facing or on device without facing configurations which returns
	// facing unknown for every camera. Try to query facing from every
	// available camera to ensure it's a true failure.
	// TODO(pihsun): Change this to a more light-weight solution that use
	// enumerateDevices() in js instead of actually switching through all
	// cameras.
	return a.RunThroughCameras(ctx, func(f Facing) error {
		if f == facing {
			return errors.Errorf("camera in wrong facing: got %v, want %v", initialFacing, facing)
		}
		return nil
	})
}
