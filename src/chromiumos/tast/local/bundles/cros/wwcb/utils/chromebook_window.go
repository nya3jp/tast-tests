// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// MoveWindowToDisplay uses the mouse to drag window to certain display.
func MoveWindowToDisplay(ctx context.Context, tconn *chrome.TestConn, win *ash.Window, destDisp *display.Info) error {
	if win.DisplayID == destDisp.ID {
		return nil
	}

	testing.ContextLogf(ctx, "Moving window[%s] from [%s] to [%s]", win.Name, win.DisplayID, destDisp.ID)

	// Set up source display.
	sourceDispID := win.DisplayID
	sourceDispIndex, sourceDispType, err := getDispIndexAndType(ctx, tconn, sourceDispID)
	if err != nil {
		return errors.Wrap(err, "failed to find source display index and tpye")
	}

	// Set up destination display.
	destDispID := destDisp.ID
	destDispIndex, destDispType, err := getDispIndexAndType(ctx, tconn, destDispID)
	if err != nil {
		return errors.Wrap(err, "failed to find dest display index and tpye")
	}

	// Set up display layout.
	dispLayout, err := GetInternalAndExternalDisplays(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get interna & external display")
	}

	// Set window state as normal.
	if win.State != ash.WindowStateNormal {
		if _, err := ash.SetWindowState(ctx, tconn, win.ID, ash.WMEventNormal, true); err != nil {
			return errors.Wrap(err, "failed to set window state to normal")
		}
	}

	if win, err = ash.GetARCAppWindowInfo(ctx, tconn, win.ARCPackageName); err != nil {
		return errors.Wrap(err, "failed to get app's window info")
	}

	// Raw mouse API.
	m, err := input.Mouse(ctx)
	if err != nil {
		return err
	}

	cursor := cursorOnDisplay{arc.DefaultDisplayID, arc.InternalDisplay}

	// Move cursor to source display.
	if err := cursor.moveTo(ctx, tconn, m, sourceDispIndex, sourceDispType, dispLayout); err != nil {
		return errors.Wrap(err, "failed to move cursor to source display")
	}

	// Move cursor to window header bar.
	headerPoint := coords.NewPoint(win.BoundsInRoot.Left+win.BoundsInRoot.Width/2, win.BoundsInRoot.Top+win.CaptionHeight/2)
	if err := mouse.Move(tconn, headerPoint, 5)(ctx); err != nil {
		return errors.Wrap(err, "failed to move mouse to window header")
	}

	if err := mouse.Press(tconn, mouse.LeftButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to press mouse on left button")
	}

	// Move cursor to destination display
	if err := cursor.moveTo(ctx, tconn, m, destDispIndex, destDispType, dispLayout); err != nil {
		return errors.Wrap(err, "failed to move cursor to destination display")
	}

	// Move cursor to center of destination display.
	destDispBounds := dispLayout.DisplayInfo(arc.InternalDisplay).Bounds
	destPt := coords.NewPoint(destDispBounds.Width/2, destDispBounds.Height/2)
	if err := mouse.Move(tconn, destPt, time.Second)(ctx); err != nil {
		return errors.Wrap(err, "failed to move mouse to center of destination display")
	}

	if err := mouse.Release(tconn, mouse.LeftButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to release mouse")
	}

	if err := EnsureWindowStable(ctx, tconn, win.ARCPackageName, win); err != nil {
		return errors.Wrap(err, "failed to ensure window stable")
	}

	// Ensure window on destination display.
	if err := EnsureWindowOnDisplay(ctx, tconn, win.ARCPackageName, destDispID); err != nil {
		return errors.Wrap(err, "failed to ensure window on display")
	}
	return nil
}

// getDispIndexAndType returns display index and type.
func getDispIndexAndType(ctx context.Context, tconn *chrome.TestConn, dispID string) (int, arc.DisplayType, error) {
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return -1, "", nil
	}

	for i, info := range infos {
		if info.ID == dispID {
			if info.IsInternal {
				return i, arc.InternalDisplay, nil
			}
			return i, arc.ExternalDisplay, nil
		}
	}
	return -1, "", nil
}

// EnsureWindowOnDisplay checks whether a window is on the given display.
func EnsureWindowOnDisplay(ctx context.Context, tconn *chrome.TestConn, pkgName, dispID string) error {
	windowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, pkgName)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info")
	}
	if windowInfo.DisplayID != dispID {
		return errors.Errorf("invalid display ID; go %q, want %q", windowInfo.DisplayID, dispID)
	}
	return nil
}

// EnsureSetWindowState checks whether the window is in requested window state. If not, make sure to set window state to the requested window state.
func EnsureSetWindowState(ctx context.Context, tconn *chrome.TestConn, pkgName string, wantState ash.WindowStateType) error {
	if state, err := ash.GetARCAppWindowState(ctx, tconn, pkgName); err != nil {
		return err
	} else if state == wantState {
		return nil
	}
	windowEventMap := map[ash.WindowStateType]ash.WMEventType{
		ash.WindowStateNormal:     ash.WMEventNormal,
		ash.WindowStateMaximized:  ash.WMEventMaximize,
		ash.WindowStateMinimized:  ash.WMEventMinimize,
		ash.WindowStateFullscreen: ash.WMEventFullscreen,
	}
	wmEvent, ok := windowEventMap[wantState]
	if !ok {
		return errors.Errorf("didn't find the event for window state %q", wantState)
	}
	state, err := ash.SetARCAppWindowState(ctx, tconn, pkgName, wmEvent)
	if err != nil {
		return err
	}
	if state != wantState {
		return errors.Errorf("unexpected window state; got %s, want %s", state, wantState)
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, pkgName, wantState); err != nil {
		return errors.Wrapf(err, "failed to wait for activity to enter %v state", wantState)
	}
	return nil
}

// EnsureWindowStable checks whether the window moves its position.
func EnsureWindowStable(ctx context.Context, tconn *chrome.TestConn, pkgName string, wantWindow *ash.Window) error {
	windowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, pkgName)
	if err != nil {
		return errors.Wrapf(err, "failed to get window info for window %q", pkgName)
	}
	if !reflect.DeepEqual(windowInfo.BoundsInRoot, wantWindow.BoundsInRoot) || windowInfo.DisplayID != wantWindow.DisplayID {
		return errors.Errorf("window moves; got bounds %+v (displayID %q), want bounds %+v (displayID %q)", windowInfo.BoundsInRoot, windowInfo.DisplayID, wantWindow.BoundsInRoot, wantWindow.DisplayID)
	}
	return nil
}

// SwitchWindowToDisplay switches current window to expected display.
func SwitchWindowToDisplay(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, externalDisplay bool) action.Action {
	return func(ctx context.Context) error {
		var expectedRootWindow *nodewith.Finder
		var display string
		ui := uiauto.New(tconn)
		w, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			return w.IsActive && w.IsFrameVisible
		})
		if err != nil {
			return errors.Wrap(err, "failed to get current active window")
		}
		if externalDisplay {
			display = "external display"
			extendedWinClassName, err := extendedDisplayWindowClassName(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to find root window on external display")
			}
			expectedRootWindow = nodewith.ClassName(extendedWinClassName).Role(role.Window)
		} else {
			display = "internal display"
			// Root window on built-in display.
			expectedRootWindow = nodewith.ClassName("RootWindow-0").Role(role.Window)
		}
		currentWindow := nodewith.Name(w.Title).Role(role.Window)
		expectedWindow := currentWindow.Ancestor(expectedRootWindow).First()
		if err := ui.Exists(expectedWindow)(ctx); err != nil {
			testing.ContextLog(ctx, "Expected window not found: ", err)
			testing.ContextLogf(ctx, "Switch window %q to %s", w.Title, display)
			return uiauto.Combine("switch window to "+display,
				kb.AccelAction("Search+Alt+M"),
				ui.WithTimeout(3*time.Second).WaitUntilExists(expectedWindow),
			)(ctx)
		}
		return nil
	}
}

// extendedDisplayWindowClassName obtains the class name of the root window on the extended display.
// If multiple display windows are present, the first one will be returned.
func extendedDisplayWindowClassName(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	ui := uiauto.New(tconn)

	// Root window on extended display has the class name in RootWindow-<id> format.
	// We found extended display window could be RootWindow-1, or RootWindow-2.
	// Here we try 1 to 10.
	for i := 1; i <= 10; i++ {
		className := fmt.Sprintf("RootWindow-%d", i)
		win := nodewith.ClassName(className).Role(role.Window)
		if err := ui.Exists(win)(ctx); err == nil {
			return className, nil
		}
	}
	return "", errors.New("failed to find any window with class name RootWindow-1 to RootWindow-10")
}

// VerifyAllWindowsOnDisplay verifies all windows on certain display.
func VerifyAllWindowsOnDisplay(ctx context.Context, tconn *chrome.TestConn, externalDisplay bool) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		var displayInfo *display.Info
		if externalDisplay {
			infos, err := GetInternalAndExternalDisplays(ctx, tconn)
			if err != nil {
				return err
			}
			displayInfo = &infos.External
		} else {
			intDispInfo, err := display.GetInternalInfo(ctx, tconn)
			if err != nil {
				return err
			}
			displayInfo = intDispInfo
		}
		return ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
			if w.DisplayID != displayInfo.ID && w.IsVisible && w.IsFrameVisible {
				return errors.Errorf("window is not shown on certain display, got %s, want %s", w.DisplayID, displayInfo.ID)
			}
			return nil
		})
	}, &testing.PollOptions{
		Timeout:  WindowTimeout,
		Interval: WindowInterval,
	})
}
