// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/internal/cdputil"
	"chromiumos/tast/local/chrome/internal/extension"
)

// AppListBubbleClassName is the automation API class name of the bubble launcher.
const AppListBubbleClassName = "AppListBubbleView"

// LauncherState represents the launcher (a.k.a AppList) state.
type LauncherState string

// LauncherState as defined in
// https://cs.chromium.org/chromium/src/ash/public/cpp/app_list/app_list_types.h
const (
	FullscreenAllApps LauncherState = "FullscreenAllApps"
	FullscreenSearch  LauncherState = "FullscreenSearch"
	Closed            LauncherState = "Closed"
)

// Accelerator represents the accelerator key to trigger certain actions.
type Accelerator struct {
	KeyCode string `json:"keyCode"`
	Shift   bool   `json:"shift"`
	Control bool   `json:"control"`
	Alt     bool   `json:"alt"`
	Search  bool   `json:"search"`
}

// Accelerator key used to trigger launcher state change.
var (
	AccelSearch      = Accelerator{KeyCode: "search", Shift: false, Control: false, Alt: false, Search: false}
	AccelShiftSearch = Accelerator{KeyCode: "search", Shift: true, Control: false, Alt: false, Search: false}
)

// WaitForLauncherState waits until the launcher state becomes state. It waits
// up to 10 seconds and fail if the launcher doesn't have the desired state.
// Expected to fail with "Not supported for bubble launcher" error when waiting
// for state different from "Closed" if called for clamshell productivity (bubble)
// launcher. Note that the autotest API is expected to return immediately, but still
// asynchronously, in this case.
// NOTE: Waiting for "Closed" state will always wait for the fullscreen launcher to
// hide, even if one would otherwise expect bubble launcher to be used for the current
// session state - this supports waiting for launcher UI hide animation to complete
// after transitioning from tablet mode to clamshell.
func WaitForLauncherState(ctx context.Context, tconn *chrome.TestConn, state LauncherState) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.waitForLauncherState)", state); err != nil {
		return errors.Wrap(err, "failed to wait for launcher state")
	}
	return nil
}

// TriggerLauncherStateChange will cause the launcher state change via accelerator.
func TriggerLauncherStateChange(ctx context.Context, tconn *chrome.TestConn, accel Accelerator) error {
	// Send the press event to store it in the history. It'll not be handled, so ignore the result.
	if err := tconn.Call(ctx, nil, `async (acceleratorKey) => {
	  acceleratorKey.pressed = true;
	  chrome.autotestPrivate.activateAccelerator(acceleratorKey, () => {});
	  acceleratorKey.pressed = false;
	  await tast.promisify(chrome.autotestPrivate.activateAccelerator)(acceleratorKey);
	}`, accel); err != nil {
		return errors.Wrap(err, "failed to execute accelerator")
	}
	return nil
}

func scaleImage(src image.Image, siz int) image.Image {
	srcSize := src.Bounds().Size().X
	scaled := image.NewRGBA(image.Rect(0, 0, siz, siz))
	for x := 0; x < siz; x++ {
		for y := 0; y < siz; y++ {
			scaled.Set(x, y, src.At(x*srcSize/siz, y*srcSize/siz))
		}
	}
	return scaled
}

func saveImageAsPng(filename string, img image.Image) error {
	w, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer w.Close()
	return png.Encode(w, img)
}

// generateFakeAppNames generates default names for fake apps.
func generateFakeAppNames(numFakeApps int) []string {
	fakeAppNames := make([]string, numFakeApps)
	for i := 0; i < numFakeApps; i++ {
		fakeAppNames[i] = fmt.Sprintf("fake app %d", i)
	}
	return fakeAppNames
}

// GeneratePrepareFakeAppsWithNamesOptions calls PrepareDefaultFakeApps() and
// returns options to be used by chrome.New() for logging in with the newly
// created fake apps. baseDir is the path to the directory for keeping app data.
// The function caller should always clean baseDir regardless of function
// execution results. names specify app names.
func GeneratePrepareFakeAppsWithNamesOptions(baseDir string, names []string) ([]chrome.Option, error) {
	dirs, err := PrepareDefaultFakeApps(baseDir, names, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create fake apps")
	}

	opts := make([]chrome.Option, 0, len(names))
	for _, dir := range dirs {
		opts = append(opts, chrome.UnpackedExtension(dir))
	}
	return opts, nil
}

// GeneratePrepareFakeAppsWithIconDataOptions is similar with GeneratePrepareFakeAppsWithNamesOptions,
// with a difference that GeneratePrepareFakeAppsWithIconDataOptions allows the
// caller to specify both app names and icon data. The caller has the duty to
// clean baseDir.
func GeneratePrepareFakeAppsWithIconDataOptions(baseDir string, names []string, iconData [][]byte) ([]chrome.Option, error) {
	if len(names) != len(iconData) {
		return nil, errors.Errorf("unexpected count of icon data: got %d, expecting %d", len(iconData), len(names))
	}

	dirs, err := prepareFakeAppsWithIconData(baseDir, names, iconData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare data for fake apps")
	}

	opts := make([]chrome.Option, 0, len(names))
	for _, dir := range dirs {
		opts = append(opts, chrome.UnpackedExtension(dir))
	}
	return opts, nil
}

// GeneratePrepareFakeAppsOptions is similar with GeneratePrepareFakeAppsWithNamesOptions,
// with a difference that GeneratePrepareFakeAppsOptions accepts the fake app
// count as the parameter.
func GeneratePrepareFakeAppsOptions(baseDir string, numFakeApps int) ([]chrome.Option, error) {
	return GeneratePrepareFakeAppsWithNamesOptions(baseDir, generateFakeAppNames(numFakeApps))
}

// prepareFakeApp creates data for a fake app with the specified app name and
// icon (if any).
func prepareFakeApp(baseDir, appName, iconDir string, iconFileMap map[int]string) (string, error) {
	// The manifest.json data for the fake hosted app; it just opens google.com
	// page on launch.
	const manifestTmpl = `{
		"description": "fake",
		"name": "%s",
		"manifest_version": 2,
		"version": "0",
		%s
		"app": {
			"launch": {
				"web_url": "https://www.google.com/"
			}
		}
	}`

	extDir := filepath.Join(baseDir, appName)
	if err := os.Mkdir(extDir, 0755); err != nil {
		return "", errors.Wrapf(err, "failed to create the directory for %s", appName)
	}

	var iconJSON string
	if iconDir != "" {
		for _, iconFileName := range iconFileMap {
			if err := os.Symlink(filepath.Join(iconDir, iconFileName), filepath.Join(extDir, iconFileName)); err != nil {
				return "", errors.Wrapf(err, "failed to create link of icon %s", iconFileName)
			}
		}

		iconJSONData, err := json.Marshal(iconFileMap)
		if err != nil {
			return "", errors.Wrap(err, "failed to turn the mapptings between icon sizes and icon names into a JSON string")
		}
		iconJSON = fmt.Sprintf(`"icons": %s,`, string(iconJSONData))
	}

	if err := ioutil.WriteFile(filepath.Join(extDir, "manifest.json"), []byte(fmt.Sprintf(manifestTmpl, appName, iconJSON)), 0644); err != nil {
		return "", errors.Wrapf(err, "failed to prepare manifest.json for %s", appName)
	}

	return extDir, nil
}

// prepareFakeAppIcon creates icon images in different scales with the given
// icon data. These images are stored in a directory created under baseDir.
// iconFolder specifies the directory's name.
func prepareFakeAppIcon(baseDir, iconFolder string, iconData []byte) (string, map[int]string, error) {
	iconDir := filepath.Join(baseDir, iconFolder)
	if err := os.Mkdir(iconDir, 0755); err != nil {
		return "", nil, errors.Wrapf(err, "failed to create the icon directory %q", iconDir)
	}

	img, err := png.Decode(bytes.NewReader(iconData))
	if err != nil {
		return "", nil, errors.Wrap(err, "failed to decode icon data")
	}

	iconFiles := map[int]string{}
	for _, siz := range []int{32, 48, 64, 96, 128, 192} {
		var imgToSave image.Image
		if siz == img.Bounds().Size().X {
			imgToSave = img
		} else {
			imgToSave = scaleImage(img, siz)
		}
		iconFile := fmt.Sprintf("icon%d.png", siz)
		iconFileFullPath := filepath.Join(iconDir, iconFile)
		if err := saveImageAsPng(iconFileFullPath, imgToSave); err != nil {
			return "", nil, errors.Wrapf(err, "failed to save the icon file to %q", iconFileFullPath)
		}
		iconFiles[siz] = iconFile
	}

	return iconDir, iconFiles, nil
}

// PrepareDefaultFakeApps creates directories for fake apps (hosted apps) under
// the directory of baseDir and returns their path names. Fake app names are
// specified by the parameter. hasIcon specifies whether a default icon should
// be used. The intermediate data may remain even when an error is returned. It
// is the caller's responsibility to clean up the contents under the baseDir.
// This also may update the ownership of baseDir.
func PrepareDefaultFakeApps(baseDir string, appNames []string, hasIcon bool) ([]string, error) {
	if err := extension.ChownContentsToChrome(baseDir); err != nil {
		return nil, errors.Wrapf(err, "failed to change ownership of %q", baseDir)
	}

	var iconDir string
	var iconFiles map[int]string
	var err error
	if hasIcon {
		iconDir, iconFiles, err = prepareFakeAppIcon(baseDir, "defaultIcons", fakeIconData)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parepare the shared icon for fake apps")
		}
	}

	var dirs []string
	for _, appName := range appNames {
		dir, err := prepareFakeApp(baseDir, appName, iconDir, iconFiles)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to prepare data for %q", appName)
		}
		dirs = append(dirs, dir)
	}

	return dirs, nil
}

// prepareFakeAppsWithIconData is similar with PrepareDefaultFakeApps, but with
// the difference that app icons are specified by the parameter.
func prepareFakeAppsWithIconData(baseDir string, appNames []string, iconData [][]byte) ([]string, error) {
	if len(appNames) != len(iconData) {
		return nil, errors.Errorf("unexpected count of icon data: got %d, expecting %d", len(iconData), len(appNames))
	}

	if err := extension.ChownContentsToChrome(baseDir); err != nil {
		return nil, errors.Wrapf(err, "failed to change ownership of %q", baseDir)
	}

	var dirs []string
	for index, appName := range appNames {
		iconDir, iconFiles, err := prepareFakeAppIcon(baseDir, appName+"Icons", iconData[index])
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parepare icons for the fake app %q", appName)
		}

		dir, err := prepareFakeApp(baseDir, appName, iconDir, iconFiles)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to prepare data for %q", appName)
		}
		dirs = append(dirs, dir)
	}

	return dirs, nil
}

// The remaining definitions are needed only for faillog & CaptureCDP.
// TODO(crbug.com/1271473): Get rid of them.
// They expose cdputil types and values. See the cdputil package for details.

// DebuggingPortPath is a file where Chrome writes debugging port.
const DebuggingPortPath = cdputil.DebuggingPortPath

// DevtoolsConn is the connection to a web content view, e.g. a tab.
type DevtoolsConn = cdputil.Conn

// Session maintains the connection to talk to the browser in Chrome DevTools Protocol
// over WebSocket.
type Session = cdputil.Session

// PortWaitOption controls whether the NewSession should wait for the port file
// to be created.
type PortWaitOption = cdputil.PortWaitOption

// PortWaitOption values.
const (
	NoWaitPort PortWaitOption = cdputil.NoWaitPort
	WaitPort   PortWaitOption = cdputil.WaitPort
)

// NewDevtoolsSession establishes a Chrome DevTools Protocol WebSocket connection to the browser.
func NewDevtoolsSession(ctx context.Context, debuggingPortPath string, portWait PortWaitOption) (sess *Session, retErr error) {
	return cdputil.NewSession(ctx, debuggingPortPath, portWait)
}
