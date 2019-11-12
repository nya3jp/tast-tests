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

// LauncherState type
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

// Accelerator type
type Accelerator string

// Accelerator key used to trigger launcher state change.
const (
	AccelSearch      Accelerator = "{keyCode: 'search', shift: false, control: false, alt: false, search: false, pressed: true}"
	AccelShiftSearch Accelerator = "{keyCode: 'search', shift: true, control: false, alt: false, search: false, pressed: true}"
)

// WaitForLauncherState waits until the launcher state becomes |state|.
func WaitForLauncherState(ctx context.Context, tconn *chrome.Conn, state LauncherState) error {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
                   chrome.autotestPrivate.waitForLauncherState(
                     '%s',
                     function() {
                       resolve(true);
                     });
                 })`, state)

	finished := false
	if err := tconn.EvalPromise(ctx, expr, &finished); err != nil {
		return errors.Wrap(err, "failed to evaluate promise")
	}
	if !finished {
		return errors.New("error")
	}
	return nil
}

// TriggerLauncherStateChangeAndWait will cause the launcher state change via accelerator and
// waits until the launcher state becomes |state|.
func TriggerLauncherStateChangeAndWait(ctx context.Context, tconn *chrome.Conn, accel Accelerator, state LauncherState) error {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
                   var acceleratorKey=%s;
                   chrome.autotestPrivate.activateAccelerator(
                     acceleratorKey,
                     function(ignored) {
                       acceleratorKey.pressed = false;
                       chrome.autotestPrivate.activateAccelerator(
                         acceleratorKey,
                         function(success) {
                           if (!success) {
                             reject(new Error(chrome.runtime.lastError.message));
                           } else {
                             chrome.autotestPrivate.waitForLauncherState(
                               '%s',
                               function() {
                                 resolve(true);
                               });
                           }
                         });
                     });
                })`, accel, state)

	finished := false
	if err := tconn.EvalPromise(ctx, expr, &finished); err != nil {
		return errors.Wrap(err, "failed to evaluate promise")
	}
	if !finished {
		return errors.New("error")
	}
	return nil
}
