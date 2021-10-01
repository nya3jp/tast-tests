// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/internal/extension"
)

// AppListBubbleClassName is the automation API class name of the bubble launcher.
const AppListBubbleClassName = "AppListBubbleView"

// LauncherState represents the launcher (a.k.a AppList) state.
type LauncherState string

// LauncherState as defined in
// https://cs.chromium.org/chromium/src/ash/public/cpp/app_list/app_list_types.h
const (
	Peeking           LauncherState = "Peeking"
	FullscreenAllApps LauncherState = "FullscreenAllApps"
	FullscreenSearch  LauncherState = "FullscreenSearch"
	Half              LauncherState = "Half"
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

// GetPrepareFakeAppsOptions calls PrepareFakeApps() and returns options to be
// used by chrome.New() for logging in with the newly created fake apps. The
// caller is also responsible for cleaning up the extDirBase which gets created.
func GetPrepareFakeAppsOptions(numFakeApps int) ([]chrome.Option, string, error) {
	extDirBase, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, extDirBase, errors.Wrap(err, "failed to create tempdir")
	}

	dirs, err := PrepareFakeApps(extDirBase, numFakeApps, nil)
	if err != nil {
		return nil, extDirBase, errors.Wrap(err, "failed to prepare fake apps")
	}

	opts := make([]chrome.Option, 0, numFakeApps)
	for _, dir := range dirs {
		opts = append(opts, chrome.UnpackedExtension(dir))
	}
	return opts, extDirBase, nil
}

// PrepareFakeApps creates directories for num fake apps (hosted apps) under
// the directory of baseDir and returns their path names. The intermediate
// data may remain even when an error is returned. It is the caller's
// responsibility to clean up the contents under the baseDir. This also may
// update the ownership of baseDir. iconData is the data of the icon for those
// fake apps in png format, or nil if the default icon is used.
func PrepareFakeApps(baseDir string, num int, iconData []byte) ([]string, error) {
	// The manifest.json data for the fake hosted app; it just opens google.com
	// page on launch.
	const manifestTmpl = `{
		"description": "fake",
		"name": "fake app %d",
		"manifest_version": 2,
		"version": "0",
		%s
		"app": {
			"launch": {
				"web_url": "https://www.google.com/"
			}
		}
	}`
	if err := extension.ChownContentsToChrome(baseDir); err != nil {
		return nil, errors.Wrapf(err, "failed to change ownership of %s", baseDir)
	}

	iconDir := filepath.Join(baseDir, "icons")
	iconFiles := map[int]string{}
	var iconJSON string
	if iconData != nil {
		if err := os.Mkdir(iconDir, 0755); err != nil {
			return nil, errors.Wrapf(err, "failed to create the icon directory %q", iconDir)
		}
		img, err := png.Decode(bytes.NewReader(iconData))
		if err != nil {
			return nil, err
		}
		for _, siz := range []int{32, 48, 64, 96, 128, 192} {
			var imgToSave image.Image
			if siz == img.Bounds().Size().X {
				imgToSave = img
			} else {
				imgToSave = scaleImage(img, siz)
			}
			iconFile := fmt.Sprintf("icon%d.png", siz)
			if err := saveImageAsPng(filepath.Join(iconDir, iconFile), imgToSave); err != nil {
				return nil, err
			}
			iconFiles[siz] = iconFile
		}
		iconJSONData, err := json.Marshal(iconFiles)
		if err != nil {
			return nil, err
		}
		iconJSON = fmt.Sprintf(`"icons": %s,`, string(iconJSONData))
	}

	extDirs := make([]string, 0, num)
	for i := 0; i < num; i++ {
		extDir := filepath.Join(baseDir, fmt.Sprintf("fake_%d", i))
		if err := os.Mkdir(extDir, 0755); err != nil {
			return nil, errors.Wrapf(err, "failed to create the directory for %d-th extension", i)
		}
		if iconJSON != "" {
			for _, iconFile := range iconFiles {
				if err := os.Symlink(filepath.Join(iconDir, iconFile), filepath.Join(extDir, iconFile)); err != nil {
					return nil, errors.Wrapf(err, "failed to create link of icon %q", iconFile)
				}
			}
		}
		if err := ioutil.WriteFile(filepath.Join(extDir, "manifest.json"), []byte(fmt.Sprintf(manifestTmpl, i, iconJSON)), 0644); err != nil {
			return nil, errors.Wrapf(err, "failed to prepare manifest.json for %d-th extension", i)
		}
		extDirs = append(extDirs, extDir)
	}
	return extDirs, nil
}
