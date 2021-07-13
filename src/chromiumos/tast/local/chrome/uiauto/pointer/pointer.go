// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pointer provides the access to the pointing device of the device,
// either of the mouse or the touch.
package pointer

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// Context provides the interface to control a pointing device.
type Context interface {
	// Close cleans up its internal resource.
	Close() error

	// Click returns a function to cause a click or a tap on the node.
	Click(finder *nodewith.Finder) uiauto.Action

	// ClickAt returns a function to cause a click or a tap on the specified
	// location.
	ClickAt(p coords.Point) uiauto.Action

	// MenuClick returns a function to cause a right-click or a long-press on the
	// node to cause the secondary behavior (i.e. opening context menu).
	MenuClick(finder *nodewith.Finder) uiauto.Action

	// Drag returns a function which initiates a dragging session, and conducts
	// the specified gestures. It ensures that the dragging session ends properly
	// at end.
	// Example:
	//  // Start from p0, move to p1, and then move to p2.
	//  pc.Drag(p0, pc.DragTo(p1, time.Second), pc.DragTo(p2, time.Second))
	Drag(initLoc coords.Point, gestures ...uiauto.Action) uiauto.Action

	// DragTo returns a function to cause a drag to the specified location.
	DragTo(p coords.Point, duration time.Duration) uiauto.Action

	// DragToNode returns a function to cause a drag to the specified node.
	DragToNode(f *nodewith.Finder, duration time.Duration) uiauto.Action

	Hold(duration time.Duration) uiauto.Action
}

// MouseContext is a Context with the mouse.
type MouseContext struct {
	ac    *uiauto.Context
	tconn *chrome.TestConn
}

// NewMouse creates a new instance of MouseContext.
func NewMouse(tconn *chrome.TestConn) *MouseContext {
	return &MouseContext{ac: uiauto.New(tconn), tconn: tconn}
}

// Close implements Context.Close.
func (mc *MouseContext) Close() error {
	return nil
}

// Click implements Context.Click.
func (mc *MouseContext) Click(finder *nodewith.Finder) uiauto.Action {
	return mc.ac.LeftClick(finder)
}

// ClickAt implements Context.ClickAt.
func (mc *MouseContext) ClickAt(loc coords.Point) uiauto.Action {
	return mouse.Click(mc.tconn, loc, mouse.LeftButton)
}

// MenuClick implements Context.MenuClick.
func (mc *MouseContext) MenuClick(finder *nodewith.Finder) uiauto.Action {
	return mc.ac.RightClick(finder)
}

func (mc *MouseContext) Hold(duration time.Duration) uiauto.Action {
	return nil
}

// Drag implements Context.Drag.
func (mc *MouseContext) Drag(loc coords.Point, gestures ...uiauto.Action) uiauto.Action {
	gestureAction := uiauto.Combine("drag gresture", gestures...)
	return func(ctx context.Context) (err error) {
		pressed := false
		defer func() {
			if !pressed {
				return
			}
			releaseErr := mouse.Release(mc.tconn, mouse.LeftButton)(ctx)
			if releaseErr != nil {
				testing.ContextLog(ctx, "Failed to release the mouse button: ", releaseErr)
				if err == nil {
					err = releaseErr
				}
			}
		}()
		sctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
		defer cancel()
		if err := uiauto.Combine(
			"start drag",
			mouse.Move(mc.tconn, loc, 0),
			mouse.Press(mc.tconn, mouse.LeftButton),
		)(sctx); err != nil {
			return errors.Wrap(err, "failed to start dragging")
		}
		pressed = true
		return gestureAction(sctx)
	}
}

// DragTo implements Context.DragTo.
func (mc *MouseContext) DragTo(p coords.Point, duration time.Duration) uiauto.Action {
	return mouse.Move(mc.tconn, p, duration)
}

// DragToNode implements Context.DragToNode.
func (mc *MouseContext) DragToNode(f *nodewith.Finder, duration time.Duration) uiauto.Action {
	return func(ctx context.Context) error {
		loc, err := mc.ac.Location(ctx, f)
		if err != nil {
			return err
		}
		return mouse.Move(mc.tconn, loc.CenterPoint(), duration)(ctx)
	}
}

// TouchContext is a Context with the touchscreen.
type TouchContext struct {
	tc *touch.Context
}

// NewTouch creates a new TouchContext instance.
func NewTouch(ctx context.Context, tconn *chrome.TestConn) (*TouchContext, error) {
	tc, err := touch.New(ctx, tconn)
	if err != nil {
		return nil, err
	}
	return &TouchContext{tc: tc}, nil
}

// Close implements Context.Close.
func (tc *TouchContext) Close() error {
	return tc.tc.Close()
}

// Click implements Context.Click.
func (tc *TouchContext) Click(finder *nodewith.Finder) uiauto.Action {
	return tc.tc.Tap(finder)
}

// ClickAt implements Context.ClickAt.
func (tc *TouchContext) ClickAt(loc coords.Point) uiauto.Action {
	return tc.tc.TapAt(loc)
}

// MenuClick implements Context.MenuClick.
func (tc *TouchContext) MenuClick(finder *nodewith.Finder) uiauto.Action {
	return tc.tc.LongPress(finder)
}

// Drag implements Context.Drag.
func (tc *TouchContext) Drag(loc coords.Point, gestures ...uiauto.Action) uiauto.Action {
	return tc.tc.Swipe(loc, gestures...)
}

// DragTo implements Context.DragTo.
func (tc *TouchContext) DragTo(loc coords.Point, duration time.Duration) uiauto.Action {
	return tc.tc.SwipeTo(loc, duration)
}

// DragToNode implements COntext.DragToNode.
func (tc *TouchContext) DragToNode(f *nodewith.Finder, duration time.Duration) uiauto.Action {
	return tc.tc.SwipeToNode(f, duration)
}

func (tc *TouchContext) Hold(duration time.Duration) uiauto.Action {
	return tc.tc.Hold(duration)
}
