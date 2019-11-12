// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"fmt"

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
                   await new Promise((resolve, reject) => {
                     chrome.autotestPrivate.activateAccelerator(acceleratorKey, (ignored) => {
                       resolve();
                     })
                   });

                   acceleratorKey.pressed = false;
                   return tast.promisify(chrome.autotestPrivate.activateAccelerator)(acceleratorKey);
                 })();`, accel)

	if err := tconn.EvalPromise(ctx, expr, nil); err != nil {
		return errors.Wrap(err, "failed to execute accelerator")
	}
	return nil
}
