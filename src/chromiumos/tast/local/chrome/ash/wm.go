// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"encoding/json"
	"fmt"

	"chromiumos/tast/local/chrome"
)

// WindowStateType represents the different window state type in ASH.
type WindowStateType string

// As defined in ash::WindowStateType here:
// https://cs.chromium.org/chromium/src/ash/public/cpp/window_state_type.h
const (
	WindowStateNormal       WindowStateType = "Normal"
	WindowStateMinimized                    = "Minimized"
	WindowStateMaximized                    = "Maximized"
	WindowStateFullscreen                   = "Fullscreen"
	WindowStateLeftSnapped                  = "LeftSnapped"
	WindowStateRightSnapped                 = "RightSnapped"
)

// WMEventType represents the different WM Event type in ash.
type WMEventType string

// As defined in ash::wm::WMEventType here:
// https://cs.chromium.org/chromium/src/ash/wm/wm_event.h
const (
	WMEventNormal     WMEventType = "WMEventNormal"
	WMEventMaximize               = "WMEventMaxmize"
	WMEventMinimize               = "WMEventMinimize"
	WMEventFullscreen             = "WMEventFullscreen"
	WMEventSnapLeft               = "WMEventSnapLeft"
	WMEventSnapRight              = "WMEventSnapRight"
)

// WindowStateChange represents the change sent to chrome.autotestPrivate.setArcAppWindowState function.
type windowStateChange struct {
	EventType      WMEventType `json:"eventType"`
	FailIfNoChange bool        `json:"failIfNoChange,omitempty"`
}

// SetARCAppWindowState sends WM event to ARC app window to change its window state, and returns the expected new state type.
func SetARCAppWindowState(ctx context.Context, c *chrome.Conn, pkgName string, et WMEventType) (WindowStateType, error) {
	change, err := json.Marshal(&windowStateChange{EventType: et})
	if err != nil {
		return WindowStateNormal, err
	}

	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.setArcAppWindowState(%q, %s, function(state) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve(state);
		    }
		  });
		})`, pkgName, string(change))

	var state WindowStateType
	if err := c.EvalPromise(ctx, expr, &state); err != nil {
		return WindowStateNormal, err
	}
	return state, nil
}
