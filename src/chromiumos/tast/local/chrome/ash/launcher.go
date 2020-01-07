// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
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
type Accelerator string

// Accelerator key used to trigger launcher state change.
const (
	AccelSearch      Accelerator = "{keyCode: 'search', shift: false, control: false, alt: false, search: false, pressed: true}"
	AccelShiftSearch Accelerator = "{keyCode: 'search', shift: true, control: false, alt: false, search: false, pressed: true}"
)

// WaitForLauncherState waits until the launcher state becomes |state|.
func WaitForLauncherState(ctx context.Context, tconn *chrome.Conn, state LauncherState) error {
	expr := fmt.Sprintf(
		`tast.promisify(chrome.autotestPrivate.waitForLauncherState)('%s')`, state)
	if err := tconn.EvalPromise(ctx, expr, nil); err != nil {
		return errors.Wrap(err, "failed to wait for launcher state")
	}
	return nil
}

// TriggerLauncherStateChange will cause the launcher state change via accelerator.
func TriggerLauncherStateChange(ctx context.Context, tconn *chrome.Conn, accel Accelerator) error {
	expr := fmt.Sprintf(
		`(async () => {
                   var acceleratorKey=%s;
                   // Send the press event to store it in the history. It'll not be handled, so ignore the result.
                   chrome.autotestPrivate.activateAccelerator(acceleratorKey, () => {});
                   acceleratorKey.pressed = false;
                   await tast.promisify(chrome.autotestPrivate.activateAccelerator)(acceleratorKey);
                 })()`, accel)

	if err := tconn.EvalPromise(ctx, expr, nil); err != nil {
		return errors.Wrap(err, "failed to execute accelerator")
	}
	return nil
}

// PrepareDummyApps creates directories for |num| dummy apps (hosted apps) and
// returns their path names.
func PrepareDummyApps(num int) ([]string, error) {
	// The manifest.json data for the dummy hosted app; it just opens google.com
	// page on launch.
	manifestTmpl := `{
		"description": "dummy",
		"name": "dummy app %d",
		"manifest_version": 2,
		"version": "0",
		"app": {
			"launch": {
				"web_url": "https://www.google.com/"
			}
		}
	}`
	var extDirs []string
	for i := 0; i < num; i++ {
		extDir, err := ioutil.TempDir("", "dummy_extension")
		if err != nil {
			return nil, err
		}
		if err := ioutil.WriteFile(filepath.Join(extDir, "manifest.json"), []byte(fmt.Sprintf(manifestTmpl, i)), 0644); err != nil {
			return nil, errors.Wrapf(err, "failed to prepare manifest.json for %d-th extension", i)
		}
		extDirs = append(extDirs, extDir)
	}
	return extDirs, nil
}
