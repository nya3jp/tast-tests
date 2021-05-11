// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cca

import (
	"context"
	"fmt"
	"image"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// Frame is the frame grabbed from preview.
type Frame struct {
	jsFrame *chrome.JSObject
}

func getIntFieldFromJsObject(ctx context.Context, obj *chrome.JSObject, field string) (int, error) {
	code := fmt.Sprintf("function() { return this.%s; }", field)
	var value int
	if err := obj.Call(ctx, &value, code); err != nil {
		return -1, errors.Wrapf(err, "failed to get int field %v", field)
	}
	return value, nil
}

// Find executes |code| for finding pixel location in the frame. The "this" in
// code is type of |CanvasRenderingContext2D| referring to the frame.
func (f *Frame) Find(ctx context.Context, code string) (*image.Point, error) {
	var xy chrome.JSObject
	if err := f.jsFrame.Call(ctx, &xy, code); err != nil {
		return nil, errors.Wrapf(err, "failed to execute point finding code %v", code)
	}
	defer xy.Release(ctx)

	x, err := getIntFieldFromJsObject(ctx, &xy, "x")
	if err != nil {
		return nil, err
	}

	y, err := getIntFieldFromJsObject(ctx, &xy, "y")
	if err != nil {
		return nil, err
	}

	return &image.Point{x, y}, nil
}

// Release releases the JSObject within the frame.
func (f *Frame) Release(ctx context.Context) error {
	return f.jsFrame.Release(ctx)
}
