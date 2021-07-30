// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"math"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// WindowStateType represents the different window state type in Ash.
type WindowStateType string

// As defined in ash::WindowStateType here:
// https://cs.chromium.org/chromium/src/ash/public/cpp/window_state_type.h
const (
	// Normal is actually used to represent both "Normal" and "Default".
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

// ErrWindowNotFound is returned when window is failed to be found.
var ErrWindowNotFound = errors.New("failed to find window")

// ErrMultipleWindowsFound is returned when multiple windows were returned when only one was requested.
var ErrMultipleWindowsFound = errors.New("found multiple matching windows")

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
	Bounds    coords.Rect `json:"bounds"`
	IsDragged bool        `json:"isDragged"`
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
	BoundsInRoot coords.Rect     `json:"boundsInRoot"`
	TargetBounds coords.Rect     `json:"targetBounds"`
	DisplayID    string          `json:"displayId"`

	Title            string `json:"title"`
	IsAnimating      bool   `json:"isAnimating"`
	IsVisible        bool   `json:"isVisible"`
	TargetVisibility bool   `json:"target_visibility"`
	CanFocus         bool   `json:"canFocus"`
	CanResize        bool   `json:"canResize"`

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
	FullRestoreWindowAppID     string              `json:"fullRestoreWindowAppId"`
}

var defaultPollOptions = &testing.PollOptions{Timeout: 20 * time.Second}

var stateToWmTypes = map[WindowStateType]WMEventType{
	WindowStateNormal:       WMEventNormal,
	WindowStateMinimized:    WMEventMinimize,
	WindowStateMaximized:    WMEventMaximize,
	WindowStateFullscreen:   WMEventFullscreen,
	WindowStateLeftSnapped:  WMEventSnapLeft,
	WindowStateRightSnapped: WMEventSnapRight,
}

// WMEventTypeForState returns the WMEventType to turn a window into the given
// state.
func WMEventTypeForState(state WindowStateType) WMEventType {
	return stateToWmTypes[state]
}

// SendWMEvent sends WMEvent to the window and returns the expected state.
func SendWMEvent(ctx context.Context, tconn *chrome.TestConn, id int, et WMEventType, waitForStateChange bool) (WindowStateType, error) {
	var state WindowStateType
	if err := tconn.Call(ctx, &state, "tast.promisify(chrome.autotestPrivate.setAppWindowState)", id, &windowStateChange{EventType: et}, waitForStateChange); err != nil {
		return WindowStateNormal, err
	}
	return state, nil
}

// SetWindowState requests changing the state of the window to the requested
// event type and returns the updated state.
func SetWindowState(ctx context.Context, tconn *chrome.TestConn, id int, et WMEventType) (WindowStateType, error) {
	return SendWMEvent(ctx, tconn, id, et, true /* waitForStateChange */)
}

// SetWindowStateAndWait requests a WMEvent to make the window for the id to be
// in the targetState, and wait for the window animations when it happens. It
// returns an error when it can't be in the target state. It will return nil
// when the window is already in the target state.
func SetWindowStateAndWait(ctx context.Context, tconn *chrome.TestConn, id int, targetState WindowStateType) error {
	gotState, err := SetWindowState(ctx, tconn, id, stateToWmTypes[targetState])
	if err != nil {
		return errors.Wrap(err, "failed to set the window state")
	}
	if gotState != targetState {
		return errors.Errorf("failed to set the window state: got %v want %v", gotState, targetState)
	}
	if err = WaitWindowFinishAnimating(ctx, tconn, id); err != nil {
		return errors.Wrap(err, "failed to wait for the window animation")
	}
	return nil
}

// SetWindowBounds requests changing the bounds of the window and which display it is on to the given values.
// It returns the actual bounds and display set, which may be different to the requested bounds and display.
// (e.g. setting bounds on an Android app may not have Android framework honour the request).
func SetWindowBounds(ctx context.Context, tconn *chrome.TestConn, id int, b coords.Rect, displayID string) (coords.Rect, string, error) {
	var result struct {
		Bounds    coords.Rect `json:"bounds"`
		DisplayID string      `json:"displayId"`
	}
	if err := tconn.Call(ctx, &result, "tast.promisify(chrome.autotestPrivate.setWindowBounds)", id, b, displayID); err != nil {
		if strings.Contains(err.Error(), "Cannot set bounds of window not in normal show state") {
			// See https://source.chromium.org/chromium/chromium/src/+/main:chrome/browser/chromeos/extensions/autotest_private/autotest_private_api.cc;l=580;drc=d6951ec895f1463826be4069fe7da51e296e4b81
			return coords.Rect{}, "", errors.New("cannot set bounds of window not in normal show state. Note that a window listed as normal show state in tast may actually be in default show state")
		}
		return coords.Rect{}, "", err
	}
	return result.Bounds, result.DisplayID, nil
}

// ActivateWindow requests to activate this window.
func (w *Window) ActivateWindow(ctx context.Context, tconn *chrome.TestConn) error {
	return tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.activateAppWindow)", w.ID)
}

// CloseWindow requests to close this window.
func (w *Window) CloseWindow(ctx context.Context, tconn *chrome.TestConn) error {
	return tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.closeAppWindow)", w.ID)
}

// SetARCAppWindowState sends WM event to ARC app window to change its window state, and returns the expected new state type.
func SetARCAppWindowState(ctx context.Context, tconn *chrome.TestConn, pkgName string, et WMEventType) (WindowStateType, error) {
	window, err := GetARCAppWindowInfo(ctx, tconn, pkgName)
	if err != nil {
		return WindowStateNormal, err
	}
	return SetWindowState(ctx, tconn, window.ID, et)
}

// GetARCAppWindowInfo queries into Ash and returns the ARC window info.
// Currently, this returns information on the top window of a specified app.
func GetARCAppWindowInfo(ctx context.Context, tconn *chrome.TestConn, pkgName string) (*Window, error) {
	return FindWindow(ctx, tconn, func(window *Window) bool {
		return window.ARCPackageName == pkgName
	})
}

// GetARCGhostWindowInfo queries into Ash and returns the ARC ghost window info by session id.
func GetARCGhostWindowInfo(ctx context.Context, tconn *chrome.TestConn, appID string) (*Window, error) {
	return FindWindow(ctx, tconn, func(window *Window) bool {
		return window.FullRestoreWindowAppID == appID
	})
}

// GetAllARCAppWindowsInfo queries into Ash and returns all of the ARC windows info.
func GetAllARCAppWindowsInfo(ctx context.Context, tconn *chrome.TestConn, pkgName string) ([]*Window, error) {
	return FindAllWindows(ctx, tconn, func(window *Window) bool {
		return window.ARCPackageName == pkgName
	})
}

// GetARCAppWindowState gets the Chrome side window state of the ARC app window with pkgName.
func GetARCAppWindowState(ctx context.Context, tconn *chrome.TestConn, pkgName string) (WindowStateType, error) {
	window, err := GetARCAppWindowInfo(ctx, tconn, pkgName)
	if err != nil {
		return WindowStateNormal, err
	}
	return window.State, nil
}

// GetAllARCAppWindowStates gets all the Chrome side window states of the ARC app windows with pkgName.
func GetAllARCAppWindowStates(ctx context.Context, tconn *chrome.TestConn, pkgName string) (windowStates []WindowStateType, err error) {
	windows, err := GetAllARCAppWindowsInfo(ctx, tconn, pkgName)
	if err != nil {
		return windowStates, err
	}
	for _, window := range windows {
		windowStates = append(windowStates, window.State)
	}
	return windowStates, nil
}

// WaitForARCAppWindowState waits for a window state to appear on the Chrome side. If you expect an Activity's window state
// to change, this method will guarantee that the state change has fully occurred and propagated to the Chrome side.
func WaitForARCAppWindowState(ctx context.Context, tconn *chrome.TestConn, pkgName string, state WindowStateType) error {
	return WaitForARCAppWindowStateWithPollOptions(ctx, tconn, pkgName, state, defaultPollOptions)
}

// WaitForARCAppWindowStateWithPollOptions waits for a window state to appear on the Chrome side. If you expect an Activity's window state
// to change, this method will guarantee that the state change has fully occurred and propagated to the Chrome side.
func WaitForARCAppWindowStateWithPollOptions(ctx context.Context, tconn *chrome.TestConn, pkgName string, state WindowStateType, pollOptions *testing.PollOptions) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		actual, err := GetARCAppWindowState(ctx, tconn, pkgName)
		if err != nil {
			// The window may not yet be known to the Chrome side, so don't stop polling here.
			return errors.Wrap(err, "failed to get Ash window state")
		}
		if actual != state {
			return errors.Errorf("window isn't in expected state yet; got: %s, want: %s", actual, state)
		}
		return nil
	}, pollOptions)
}

// WaitForVisible waits for a window to be visible on the Chrome side. Visibility is defined to be the corresponding
// Aura window's visibility.
func WaitForVisible(ctx context.Context, tconn *chrome.TestConn, pkgName string) error {
	return WaitForCondition(ctx, tconn, func(window *Window) bool {
		return window.ARCPackageName == pkgName && window.IsVisible
	}, defaultPollOptions)
}

// WaitForHidden waits for a window to be dismissed on the Chrome side. Visibility is defined to be the corresponding
// Aura window's visibility.
func WaitForHidden(ctx context.Context, tconn *chrome.TestConn, pkgName string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		ws, err := GetAllWindows(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get the window list"))
		}
		found := false
		for _, window := range ws {
			if window.ARCPackageName == pkgName && window.IsVisible {
				found = true
				break
			}
		}
		if found {
			return errors.New("there is still a visible window")
		}
		return nil
	}, defaultPollOptions)
}

// WaitWindowFinishAnimating waits for a window with a given ID to finish animating on the Chrome side.
func WaitWindowFinishAnimating(ctx context.Context, tconn *chrome.TestConn, windowID int) error {
	return WaitForCondition(ctx, tconn, func(window *Window) bool {
		return window.ID == windowID && !window.IsAnimating
	}, defaultPollOptions)
}

// WaitForFullScreen waits until any window that exists is in the full screen state.
func WaitForFullScreen(ctx context.Context, tconn *chrome.TestConn) error {
	return WaitForCondition(ctx, tconn, func(window *Window) bool {
		return window.State == WindowStateFullscreen
	}, defaultPollOptions)
}

// WaitForCondition waits for a window to satisfy the given predicate.
func WaitForCondition(ctx context.Context, tconn *chrome.TestConn, predicate func(window *Window) bool, pollOptions *testing.PollOptions) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		ws, err := GetAllWindows(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get the window list"))
		}
		for _, window := range ws {
			if predicate(window) {
				return nil
			}
		}
		return errors.New("no window satisfies the condition")
	}, pollOptions)
}

// SwapWindowsInSplitView swaps the positions of snapped windows in split view.
func SwapWindowsInSplitView(ctx context.Context, tconn *chrome.TestConn) error {
	return tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.swapWindowsInSplitView)")
}

// OverviewState represents the animation state of overview mode.
type OverviewState string

// OverviewState represents two final states for overview mode, when animations complete.
const (
	Shown  OverviewState = "Shown"
	Hidden OverviewState = "Hidden"
)

// WaitForOverviewState waits until overview is shown or hidden completely. Returns immediately if overview mode state matches |overview_state|.
func WaitForOverviewState(ctx context.Context, tconn *chrome.TestConn, state OverviewState, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.waitForOverviewState)", state); err != nil {
		return errors.Wrap(err, "failed to wait for overview state")
	}
	return nil
}

// InternalDisplayMode returns the display mode that is currently selected in the internal display.
func InternalDisplayMode(ctx context.Context, tconn *chrome.TestConn) (*display.DisplayMode, error) {
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
func GetAllWindows(ctx context.Context, tconn *chrome.TestConn) ([]*Window, error) {
	var windows []*Window
	if err := tconn.Call(ctx, &windows, "tast.promisify(chrome.autotestPrivate.getAppWindowList)"); err != nil {
		return nil, err
	}
	return windows, nil
}

// GetWindow is a utility function to return the info of the window for the
// given ID.
func GetWindow(ctx context.Context, tconn *chrome.TestConn, windowID int) (*Window, error) {
	ws, err := GetAllWindows(ctx, tconn)
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

// ForEachWindow runs a specified function on each window. If the given function returns an error, it is returned
// and f won't be called for following windows.
func ForEachWindow(ctx context.Context, tconn *chrome.TestConn, f func(window *Window) error) error {
	ws, err := GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the window list")
	}
	for _, window := range ws {
		if err := f(window); err != nil {
			return errors.Wrapf(err, "failure on window (%d) %q", window.ID, window.Title)
		}
	}
	return nil
}

// FindWindow returns the Chrome window with which the given predicate returns true.
// If there are multiple, this returns the first found window.
func FindWindow(ctx context.Context, tconn *chrome.TestConn, predicate func(*Window) bool) (*Window, error) {
	windows, err := GetAllWindows(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all windows")
	}
	for _, window := range windows {
		if predicate(window) {
			return window, nil
		}
	}
	return nil, ErrWindowNotFound
}

// FindOnlyWindow returns the Chrome window with which the given predicate returns true.
// If there are multiple, this returns an error.
func FindOnlyWindow(ctx context.Context, tconn *chrome.TestConn, predicate func(*Window) bool) (*Window, error) {
	windows, err := FindAllWindows(ctx, tconn, predicate)
	if err != nil {
		return nil, err
	}
	if len(windows) != 1 {
		return nil, ErrMultipleWindowsFound
	}
	return windows[0], err
}

// GetActiveWindow returns the active window.
func GetActiveWindow(ctx context.Context, tconn *chrome.TestConn) (*Window, error) {
	return FindOnlyWindow(ctx, tconn, func(w *Window) bool {
		return w.IsActive
	})
}

// FindAllWindows returns the Chrome windows with which the given predicate returns true.
func FindAllWindows(ctx context.Context, tconn *chrome.TestConn, predicate func(*Window) bool) (matchingWindows []*Window, err error) {
	windows, err := GetAllWindows(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all windows")
	}
	for _, window := range windows {
		if predicate(window) {
			matchingWindows = append(matchingWindows, window)
		}
	}
	return matchingWindows, nil
}

// ConnSource is an interface which allows new chrome.Conn connections to be created.
type ConnSource interface {
	NewConn(ctx context.Context, url string, opts ...cdputil.CreateTargetOption) (*chrome.Conn, error)
}

// CountVisibleWindows returns number of visible windows
func CountVisibleWindows(ctx context.Context, tconn *chrome.TestConn) (int, error) {
	visible := 0
	err := ForEachWindow(ctx, tconn, func(w *Window) error {
		if w.IsVisible {
			visible++
		}
		return nil
	})
	return visible, err
}

// CreateWindows create n browser windows with specified URL and wait for them to become visible.
// It will fail and return an error if at least one request fails to fulfill. Note that this will
// parallelize the requests to create windows, which may be bad if the caller
// wants to measure the performance of Chrome. This should be used for a
// preparation, before the measurement happens.
func CreateWindows(ctx context.Context, tconn *chrome.TestConn, cs ConnSource, url string, n int) error {
	prevvis, err := CountVisibleWindows(ctx, tconn)
	if err != nil {
		return err
	}

	g, dctx := errgroup.WithContext(ctx)
	conns := make([]*chrome.Conn, 0, n)
	defer func() {
		for i, c := range conns {
			if err := c.Close(); err != nil {
				testing.ContextLogf(ctx, "Failed to close the %d-th connection: %v", i, err)
			}
		}
	}()
	var mu sync.Mutex
	for i := 0; i < n; i++ {
		g.Go(func() error {
			conn, err := cs.NewConn(dctx, url, cdputil.WithNewWindow())
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
		return err
	}

	// N.B. This assumes that no existing windows will be closed during the duration of this function (CreateWindows).
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		nowvis, err := CountVisibleWindows(ctx, tconn)
		if err != nil {
			return testing.PollBreak(err)
		}

		if nowvis-prevvis != n {
			return errors.Errorf("failed to create %d visible windows - went from %d => %d visible windows, creating %d",
				n, prevvis, nowvis, nowvis-prevvis)
		}
		return nil
	}, defaultPollOptions); err != nil {
		return err
	}

	return nil
}

// DraggedWindowInOverview returns the window that is currently being dragged
// under overview mode. It is an error if no window is being dragged.
func DraggedWindowInOverview(ctx context.Context, tconn *chrome.TestConn) (*Window, error) {
	windows, err := GetAllWindows(ctx, tconn)
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
func SnappedWindows(ctx context.Context, tconn *chrome.TestConn) ([]*Window, error) {
	windows, err := GetAllWindows(ctx, tconn)
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

// FindFirstWindowInOverview returns the window which positioned the first item
// of the overview (i.e. appears at the top-left in the overview mode).
func FindFirstWindowInOverview(ctx context.Context, tconn *chrome.TestConn) (*Window, error) {
	ws, err := GetAllWindows(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the window list")
	}
	if len(ws) == 0 {
		return nil, errors.New("no windows exist")
	}
	var result *Window
	resultBounds := coords.NewRect(math.MaxInt32, math.MaxInt32, math.MaxInt32, math.MaxInt32)
	for _, w := range ws {
		if w.OverviewInfo == nil {
			continue
		}
		bounds := w.OverviewInfo.Bounds
		// This code is to find the leftmost one at the topmost row, but the windows
		// in the same row should have the exactly same top value, assuming that the
		// windows are arranged into a grid in the overview mode.
		if result == nil || (bounds.Left <= resultBounds.Left && bounds.Top <= resultBounds.Top) {
			result = w
			resultBounds = bounds
		}
	}
	if result == nil {
		return nil, errors.New("no windows are in overview mode")
	}
	return result, nil
}

// DragToShowOverview shows overview by dragging up, pausing for the gesture to be recognized, then ending the gesture.
// Note that this action only works in tablet mode.
func DragToShowOverview(ctx context.Context, tsw *input.TouchscreenEventWriter, stw *input.SingleTouchEventWriter, tconn *chrome.TestConn) error {
	if inTabletMode, err := TabletModeEnabled(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to get tablet-mode status")
	} else if !inTabletMode {
		return errors.New("this function does not support clamshell mode")
	}
	windows, err := GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get all windows")
	}
	if len(windows) == 0 {
		return errors.Wrap(err, "there must be at least one window to go to overview")
	}

	displayInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "unable to get display info")
	}

	tcc := tsw.NewTouchCoordConverter(displayInfo.Bounds.Size())
	if err != nil {
		return errors.Wrap(err, "unable to create touch coordinate converter")
	}

	// Make gesture duration sufficiently long for window drag not to be recognized as a gesture to the home screen.
	duration := time.Duration(displayInfo.Bounds.Height/3) * time.Millisecond

	start := displayInfo.Bounds.BottomCenter()
	startX, startY := tcc.ConvertLocation(start)

	end := displayInfo.Bounds.CenterPoint()
	endX, endY := tcc.ConvertLocation(end)

	testing.ContextLog(ctx, "Dragging from the bottom slowly to open overview")
	if err := stw.Swipe(ctx, startX, startY-1, endX, endY, duration); err != nil {
		return errors.Wrap(err, "failed to swipe")
	}

	// Wait with the swipe paused so the overview mode gesture is recognized. Use 1 second because this is roughly the amount of time it takes for the 'swipe up and hold' overview gesture to trigger.
	const pauseDuration = time.Second
	if err := testing.Sleep(ctx, pauseDuration); err != nil {
		return errors.Wrap(err, "failed to sleep while waiting for overview to trigger")
	}
	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to finish the swipe gesture")
	}

	// When the drag up ends overview is already fully shown. The only thing that remains is to wait for the windows to finish animating to their final point in the overview grid.
	for _, window := range windows {
		if err := WaitWindowFinishAnimating(ctx, tconn, window.ID); err != nil {
			return errors.Wrap(err, "failed to wait for the dragged window to animate")
		}
	}

	// Now that all windows are done animating, ensure overview is still shown.
	if err := WaitForOverviewState(ctx, tconn, Shown, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for animation to finish")
	}
	return nil
}

// DragToShowHomescreen shows the homescreen (app-list) by dragging up from the
// bottom of the screen quickly. Note that this action only works in tablet
// mode.
func DragToShowHomescreen(ctx context.Context, width, height input.TouchCoord, stw *input.SingleTouchEventWriter, tconn *chrome.TestConn) error {
	if inTabletMode, err := TabletModeEnabled(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to get tablet-mode status")
	} else if !inTabletMode {
		return errors.New("this function does not support clamshell mode")
	}
	windows, err := FindAllWindows(ctx, tconn, func(window *Window) bool {
		return window.IsVisible
	})
	if err != nil {
		return errors.Wrap(err, "failed to get all windows")
	}
	// Do nothing if there are no visible windows. Homescreen should be there already.
	if len(windows) == 0 {
		return nil
	}

	startX := width / 2
	startY := height - 1
	endX := startX
	endY := height / 2

	if err := stw.Swipe(ctx, startX, startY, endX, endY, 300*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to swipe")
	}
	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to finish the swipe gesture")
	}
	for _, window := range windows {
		if err := WaitWindowFinishAnimating(ctx, tconn, window.ID); err != nil {
			return errors.Wrap(err, "failed to wait for the dragged window to animate")
		}
	}
	return nil
}
