// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type uiOperationType int

const (
	uiMouse uiOperationType = iota
	uiTouch
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     AppDragAndDrops,
		Desc:     "Test the functionality of dragging and dropping on app icons",
		Contacts: []string{"kyle.chen@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Fixture:  "chromeLoggedIn",
		Params: []testing.Param{
			{
				Name: "mouse",
				Val:  uiMouse,
			}, {
				Name: "touch",
				Val:  uiTouch,
			},
		},
	})
}

const drapDropDuration = time.Second

// AppDragAndDrops tests the functionality of dragging and dropping on app icons.
func AppDragAndDrops(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	opType := s.Param().(uiOperationType)
	isTouch := (opType == uiTouch)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	var draganddrop dragAndDrop
	if isTouch {
		if err := ash.SetTabletModeEnabled(ctx, tconn, true); err != nil {
			s.Fatal("Failed to set tablet mode enabled: ", err)
		}

		if draganddrop, err = newDragAndDropByTouch(ctx, cr, tconn); err != nil {
			s.Fatal("Failed to new a interface by touch: ", err)
		}
	} else {
		if draganddrop, err = newDragAndDropByMouse(ctx, cr, tconn); err != nil {
			s.Fatal("Failed to new a interface by mouse: ", err)
		}
	}
	defer draganddrop.close()

	cleanupCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer faillog.SaveScreenshotOnError(cleanupCtx, cr, s.OutDir(), s.HasError)
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	if err := draganddrop.openLauncher(ctx); err != nil {
		s.Fatal("Failed to open the launcher: ", err)
	}

	targetApp := nodewith.ClassName("AppListItemView").First()

	if err := draganddrop.dragNodeToCenter(ctx, targetApp); err != nil {
		s.Fatal("Failed to drag icon to center: ", err)
	}

	if err := draganddrop.dragToNextPage(ctx, targetApp); err != nil {
		s.Fatal("Failed to drag icon to next page: ", err)
	}

	if err := draganddrop.verifySecondPage(ctx); err != nil {
		s.Fatal("Failed to return the first page: ", err)
	}
}

type dragAndDrop interface {
	dragNodeToCenter(ctx context.Context, src *nodewith.Finder) error
	dragToNextPage(ctx context.Context, src *nodewith.Finder) error
	openLauncher(ctx context.Context) error
	verifySecondPage(ctx context.Context) error
	close()
}

type dragAndDropByTouch struct {
	tconn    *chrome.TestConn
	ui       *uiauto.Context
	pc       pointer.Context
	tew      *input.TouchscreenEventWriter
	stw      *input.SingleTouchEventWriter
	touchCtx *touch.Context
}

// newDragAndDropByTouch implemented a interface for touch.
func newDragAndDropByTouch(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) (*dragAndDropByTouch, error) {
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

	return &dragAndDropByTouch{
		tconn:    tconn,
		ui:       uiauto.New(tconn),
		pc:       pc,
		tew:      tew,
		stw:      stw,
		touchCtx: touchCtx,
	}, nil
}

// close runs multiple Close() for touch method.
func (t *dragAndDropByTouch) close() {
	t.touchCtx.Close()
	t.stw.Close()
	t.tew.Close()
	t.pc.Close()
	t.tconn.Close()
}

// dragNodeToCenter drags from the src item to the center of display with touch.
func (t *dragAndDropByTouch) dragNodeToCenter(ctx context.Context, src *nodewith.Finder) error {
	locBefore, err := t.ui.Location(ctx, src)
	if err != nil {
		return errors.Wrap(err, "failed to get location for icon before dragging")
	}
	appInfo, err := t.ui.Info(ctx, src)
	if err != nil {
		return errors.Wrap(err, "failed to get the information of the app you dragging")
	}

	start, err := t.ui.Location(ctx, src)
	if err != nil {
		return errors.Wrap(err, "failed to get location for src icon")
	}
	info, err := display.GetPrimaryInfo(ctx, t.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find the primary display info")
	}
	endPoint := coords.NewPoint(info.Bounds.CenterX(), info.Bounds.CenterY())

	// Longpress and swipe the icon to the endpoint.
	if err := t.touchCtx.Swipe(start.CenterPoint(), t.touchCtx.Hold(drapDropDuration), t.touchCtx.SwipeTo(endPoint, drapDropDuration))(ctx); err != nil {
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

// dragToNextPage drags an icon to the next page of the app list with touch.
func (t *dragAndDropByTouch) dragToNextPage(ctx context.Context, src *nodewith.Finder) error {
	start, err := t.ui.Location(ctx, src)
	if err != nil {
		return errors.Wrap(err, "failed to get location for src icon")
	}
	end, err := t.ui.Location(ctx, nodewith.ClassName("AppsGridView"))
	if err != nil {
		return errors.Wrap(err, "failed to get location for AppsGridView")
	}
	endPoint := coords.NewPoint(end.CenterPoint().X, end.Bottom())

	gestures := []action.Action{t.touchCtx.Hold(drapDropDuration), t.touchCtx.SwipeTo(endPoint, drapDropDuration), t.touchCtx.Hold(drapDropDuration)}
	return t.touchCtx.Swipe(start.CenterPoint(), gestures...)(ctx)
}

// openLauncher opens launcher for dragging by touch.
func (t *dragAndDropByTouch) openLauncher(ctx context.Context) error {
	return ash.DragToShowHomescreen(ctx, t.tew.Width(), t.tew.Height(), t.stw, t.tconn)
}

// verifySecondPage verifies that the second page exist or not.
func (t *dragAndDropByTouch) verifySecondPage(ctx context.Context) error {
	return verifySecondPageExist(ctx, t.tconn, t.ui, t.pc)
}

type dragAndDropByMouse struct {
	tconn *chrome.TestConn
	ui    *uiauto.Context
	pc    pointer.Context
	kw    *input.KeyboardEventWriter
}

// newDragAndDropByMouse implemented a interface for mouse.
func newDragAndDropByMouse(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) (*dragAndDropByMouse, error) {
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

	return &dragAndDropByMouse{
		tconn: tconn,
		ui:    uiauto.New(tconn),
		pc:    pc,
		kw:    kw,
	}, nil
}

// close runs multiple Close() for mouse method.
func (m *dragAndDropByMouse) close() {
	m.kw.Close()
	m.pc.Close()
	m.tconn.Close()
}

// dragNodeToCenter drags from the src item to the center of display with mouse.
func (m *dragAndDropByMouse) dragNodeToCenter(ctx context.Context, src *nodewith.Finder) error {
	locBefore, err := m.ui.Location(ctx, src)
	if err != nil {
		return errors.Wrap(err, "failed to get location for icon before dragging")
	}
	appInfo, err := m.ui.Info(ctx, src)
	if err != nil {
		return errors.Wrap(err, "failed to get the information of the app you dragging")
	}

	start, err := m.ui.Location(ctx, src)
	if err != nil {
		return errors.Wrap(err, "failed to get location for src icon")
	}
	if err := mouse.Move(m.tconn, start.CenterPoint(), 0)(ctx); err != nil {
		return errors.Wrap(err, "failed to move to the start location")
	}
	if err := mouse.Press(m.tconn, mouse.LeftButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to press the button")
	}

	// Move a little bit first to trigger launcher-app-paging.
	if err := mouse.Move(m.tconn, start.CenterPoint().Add(coords.Point{X: 10, Y: 10}), drapDropDuration)(ctx); err != nil {
		return errors.Wrap(err, "failed to move the mouse")
	}

	info, err := display.GetPrimaryInfo(ctx, m.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find the primary display info")
	}
	displayCenter := coords.NewPoint(info.Bounds.CenterX(), info.Bounds.CenterY())
	if err := mouse.Move(m.tconn, displayCenter, drapDropDuration)(ctx); err != nil {
		return errors.Wrap(err, "failed to move the mouse")
	}
	if err := mouse.Release(m.tconn, mouse.LeftButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to release the mouse")
	}

	// Verify the dropped app is in new position by checking its position before and after drag-and-drop.
	iconAfterDrag := launcher.AppItemViewFinder(appInfo.Name)
	locAfter, err := m.ui.Location(ctx, iconAfterDrag)
	if err != nil {
		return errors.Wrap(err, "failed to get location for icon after dragging")
	}
	if (locBefore.CenterX() == locAfter.CenterX()) && (locBefore.CenterY() == locAfter.CenterY()) {
		return errors.New("Dropped app is not at new position")
	}
	return nil
}

// dragToNextPage drags an icon to the next page of the app list with mouse.
func (m *dragAndDropByMouse) dragToNextPage(ctx context.Context, src *nodewith.Finder) error {
	start, err := m.ui.Location(ctx, src)
	if err != nil {
		return errors.Wrap(err, "failed to get location for src icon")
	}
	if err := mouse.Move(m.tconn, start.CenterPoint(), 0)(ctx); err != nil {
		return errors.Wrap(err, "failed to move to the start location")
	}
	if err := mouse.Press(m.tconn, mouse.LeftButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to press the icon")
	}

	// Move a little bit first to trigger launcher-app-paging.
	if err := mouse.Move(m.tconn, start.CenterPoint().Add(coords.Point{X: 1, Y: 1}), drapDropDuration)(ctx); err != nil {
		return errors.Wrap(err, "failed to move the mouse")
	}

	end, err := m.ui.Location(ctx, nodewith.ClassName("AppsGridView"))
	if err != nil {
		return errors.Wrap(err, "failed to get location for AppsGridView")
	}
	endPoint := coords.NewPoint(end.CenterPoint().X, end.Bottom())

	if err := mouse.Move(m.tconn, endPoint, drapDropDuration)(ctx); err != nil {
		return errors.Wrap(err, "failed to move the mouse")
	}

	// Move a little bit and wait for page change.
	pageSwitcher := nodewith.ClassName("PageSwitcher")
	if err := m.ui.WaitForEvent(pageSwitcher, event.Alert, mouse.Move(m.tconn, endPoint.Add(coords.Point{X: 1, Y: 0}), drapDropDuration))(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for page change event")
	}

	return mouse.Release(m.tconn, mouse.LeftButton)(ctx)
}

// openLauncher opens launcher for dragging by mouse.
func (m *dragAndDropByMouse) openLauncher(ctx context.Context) error {
	// Hit shift+search to turn the launcher into the fullscreen.
	return m.kw.AccelAction("Shift+Search")(ctx)
}

// verifySecondPage verifies that the second page exist or not.
func (m *dragAndDropByMouse) verifySecondPage(ctx context.Context) error {
	return verifySecondPageExist(ctx, m.tconn, m.ui, m.pc)
}

// verifySecondPageExist verifies that the second page exist or not.
func verifySecondPageExist(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, pc pointer.Context) error {
	pageSwitcher := nodewith.ClassName("PageSwitcher")
	pageButtons := nodewith.ClassName("Button").Ancestor(pageSwitcher)

	// The existing of page button means that we drag the app to next page sucessfully.
	if err := ui.WaitForEvent(pageSwitcher, event.Alert, pc.Click(pageButtons.First()))(ctx); err != nil {
		return errors.Wrap(err, "failed to click the page button")
	}
	return nil
}
