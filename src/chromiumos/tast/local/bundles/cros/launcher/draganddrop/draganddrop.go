// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package draganddrop contains interface and implementions of app drag and drop in launcher.
package draganddrop

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const drapDropDuration = time.Second

// DragAndDrop defines the interface to implement drag and drop applications.
type DragAndDrop interface {
	// DragFirstIconToThirdIcon drags app icon from the first position of app list to the third position of app list
	DragFirstIconToThirdIcon(ctx context.Context) error

	// DragFirstIconToNextPage drags an icon to the next page of the app list.
	DragFirstIconToNextPage(ctx context.Context) error

	// VerifySecondPage verifies that the second page exist or not.
	VerifySecondPage(ctx context.Context) error

	// Close releases the underlying resouses.
	Close()
}

// TouchHandler holds resources for dragging and dropping on app icons with touch.
type TouchHandler struct {
	tconn    *chrome.TestConn
	ui       *uiauto.Context
	pc       pointer.Context
	tew      *input.TouchscreenEventWriter
	stw      *input.SingleTouchEventWriter
	touchCtx *touch.Context
}

// NewTouchHandler creates a new instance of TouchHandler.
func NewTouchHandler(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) (*TouchHandler, error) {
	var (
		succ     bool
		err      error
		pc       pointer.Context
		tew      *input.TouchscreenEventWriter
		stw      *input.SingleTouchEventWriter
		touchCtx *touch.Context
	)

	defer func() {
		if succ {
			return
		}
		if touchCtx != nil {
			if err := touchCtx.Close(); err != nil {
				testing.ContextLog(ctx, "Failed to close touch context")
			}
		}
		if stw != nil {
			stw.Close()
		}
		if tew != nil {
			if err := tew.Close(); err != nil {
				testing.ContextLog(ctx, "Failed to close touch event writer")
			}
		}
		if pc != nil {
			if err := pc.Close(); err != nil {
				testing.ContextLog(ctx, "Failed to close pointer context")
			}
		}
	}()

	if pc, err = pointer.NewTouch(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to create a touch controller")
	}

	if tew, err = input.Touchscreen(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to access to the touch screen")
	}

	if stw, err = tew.NewSingleTouchWriter(); err != nil {
		return nil, errors.Wrap(err, "failed to create a single touch writer")
	}

	if touchCtx, err = touch.New(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to get touch screen")
	}

	succ = true

	return &TouchHandler{
		tconn:    tconn,
		ui:       uiauto.New(tconn),
		pc:       pc,
		tew:      tew,
		stw:      stw,
		touchCtx: touchCtx,
	}, nil
}

// Close releases the underlying resouses.
func (t *TouchHandler) Close() {
	t.touchCtx.Close()
	t.stw.Close()
	t.tew.Close()
	t.pc.Close()
	t.tconn.Close()
}

// DragFirstIconToThirdIcon drags app icon from the first position of app list to the third position of app list with touch.
func (t *TouchHandler) DragFirstIconToThirdIcon(ctx context.Context) error {
	src := nodewith.HasClass("AppListItemView").First()

	appInfo, err := t.ui.Info(ctx, src)
	if err != nil {
		return errors.Wrap(err, "failed to get the information of the app you dragging")
	}

	start, err := t.ui.Location(ctx, src)
	if err != nil {
		return errors.Wrap(err, "failed to get location for src icon")
	}
	locBefore := start

	end, err := t.ui.Location(ctx, nodewith.HasClass("AppListItemView").Nth(2))
	if err != nil {
		return errors.Wrap(err, "failed to get location for the third icon")
	}

	// Longpress and swipe the icon to the endpoint.
	if err := t.touchCtx.Swipe(start.CenterPoint(), t.touchCtx.Hold(drapDropDuration), t.touchCtx.SwipeTo(end.CenterPoint(), drapDropDuration))(ctx); err != nil {
		return errors.Wrap(err, "failed to longpress and swpie the icon")
	}

	// Verify the dropped app is in new position by checking its position before and after drag-and-drop.
	iconAfterDrag := launcher.AppItemViewFinder(appInfo.Name)
	locAfter, err := t.ui.Location(ctx, iconAfterDrag)
	if err != nil {
		return errors.Wrap(err, "failed to get location for icon after dragging")
	}
	if (locBefore.CenterX() == locAfter.CenterX()) && (locBefore.CenterY() == locAfter.CenterY()) {
		return errors.New("dropped app is not at new position")
	}
	return nil
}

// DragFirstIconToNextPage drags an icon to the next page of the app list with touch.
func (t *TouchHandler) DragFirstIconToNextPage(ctx context.Context) error {
	src := nodewith.HasClass("AppListItemView").First()

	start, err := t.ui.Location(ctx, src)
	if err != nil {
		return errors.Wrap(err, "failed to get location for src icon")
	}
	end, err := t.ui.Location(ctx, nodewith.HasClass("AppsGridView"))
	if err != nil {
		return errors.Wrap(err, "failed to get location for AppsGridView")
	}
	endPoint := coords.NewPoint(end.CenterPoint().X, end.Bottom())

	gestures := []action.Action{t.touchCtx.Hold(drapDropDuration), t.touchCtx.SwipeTo(endPoint, drapDropDuration), t.touchCtx.Hold(drapDropDuration)}
	return t.touchCtx.Swipe(start.CenterPoint(), gestures...)(ctx)
}

// VerifySecondPage verifies that the second page exist or not.
func (t *TouchHandler) VerifySecondPage(ctx context.Context) error {
	return verifySecondPageExist(ctx, t.tconn, t.ui, t.pc)
}

// MouseHandler holds resources for dragging and dropping on app icons with mouse.
type MouseHandler struct {
	tconn *chrome.TestConn
	ui    *uiauto.Context
	pc    pointer.Context
	kw    *input.KeyboardEventWriter
}

// NewMouseHandler creates a new instance of MouseHandler.
func NewMouseHandler(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) (*MouseHandler, error) {
	var (
		succ bool
		err  error
		pc   pointer.Context
		kw   *input.KeyboardEventWriter
	)

	defer func() {
		if succ {
			return
		}
		if kw != nil {
			if err := kw.Close(); err != nil {
				testing.ContextLog(ctx, "Failed to close keyboard event writer")
			}
		}
		if pc != nil {
			if err := pc.Close(); err != nil {
				testing.ContextLog(ctx, "Failed to close pointer context")
			}
		}
	}()

	pc = pointer.NewMouse(tconn)

	if kw, err = input.Keyboard(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to take keyboard")
	}

	succ = true

	return &MouseHandler{
		tconn: tconn,
		ui:    uiauto.New(tconn),
		pc:    pc,
		kw:    kw,
	}, nil
}

// Close releases the underlying resouses.
func (m *MouseHandler) Close() {
	m.kw.Close()
	m.pc.Close()
	m.tconn.Close()
}

// DragFirstIconToThirdIcon drags app icon from the first position of app list to the third position of app list with mouse.
func (m *MouseHandler) DragFirstIconToThirdIcon(ctx context.Context) error {
	return launcher.DragIconToIcon(m.tconn, 0, 2)(ctx)
}

// DragFirstIconToNextPage drags an icon to the next page of the app list with mouse.
func (m *MouseHandler) DragFirstIconToNextPage(ctx context.Context) error {
	return launcher.DragIconToNextPage(m.tconn)(ctx)
}

// VerifySecondPage verifies that the second page exist or not.
func (m *MouseHandler) VerifySecondPage(ctx context.Context) error {
	return verifySecondPageExist(ctx, m.tconn, m.ui, m.pc)
}

// verifySecondPageExist verifies that the second page exist or not.
func verifySecondPageExist(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, pc pointer.Context) error {
	pageSwitcher := nodewith.HasClass("PageSwitcher")
	pageButtons := nodewith.HasClass("Button").Ancestor(pageSwitcher)

	// The existing of page button means that we drag the app to next page sucessfully.
	if err := ui.WaitForEvent(pageSwitcher, event.Alert, pc.Click(pageButtons.First()))(ctx); err != nil {
		return errors.Wrap(err, "failed to click the page button")
	}
	return nil
}
