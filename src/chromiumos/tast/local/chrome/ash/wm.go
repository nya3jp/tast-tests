// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/display"
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

// CaptionButtonStatus represents the bit mask flag in ArcAppWindowInfo
type CaptionButtonStatus uint

// As defined in views::CaptionButtonIcon here:
// https://cs.chromium.org/chromium/src/ui/views/window/caption_button_types.h
const (
	CaptionButtonMinimize CaptionButtonStatus = 1 << iota
	CaptionButtonMaximizeAndRestore
	CaptionButtonClose
	CaptionButtonLeftSnapped
	CaptionButtonRightSnapped
	CaptionButtonBack
	CaptionButtonLocation
	CaptionButtonMenu
	CaptionButtonZoom
	CaptionButtonCount
)

// String returns the CaptionButtonStatus string representation.
func (c *CaptionButtonStatus) String() string {
	ret := ""
	if *c&CaptionButtonMinimize != 0 {
		ret += "Minimize,"
	}
	if *c&CaptionButtonMaximizeAndRestore != 0 {
		ret += "MaximizeAndRestore,"
	}
	if *c&CaptionButtonClose != 0 {
		ret += "Close,"
	}
	if *c&CaptionButtonLeftSnapped != 0 {
		ret += "LeftSnapped,"
	}
	if *c&CaptionButtonRightSnapped != 0 {
		ret += "RightSnapped,"
	}
	if *c&CaptionButtonBack != 0 {
		ret += "Back,"
	}
	if *c&CaptionButtonLocation != 0 {
		ret += "Location,"
	}
	if *c&CaptionButtonMenu != 0 {
		ret += "Menu,"
	}
	if *c&CaptionButtonZoom != 0 {
		ret += "Zoom,"
	}
	if *c&CaptionButtonCount != 0 {
		ret += "Count,"
	}
	return ret
}

// Rect represents the bounds of a window
// TODO(takise): We may be able to consolidate this with the one in display.go
type Rect struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Width  int `json:"width"`
	Height int `json:"height"`
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

// OverviewInfo holds overview info of a window.
// https://cs.chromium.org/chromium/src/chrome/common/extensions/api/autotest_private.idl
type OverviewInfo struct {
	Bounds    Rect `json:"bounds"`
	IsDragged bool `json:"isDragged"`
}

// FrameMode represents the frame mode of the window.
type FrameMode string

// As defined in autotest_private.idl:
// https://cs.chromium.org/chromium/src/chrome/common/extensions/api/autotest_private.idl?q=FrameMode
const (
	FrameModeNormal    FrameMode = "Normal"
	FrameModeImmersive FrameMode = "Immersive"
)

// Window represents a normal window (i.e. browser windows or ARC app windows).
// As defined in AppWindowInfo in
// https://cs.chromium.org/chromium/src/chrome/common/extensions/api/autotest_private.idl
type Window struct {
	ID           int             `json:"id"`
	Name         string          `json:"name"`
	WindowType   WindowType      `json:"windowType"`
	State        WindowStateType `json:"stateType"`
	BoundsInRoot Rect            `json:"boundsInRoot"`
	TargetBounds Rect            `json:"targetBounds"`
	DisplayID    string          `json:"displayId"`

	Title            string `json:"title"`
	IsAnimating      bool   `json:"isAnimating"`
	IsVisible        bool   `json:"isVisible"`
	TargetVisibility bool   `json:"target_visibility"`
	CanFocus         bool   `json:"canFocus"`

	IsActive                   bool                `json:"isActive"`
	HasFocus                   bool                `json:"hasFocus"`
	OnActiveDesk               bool                `json:"onActiveDesk"`
	HasCapture                 bool                `json:"hasCapture"`
	CaptionHeight              int                 `json:"captionHeight"`
	CaptionButtonEnabledStatus CaptionButtonStatus `json:"captionButtonEnabledStatus"`
	CaptionButtonVisibleStatus CaptionButtonStatus `json:"captionButtonVisibleStatus"`
	ARCPackageName             string              `json:"arcPackageName"`
	OverviewInfo               *OverviewInfo       `json:"overviewInfo,omitempty"`
	IsFrameVisible             bool                `json:"isFrameVisible"`
	FrameMode                  FrameMode           `json:"FrameMode"`
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
	window, err := GetARCAppWindowInfo(ctx, c, pkgName)
	if err != nil {
		return WindowStateNormal, err
	}
	return SetWindowState(ctx, c, window.ID, et)
}

// GetARCAppWindowInfo queries into Ash and returns the ARC window info.
// Currently, this returns information on the top window of a specified app.
func GetARCAppWindowInfo(ctx context.Context, c *chrome.Conn, pkgName string) (*Window, error) {
	windows, err := GetAllWindows(ctx, c)
	if err != nil {
		return nil, err
	}
	for _, window := range windows {
		if window.ARCPackageName == pkgName {
			return window, nil
		}
	}
	return nil, errors.Errorf("failed to find the window for %q", pkgName)
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
	window, err := GetARCAppWindowInfo(ctx, c, pkgName)
	if err != nil {
		return WindowStateNormal, err
	}
	return window.State, nil
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

		if !info.IsVisible {
			return errors.New("the window is still invisible")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// WaitWindowFinishAnimating waits for a window with a given ID to finish animating on the Chrome side.
func WaitWindowFinishAnimating(ctx context.Context, c *chrome.Conn, windowID int) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		window, err := GetWindow(ctx, c, windowID)
		if err != nil {
			return err
		}

		if window.IsAnimating {
			return errors.New("the top window is still animating")
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Second})
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

// InternalDisplayMode returns the display mode that is currently selected in the internal display.
func InternalDisplayMode(ctx context.Context, tconn *chrome.Conn) (*display.DisplayMode, error) {
	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get internal display info")
	}
	for _, mode := range dispInfo.Modes {
		if mode.IsSelected {
			return mode, nil
		}
	}
	return nil, errors.New("failed to get selected mode")
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

// GetWindow is a utility function to return the info of the window for the
// given ID.
func GetWindow(ctx context.Context, c *chrome.Conn, windowID int) (*Window, error) {
	ws, err := GetAllWindows(ctx, c)
	if err != nil {
		return nil, err
	}
	for _, w := range ws {
		if w.ID == windowID {
			return w, nil
		}
	}
	return nil, errors.Errorf("failed to find the window with ID %d", windowID)
}

// GetWindowWithPredicate returns the Chrome window with which the given predicate returns true.
// If there are multiple, this returns the first found window.
func GetWindowWithPredicate(ctx context.Context, c *chrome.Conn, predicate func(*Window) bool) (*Window, error) {
	windows, err := GetAllWindows(ctx, c)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all windows")
	}
	for _, window := range windows {
		if predicate(window) {
			return window, nil
		}
	}
	return nil, errors.New("failed to find window")
}

// CreateWindows create n browser windows with specified URL. It will fail and
// return an error if at least one request fails to fulfill. Note that this will
// parallelize the requests to create windows, which may be bad if the caller
// wants to measure the performance of Chrome. This should be used for a
// preparation, before the measurement happens.
func CreateWindows(ctx context.Context, cr *chrome.Chrome, url string, n int) (chrome.Conns, error) {
	g, ctx := errgroup.WithContext(ctx)
	conns := chrome.Conns(make([]*chrome.Conn, 0, n))
	var mu sync.Mutex
	for i := 0; i < n; i++ {
		g.Go(func() error {
			conn, err := cr.NewConn(ctx, url, cdputil.WithNewWindow())
			if err != nil {
				return err
			}
			mu.Lock()
			defer mu.Unlock()
			conns = append(conns, conn)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		conns.Close()
		return nil, err
	}
	return conns, nil
}

// DraggedWindowInOverview returns the window that is currently being dragged
// under overview mode. It is an error if no window is being dragged.
func DraggedWindowInOverview(ctx context.Context, c *chrome.Conn) (*Window, error) {
	windows, err := GetAllWindows(ctx, c)
	if err != nil {
		return nil, err
	}

	for _, w := range windows {
		if w.OverviewInfo != nil && w.OverviewInfo.IsDragged {
			return w, nil
		}
	}
	return nil, errors.New("no dragged window in overview")
}

// SnappedWindows returns the snapped windows if any.
func SnappedWindows(ctx context.Context, c *chrome.Conn) ([]*Window, error) {
	windows, err := GetAllWindows(ctx, c)
	if err != nil {
		return nil, err
	}

	var snapped []*Window
	for _, w := range windows {
		if w.State == WindowStateLeftSnapped || w.State == WindowStateRightSnapped {
			snapped = append(snapped, w)
		}
	}
	return snapped, nil
}
