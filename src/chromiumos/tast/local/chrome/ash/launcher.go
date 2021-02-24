// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
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

func newColor(r, g, b uint8) color.RGBA {
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

// PrepareFakeApps creates directories for num fake apps (hosted apps) under
// the directory of baseDir and returns their path names. The intermediate
// data may remain even when an error is returned. It is the caller's
// responsibility to clean up the contents under the baseDir. This also may
// update the ownership of baseDir.
func PrepareFakeApps(baseDir string, num int) ([]string, error) {
	// The manifest.json data for the fake hosted app; it just opens google.com
	// page on launch.
	const manifestTmpl = `{
		"description": "fake",
		"name": "fake app %d",
		"manifest_version": 2,
		"version": "0",
		"icons": %s,
		"app": {
			"launch": {
				"web_url": "https://www.google.com/"
			}
		}
	}`
	if err := extension.ChownContentsToChrome(baseDir); err != nil {
		return nil, errors.Wrapf(err, "failed to change ownership of %s", baseDir)
	}

	// creating icon images for the fake apps. To be a bit realistic, the icon
	// is a circle in which diagonal stripes are drawn.
	// Icons are shared among all fake apps. Icons are created in its own
	// directory, and each app directory has symlinks to those icon files.
	iconDir := filepath.Join(baseDir, "icons")
	if err := os.Mkdir(iconDir, 0755); err != nil {
		return nil, errors.Wrapf(err, "failed to create the icon directory %q", iconDir)
	}
	sizes := []int{32, 48, 64, 96, 128, 192, 256}
	iconNames := make(map[int]string, len(sizes))

	colors := []color.Color{
		newColor(255, 0, 0), newColor(0, 255, 0), newColor(0, 0, 255),
		newColor(255, 255, 0), newColor(255, 0, 255), newColor(0, 255, 255)}
	for _, siz := range sizes {
		img := image.NewRGBA(image.Rect(0, 0, siz, siz))
		for x := 0; x < siz; x++ {
			for y := 0; y < siz; y++ {
				// Do not draw outside of the circle.
				if (x-siz/2)*(x-siz/2)+(y-siz/2)*(y-siz/2) > siz*siz/4 {
					continue
				}
				// Choose the color with diagonal stripe.
				c := colors[((x+y)*7/siz)%len(colors)]
				img.Set(x, y, c)
			}
		}
		iconName := fmt.Sprintf("icon%d.png", siz)
		iconNames[siz] = iconName
		// Save the icon image into the iconDir in png format.
		if err := func() error {
			f, err := os.OpenFile(filepath.Join(iconDir, iconName), os.O_WRONLY|os.O_CREATE, 0644)
			if err != nil {
				return err
			}
			defer f.Close()
			return png.Encode(f, img)
		}(); err != nil {
			return nil, errors.Wrapf(err, "failed to save the icon file %q", iconName)
		}
	}
	// iconJSON is the part of JSON data in the manifest.json file.
	iconJSON, err := json.Marshal(iconNames)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal icon names")
	}

	extDirs := make([]string, 0, num)
	for i := 0; i < num; i++ {
		extDir := filepath.Join(baseDir, fmt.Sprintf("fake_%d", i))
		if err := os.Mkdir(extDir, 0755); err != nil {
			return nil, errors.Wrapf(err, "failed to create the directory for %d-th extension", i)
		}
		// Create symlinks for the icons.
		for _, iconName := range iconNames {
			if err := os.Symlink(filepath.Join(iconDir, iconName), filepath.Join(extDir, iconName)); err != nil {
				return nil, errors.Wrapf(err, "failed to create link of icon %q for %d-th extension", iconName, i)
			}
		}
		if err := ioutil.WriteFile(filepath.Join(extDir, "manifest.json"), []byte(fmt.Sprintf(manifestTmpl, i, string(iconJSON))), 0644); err != nil {
			return nil, errors.Wrapf(err, "failed to prepare manifest.json for %d-th extension", i)
		}
		extDirs = append(extDirs, extDir)
	}
	return extDirs, nil
}
