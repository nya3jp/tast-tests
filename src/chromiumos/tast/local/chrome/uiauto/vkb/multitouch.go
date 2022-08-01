// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vkb

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// MultitouchContext represents a context for multitouch typing.
type MultitouchContext struct {
	VirtualKeyboardContext
	tsw  *input.TouchscreenEventWriter
	stws []*input.SingleTouchEventWriter
	tcc  *input.TouchCoordConverter
}

// NewMultitouchContext creates a new context for multitouch.
func (vkbCtx *VirtualKeyboardContext) NewMultitouchContext(ctx context.Context, numTouches int) (*MultitouchContext, error) {
	tsw, tcc, err := touch.NewTouchscreenAndConverter(ctx, vkbCtx.tconn)
	if err != nil {
		return nil, err
	}

	stws := make([]*input.SingleTouchEventWriter, numTouches)
	for i := 0; i < numTouches; i++ {
		stw, err := tsw.NewSingleTouchWriter()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get touch writer")
		}
		stws[i] = stw
	}

	mtCtx := &MultitouchContext{
		VirtualKeyboardContext: *vkbCtx,
		tsw:                    tsw,
		stws:                   stws,
		tcc:                    tcc,
	}

	return mtCtx, nil
}

// Close closes access to the multitouch writers.
func (mtCtx *MultitouchContext) Close() {
	for _, stw := range mtCtx.stws {
		stw.Close()
	}
	mtCtx.tsw.Close()
}

func (mtCtx *MultitouchContext) holdAt(ctx context.Context, loc coords.Point, touchIndex int) error {
	stw := mtCtx.stws[touchIndex]

	x, y := mtCtx.tcc.ConvertLocation(loc)
	if err := stw.Move(x, y); err != nil {
		return errors.Wrap(err, "failed to move the single touch")
	}

	testing.Sleep(ctx, 2000*time.Millisecond)

	return nil
}

// Hold returns a function that holds the node through the touchscreen.
func (mtCtx *MultitouchContext) Hold(finder *nodewith.Finder, touchIndex int) uiauto.Action {
	return func(ctx context.Context) error {
		ui := uiauto.New(mtCtx.tconn)
		loc, err := ui.Location(ctx, finder)
		if err != nil {
			return errors.Wrap(err, "failed to get the location of the node")
		}
		return mtCtx.holdAt(ctx, loc.CenterPoint(), touchIndex)
	}
}

// Release returns a function that releases the node through the touchscreen.
func (mtCtx *MultitouchContext) Release(touchIndex int) uiauto.Action {
	return func(ctx context.Context) error {
		stw := mtCtx.stws[touchIndex]
		stw.End()
		return nil
	}
}
