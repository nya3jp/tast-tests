// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"fmt"

	"chromiumos/tast/local/chrome"
)

// WindowStateType represents the different window state type in ash.
type WindowStateType string

// As defined in ash::WindowStateType here:
// https://cs.chromium.org/chromium/src/ash/public/cpp/window_state_type.h
const (
	WindowStateNormal       WindowStateType = "kNormal"
	WindowStateMinimized                    = "kMinimized"
	WindowStateMaximized                    = "kMaximized"
	WindowStateFullscreen                   = "kFullscreen"
	WindowStateLeftSnapped                  = "kLeftSnapped"
	WindowStateRightSnapped                 = "kRightSnapped"
)

// WMEventType represents the different WM Event type in ash.
type WMEventType string

// As defined in ash::wm::WMEventType here:
// https://cs.chromium.org/chromium/src/ash/wm/wm_event.h
const (
	WMEventNormal     WMEventType = "kWMEventNormal"
	WMEventMaximize               = "kWMEventMaxmize"
	WMEventMinimize               = "kWMEventMinimize"
	WMEventFullscreen             = "kWMEventFullscreen"
	WMEventSnapLeft               = "kWMEventSnapLeft"
	WMEventSnapRight              = "kWMEventSnapRight"
)

// SetArcAppWindowState Sends wm event to Arc app window to change its window state.
func SetArcAppWindowState(ctx context.Context, c *chrome.Conn, pkgName string, eventType WMEventType) (WindowStateType, error) {
	var state WindowStateType
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.setArcAppWindowState(%q, %q, function(state) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve(state);
		    }
		  });
		})`, pkgName, eventType)
	if err := c.EvalPromise(ctx, expr, &state); err != nil {
		return WindowStateNormal, err
	}
	return state, nil
}
