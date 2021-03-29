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
	"chromiumos/tast/local/chrome/ash"
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
	Drag(initLoc coords.Point, gestures ...uiauto.Action) uiauto.Action

	// DragTo returns a function to cause a drag to the specified location.
	DragTo(p coords.Point, duration time.Duration) uiauto.Action

	// DragToNode returns a function to cause a drag to the specified node.
	DragToNode(f *nodewith.Finder, duration time.Duration) uiauto.Action
}

// New checks the current tablet-mode state and returns the Context on that
// mode.
func New(ctx context.Context, tconn *chrome.TestConn) (Context, error) {
	isTablet, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		return nil, err
	}
	// Use touch device if it's in the tablet mode.
	return NewWithTouch(ctx, tconn, isTablet)
}

// NewWithTouch creates the Context instance for either of touch or mouse.
func NewWithTouch(ctx context.Context, tconn *chrome.TestConn, useTouch bool) (Context, error) {
	if useTouch {
		tc, err := touch.New(ctx, tconn)
		if err != nil {
			return nil, err
		}
		return &touchContext{tc: tc}, nil
	}
	return &mouseContext{ac: uiauto.New(tconn), tconn: tconn}, nil
}

type mouseContext struct {
	ac    *uiauto.Context
	tconn *chrome.TestConn
}

func (mc *mouseContext) Close() error {
	return nil
}

func (mc *mouseContext) Click(finder *nodewith.Finder) uiauto.Action {
	return mc.ac.LeftClick(finder)
}

func (mc *mouseContext) ClickAt(loc coords.Point) uiauto.Action {
	return mouse.Click(mc.tconn, loc, mouse.LeftButton)
}

func (mc *mouseContext) MenuClick(finder *nodewith.Finder) uiauto.Action {
	return mc.ac.RightClick(finder)
}

func (mc *mouseContext) Drag(loc coords.Point, gestures ...uiauto.Action) uiauto.Action {
	gestureAction := uiauto.Combine("drag gresture", gestures...)
	return func(ctx context.Context) (err error) {
		pressed := false
		defer func() {
			if !pressed {
				return
			}
			testing.ContextLog(ctx, "releasing: ", ctx.Err())
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
		testing.ContextLog(ctx, "started")
		return gestureAction(sctx)
	}
}

func (mc *mouseContext) DragTo(p coords.Point, duration time.Duration) uiauto.Action {
	return mouse.Move(mc.tconn, p, duration)
}

func (mc *mouseContext) DragToNode(f *nodewith.Finder, duration time.Duration) uiauto.Action {
	return func(ctx context.Context) error {
		loc, err := mc.ac.Location(ctx, f)
		if err != nil {
			return err
		}
		return mouse.Move(mc.tconn, loc.CenterPoint(), duration)(ctx)
	}
}

type touchContext struct {
	tc *touch.Context
}

func (tc *touchContext) Close() error {
	return tc.tc.Close()
}

func (tc *touchContext) Click(finder *nodewith.Finder) uiauto.Action {
	return tc.tc.Tap(finder)
}

func (tc *touchContext) ClickAt(loc coords.Point) uiauto.Action {
	return tc.tc.TapAt(loc)
}

func (tc *touchContext) MenuClick(finder *nodewith.Finder) uiauto.Action {
	return tc.tc.LongPress(finder)
}

func (tc *touchContext) Drag(loc coords.Point, gestures ...uiauto.Action) uiauto.Action {
	return tc.tc.Swipe(loc, gestures...)
}

func (tc *touchContext) DragTo(loc coords.Point, duration time.Duration) uiauto.Action {
	return tc.tc.SwipeTo(loc, duration)
}

func (tc *touchContext) DragToNode(f *nodewith.Finder, duration time.Duration) uiauto.Action {
	return tc.tc.SwipeToNode(f, duration)
}
