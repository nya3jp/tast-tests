// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/internal/extension"
)

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

	var iconFile string
	var iconJSON string
	if iconData != nil {
		img, err := png.Decode(bytes.NewReader(iconData))
		if err != nil {
			return nil, err
		}
		iconDir := filepath.Join(baseDir, "icons")
		iconSize := img.Bounds().Size().X
		iconFile = fmt.Sprintf("icon%d.png", iconSize)
		iconJSON = fmt.Sprintf(`"icons": {"%d": "%s"},`, iconSize, iconFile)
		iconFile = filepath.Join(baseDir, "icons", iconFile)
		if err := os.Mkdir(iconDir, 0755); err != nil {
			return nil, errors.Wrapf(err, "failed to create the icon directory %q", iconDir)
		}
		if err := ioutil.WriteFile(iconFile, iconData, 0644); err != nil {
			return nil, err
		}
	}

	extDirs := make([]string, 0, num)
	for i := 0; i < num; i++ {
		extDir := filepath.Join(baseDir, fmt.Sprintf("fake_%d", i))
		if err := os.Mkdir(extDir, 0755); err != nil {
			return nil, errors.Wrapf(err, "failed to create the directory for %d-th extension", i)
		}
		if iconFile != "" {
			if err := os.Symlink(iconFile, filepath.Join(extDir, filepath.Base(iconFile))); err != nil {
				return nil, errors.Wrapf(err, "failed to create link of icon %q", iconFile)
			}
		}
		if err := ioutil.WriteFile(filepath.Join(extDir, "manifest.json"), []byte(fmt.Sprintf(manifestTmpl, i, iconJSON)), 0644); err != nil {
			return nil, errors.Wrapf(err, "failed to prepare manifest.json for %d-th extension", i)
		}
		extDirs = append(extDirs, extDir)
	}
	return extDirs, nil
}
