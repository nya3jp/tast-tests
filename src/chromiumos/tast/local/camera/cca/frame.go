// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cca

import (
	"context"
	"image"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// Frame is the frame grabbed from preview.
type Frame struct {
	jsFrame *chrome.JSObject
}

type point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// PixelTarget specifies the target pixel to be found in frame.
type PixelTarget struct {
	code string
}

var (
	// FirstBlack is the first black pixel met when scans frame pixels from first to last row.
	FirstBlack = PixelTarget{`function() {
		const {width, height} = this.canvas;
		const {data} = this.getImageData(0, 0, width, height);
		let idx = 0;
		for (let y = 0 ; y < height ; y ++) {
			for (let x = 0 ; x < width ; x ++) {
				if (data[idx] === 0) {
					return {x, y};
				}
				idx += 4;
			}
		}
		throw new Error('Cannot find point');
	}`}
	// LastBlack is the last black pixel met when scans frame pixels from first to last row.
	LastBlack = PixelTarget{`function() {
		const {width, height} = this.canvas;
		const {data} = this.getImageData(0, 0, width, height);
		let idx = data.length - 4;
		for (let y = height-1 ; y >= 0 ; y --) {
			for (let x = width-1 ; x >= 0 ; x --) {
				if (data[idx] === 0) {
					return {x, y};
				}
				idx -= 4;
			}
		}
		throw new Error('Cannot find point');
	}`}
)

func (p *point) Point() *image.Point {
	return &image.Point{p.X, p.Y}
}

// Find finds pixel location in the frame.
func (f *Frame) Find(ctx context.Context, t *PixelTarget) (*image.Point, error) {
	var p point
	if err := f.jsFrame.Call(ctx, &p, t.code); err != nil {
		return nil, errors.Wrapf(err, "failed to execute point finding code %v", t.code)
	}

	return p.Point(), nil
}

// Release releases the JSObject within the frame.
func (f *Frame) Release(ctx context.Context) error {
	return f.jsFrame.Release(ctx)
}
