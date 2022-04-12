// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"reflect"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// MoveWindowToDisplay use mouse to drag window to certain display
func MoveWindowToDisplay(ctx context.Context, tconn *chrome.TestConn, win *ash.Window, destDisp *display.Info) error {
	// window is already on display
	if win.DisplayID == destDisp.ID {
		return nil
	}

	testing.ContextLogf(ctx, "Moving window[%s] from [%s] to [%s]", win.Name, win.DisplayID, destDisp.ID)

	// get display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}

	// declare destination display variables
	var sourceDispIndex, destDispIndex int
	var sourceDispType, destDispType arc.DisplayType

	sourceDispID := win.DisplayID
	destDispID := destDisp.ID

	// assign source & desination display's variables
	for i := range infos {
		// deal with source display
		if infos[i].ID == sourceDispID {
			// assign index
			sourceDispIndex = i
			// assign type
			if infos[i].IsInternal {
				sourceDispType = arc.InternalDisplay
			} else {
				sourceDispType = arc.ExternalDisplay
			}
		}
		// deal with destination display
		if infos[i].ID == destDispID {
			// assign index
			destDispIndex = i
			// assign type
			if infos[i].IsInternal {
				destDispType = arc.InternalDisplay
			} else {
				destDispType = arc.ExternalDisplay
			}
		}
	}

	// Setup display layout.
	disp, err := GetInternalAndExternalDisplays(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get interna & external display")
	}

	// if window state is not normal, then the window can't be moved
	if win.State != ash.WindowStateNormal {
		if _, err := ash.SetWindowState(ctx, tconn, win.ID, ash.WMEventNormal, true); err != nil {
			return errors.Wrap(err, "failed to set window state to normal")
		}
	}

	// window info might be changed after changing window state
	if win, err = ash.GetARCAppWindowInfo(ctx, tconn, win.ARCPackageName); err != nil {
		return errors.Wrap(err, "failed to get app's window info")
	}

	// Raw mouse API.
	m, err := input.Mouse(ctx)
	if err != nil {
		return err
	}

	// start from built-in display
	cursor := cursorOnDisplay{arc.DefaultDisplayID, arc.InternalDisplay}

	// move to source display
	if err := cursor.moveTo(ctx, tconn, m, sourceDispIndex, sourceDispType, disp); err != nil {
		return errors.Wrap(err, "failed to move cursor to source display")
	}

	// move to window header bar
	headerPoint := coords.NewPoint(win.BoundsInRoot.Left+win.BoundsInRoot.Width/2, win.BoundsInRoot.Top+win.CaptionHeight/2)
	if err := mouse.Move(tconn, headerPoint, 5)(ctx); err != nil {
		return errors.Wrap(err, "failed to move mouse to window header")
	}

	// press leftbutton
	if err := mouse.Press(tconn, mouse.LeftButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to press mouse on left button")
	}

	// move to destination display
	if err := cursor.moveTo(ctx, tconn, m, destDispIndex, destDispType, disp); err != nil {
		return errors.Wrap(err, "failed to move cursor to destination display")
	}

	// move to center of destination display
	destDispBounds := disp.DisplayInfo(arc.InternalDisplay).Bounds
	dstPt := coords.NewPoint(destDispBounds.Width/2, destDispBounds.Height/2)
	if err := mouse.Move(tconn, dstPt, time.Second)(ctx); err != nil {
		return errors.Wrap(err, "failed to move mouse to center of destination display")
	}

	// release left button
	if err := mouse.Release(tconn, mouse.LeftButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to release mouse")
	}

	if err := EnsureWindowStable(ctx, tconn, win.ARCPackageName, win); err != nil {
		return errors.Wrapf(err, "failed to ensure window[%s] is stable", win.ARCPackageName)
	}

	// ensure window on display
	if err := EnsureWindowOnDisplay(ctx, tconn, win.ARCPackageName, infos[destDispIndex].ID); err != nil {
		return errors.Wrapf(err, "failed to ensure [%s] window on display {seq:%d, ID:%s, Name:%s}",
			win.ARCPackageName, destDispIndex, infos[destDispIndex].ID, infos[destDispIndex].Name)
	}
	return nil
}

// EnsureWindowOnDisplay checks whether a window is on a certain display.
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

// ReopenAllWindowsOnInternal close all windows then open them on internal display
func ReopenAllWindowsOnInternal(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC) error {
	testing.ContextLog(ctx, "Close all windows and reopen two apps on interal display")

	// get all windows
	windows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get all windows ")
	}

	// close all windows
	for _, window := range windows {
		if err := window.CloseWindow(ctx, tconn); err != nil {
			return errors.Wrapf(err, "Failde to close [%s] window on display %s", window.ARCPackageName, window.DisplayID)
		}
	}

	// get internal display info
	intDispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display")
	}

	// start two activity on internal display
	for _, param := range []struct {
		pkgName string
		actName string
	}{
		{SettingsPkg, SettingsAct},
		{TestappPkg, TestappAct},
	} {
		testing.ContextLogf(ctx, "Start [%s] window on display - %s ", param.pkgName, intDispInfo.ID)

		// new activity on internal display
		act, err := arc.NewActivityOnDisplay(a, param.pkgName, param.actName, 0)
		if err != nil {
			return err
		}

		// start activity
		if err := act.Start(ctx, tconn); err != nil {
			return err
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {

			win, err := ash.GetARCAppWindowInfo(ctx, tconn, param.pkgName)
			if err != nil {
				return errors.Wrapf(err, "failed to get window[%s] info: ", param.pkgName)
			}

			if err := EnsureWindowStable(ctx, tconn, param.pkgName, win); err != nil {
				return errors.Wrap(err, "failed to ensure window is stable")
			}

			// ensure activity on internal display
			if err := EnsureWindowOnDisplay(ctx, tconn, param.pkgName, intDispInfo.ID); err != nil {
				return errors.Wrapf(err, "failed to ensure [%s] window on display {seq:%d, ID:%s, Name:%s}",
					param.pkgName, intDispInfo.ID, intDispInfo.ID, intDispInfo.Name)
			}

			// set window to normal state
			if err := act.SetWindowState(ctx, tconn, arc.WindowStateNormal); err != nil {
				return err
			}
			if err := ash.WaitForARCAppWindowState(ctx, tconn, param.pkgName, ash.WindowStateNormal); err != nil {
				return errors.Wrap(err, "failed to wait for GFXBench APP to be maximized")
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return err
		}
	}
	return nil
}
