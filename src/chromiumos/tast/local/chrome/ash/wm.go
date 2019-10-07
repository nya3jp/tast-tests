// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// WindowStateType represents the different window state type in Ash.
type WindowStateType string

// As defined in ash::WindowStateType here:
// https://cs.chromium.org/chromium/src/ash/public/cpp/window_state_type.h
const (
	WindowStateNormal       WindowStateType = "Normal"
	WindowStateMinimized    WindowStateType = "Minimized"
	WindowStateMaximized    WindowStateType = "Maximized"
	WindowStateFullscreen   WindowStateType = "Fullscreen"
	WindowStateLeftSnapped  WindowStateType = "LeftSnapped"
	WindowStateRightSnapped WindowStateType = "RightSnapped"
	WindowStatePIP          WindowStateType = "PIP"
)

// WMEventType represents the different WM Event type in Ash.
type WMEventType string

// As defined in ash::wm::WMEventType here:
// https://cs.chromium.org/chromium/src/ash/wm/wm_event.h
const (
	WMEventNormal     WMEventType = "WMEventNormal"
	WMEventMaximize   WMEventType = "WMEventMaxmize"
	WMEventMinimize   WMEventType = "WMEventMinimize"
	WMEventFullscreen WMEventType = "WMEventFullscreen"
	WMEventSnapLeft   WMEventType = "WMEventSnapLeft"
	WMEventSnapRight  WMEventType = "WMEventSnapRight"
)

// SnapPosition represents the different snap position in split view.
type SnapPosition string

// As defined in ash::SplitViewController here:
// https://cs.chromium.org/chromium/src/ash/wm/splitview/split_view_controller.h
const (
	SnapPositionLeft  SnapPosition = "Left"
	SnapPositionRight SnapPosition = "Right"
)

// Rect represents the bounds of a window
// TODO(takise): We may be able to consolidate this with the one in display.go
type Rect struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// ArcAppWindowInfo represents the ARC window info as returned from Ash.
type ArcAppWindowInfo struct {
	Bounds      Rect `json:"bounds"`
	IsAnimating bool `json:"is_animating"`
}

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

// GetARCAppWindowInfo queries into Ash and returns the ARC window info.
// Currently, this returns information on the top window of a specified app.
func GetARCAppWindowInfo(ctx context.Context, c *chrome.Conn, pkgName string) (ArcAppWindowInfo, error) {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.getArcAppWindowInfo(%q, function(info) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve(info);
		    }
		  });
		})`, pkgName)

	var info ArcAppWindowInfo
	if err := c.EvalPromise(ctx, expr, &info); err != nil {
		return ArcAppWindowInfo{}, err
	}
	return ArcAppWindowInfo{info.Bounds, info.IsAnimating}, nil
}

// GetARCAppWindowState gets the Chrome side window state of the ARC app window with pkgName.
func GetARCAppWindowState(ctx context.Context, c *chrome.Conn, pkgName string) (WindowStateType, error) {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.getArcAppWindowState(%q, function(state) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve(state);
		    }
		  });
		})`, pkgName)

	var state WindowStateType
	if err := c.EvalPromise(ctx, expr, &state); err != nil {
		return WindowStateNormal, err
	}
	return state, nil
}

// SwapWindowsInSplitView swaps the positions of snapped windows in split view.
func SwapWindowsInSplitView(ctx context.Context, c *chrome.Conn) error {
	expr := `new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.swapWindowsInSplitView(function() {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve();
		    }
		  });
		})`
	return c.EvalPromise(ctx, expr, nil)
}

// WaitForSystemUIStabilized waits a bit until the system UI state is stabilized
// and ready for performance test. Some initialization might skew the performance
// result.
func WaitForSystemUIStabilized(ctx context.Context) error {
	// The duration to wait for system UI stabilized.
	const timeUntilSystemUIStabilized time.Duration = 5 * time.Second

	// Right now, it just waits a bit.
	// TODO(mukai, oshima): find the way to check the status and replace this by
	// testing.Poll().  See: https://crbug.com/1001314
	return testing.Sleep(ctx, timeUntilSystemUIStabilized)
}
