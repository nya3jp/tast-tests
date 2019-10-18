// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"chromiumos/tast/errors"
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
	WMEventMaximize   WMEventType = "WMEventMaximize"
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
	Visible     bool   `json:"is_visible"`
	Bounds      Rect   `json:"bounds"`
	IsAnimating bool   `json:"is_animating"`
	DisplayID   string `json:"display_id"`
}

// WindowStateChange represents the change sent to chrome.autotestPrivate.setArcAppWindowState function.
type windowStateChange struct {
	EventType      WMEventType `json:"eventType"`
	FailIfNoChange bool        `json:"failIfNoChange,omitempty"`
}

// WindowType represents the type of a window.
type WindowType string

// As defined in ash::AppType here:
// https://cs.chromium.org/chromium/src/ash/public/cpp/app_types.h
const (
	WindowTypeBrowser   WindowType = "Browser"
	WindowTypeChromeApp WindowType = "ChromeApp"
	WindowTypeArc       WindowType = "ArcApp"
	WindowTypeCrostini  WindowType = "CrostiniApp"
	WindowTypeSystem    WindowType = "SystemApp"
	WindowTypeExtension WindowType = "ExtensionApp"
)

// Window represents a normal window (i.e. browser windows or ARC app windows).
type Window struct {
	ID           int             `json:"id"`
	Name         string          `json:"name"`
	WindowType   WindowType      `json:"window_type"`
	State        WindowStateType `json:"state_type"`
	BoundsInRoot Rect            `json:"bounds_in_root"`
	TargetBounds Rect            `json:"target_bounds"`
	DisplayID    string          `json:"display_id"`

	Title            string `json:"title"`
	IsAnimating      bool   `json:"is_animating"`
	IsVisible        bool   `json:"is_visible"`
	TargetVisibility bool   `json:"target_visibility"`
	CanFocus         bool   `json:"can_focus"`

	IsActive       bool   `json:"is_active"`
	HasFocus       bool   `json:"has_focus"`
	OnActiveDesk   bool   `json:"on_active_desk"`
	HasCapture     bool   `json:"has_capture"`
	ARCPackageName string `json:"arc_package_name"`
}

// SetWindowState requests changing the state of the window to the requested
// event type and returns the updated state.
func SetWindowState(ctx context.Context, c *chrome.Conn, id int, et WMEventType) (WindowStateType, error) {
	change, err := json.Marshal(&windowStateChange{EventType: et})
	if err != nil {
		return WindowStateNormal, err
	}

	expr := fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.setAppWindowState)(%d, %s)`, id, string(change))
	var state WindowStateType
	if err = c.EvalPromise(ctx, expr, &state); err != nil {
		return WindowStateNormal, err
	}
	return state, nil
}

// CloseWindow requests to close this window.
func (w *Window) CloseWindow(ctx context.Context, c *chrome.Conn) error {
	return c.EvalPromise(ctx, fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.closeAppWindow)(%d)`, w.ID), nil)
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
	return ArcAppWindowInfo{info.Visible, info.Bounds, info.IsAnimating, info.DisplayID}, nil
}

// ConvertBoundsFromDpToPx converts the given bounds in DP to pixles based on the given device scale factor.
func ConvertBoundsFromDpToPx(bounds Rect, dsf float64) Rect {
	return Rect{
		int(math.Round(float64(bounds.Left) * dsf)),
		int(math.Round(float64(bounds.Top) * dsf)),
		int(math.Round(float64(bounds.Width) * dsf)),
		int(math.Round(float64(bounds.Height) * dsf))}
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

// WaitForARCAppWindowState waits for a window state to appear on the Chrome side. If you expect an Activity's window state
// to change, this method will guarantee that the state change has fully occurred and propagated to the Chrome side.
func WaitForARCAppWindowState(ctx context.Context, c *chrome.Conn, pkgName string, state WindowStateType) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		actual, err := GetARCAppWindowState(ctx, c, pkgName)
		if err != nil {
			// The window may not yet be known to the Chrome side, so don't stop polling here.
			return errors.Wrap(err, "failed to get Ash window state")
		}
		if actual != state {
			return errors.Errorf("window isn't in expected state yet; got: %s, want: %s", state, actual)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// WaitForVisible waits for a window to be visible on the Chrome side. Visibility is defined to be the corresponding
// Aura window's visibility.
func WaitForVisible(ctx context.Context, c *chrome.Conn, pkgName string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		info, err := GetARCAppWindowInfo(ctx, c, pkgName)
		if err != nil {
			// The window may not yet be known to the Chrome side, so don't stop polling here.
			return errors.Wrap(err, "failed to get ARC window info")
		}

		if !info.Visible {
			return errors.New("the window is still invisible")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
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

// GetAllWindows queries Chrome to list all of the app windows currently in the
// system.
func GetAllWindows(ctx context.Context, c *chrome.Conn) ([]*Window, error) {
	var windows []*Window
	if err := c.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.getAppWindowList)()`, &windows); err != nil {
		return nil, err
	}
	return windows, nil
}
