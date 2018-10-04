// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package colorcmp

import (
	"image"
	"image/color"
	"math"
	"reflect"
	"testing"
)

func TestColorStr(t *testing.T) {
	for _, tc := range []struct {
		clr color.Color
		exp string
	}{
		{&color.NRGBA{R: 0xff, G: 0x80, B: 0x40, A: 0xff}, "#ff8040"},
		{&color.NRGBA{R: 0xff, G: 0x80, B: 0x40, A: 0x80}, "#ff804080"},
		{&color.RGBA{R: 0xff, G: 0x80, B: 0x40, A: 0xff}, "#ff8040"},
	} {
		if s := ColorStr(tc.clr); s != tc.exp {
			t.Errorf("ColorStr(%+v) = %q; want %q", tc.clr, s, tc.exp)
		}
	}
}

func TestDominantColor(t *testing.T) {
	const (
		min      = -4 // inclusive
		max      = 4  // exclusive
		expRatio = 0.75
	)

	c1 := color.NRGBA{R: 0xff, G: 0x0, B: 0x0, A: 0xff}
	c2 := color.NRGBA{R: 0x0, G: 0xff, B: 0x0, A: 0xff}

	// Create an image with c2 in the bottom-right quadrant and c1 everywhere else.
	img := image.NewNRGBA(image.Rect(min, min, max, max))
	for x := min; x < max; x++ {
		for y := min; y < max; y++ {
			if x < 0 || y < 0 {
				img.SetNRGBA(x, y, c1)
			} else {
				img.SetNRGBA(x, y, c2)
			}
		}
	}

	dom, ratio := DominantColor(img)
	if domn := toNRGBA(dom); !reflect.DeepEqual(domn, c1) {
		t.Errorf("DominantColor() returned color %+v; want %+v", domn, c1)
	}
	if math.Abs(ratio-expRatio) > 0.01 {
		t.Errorf("DominantColor() returned ratio %0.2f; want %0.2f", ratio, expRatio)
	}
}

func TestColorsMatch(t *testing.T) {
	for _, tc := range []struct {
		a, b color.Color
		diff uint8
		exp  bool
	}{
		{RGB(0xff, 0x0, 0x0), RGB(0xff, 0x0, 0x0), 0x0, true},
		{RGB(0xff, 0x0, 0x0), RGB(0xfe, 0x0, 0x0), 0x0, false},
		{RGB(0xff, 0x0, 0x0), RGB(0xff, 0x0, 0x1), 0x0, false},
		{RGB(0xff, 0x0, 0x0), RGB(0xff, 0x0, 0x1), 0x1, true},
		{color.NRGBA{R: 0xff, G: 0x0, B: 0x0, A: 0x77}, color.NRGBA{R: 0xff, G: 0x0, B: 0x0, A: 0x77}, 0x0, true},
		{color.NRGBA{R: 0xff, G: 0x0, B: 0x0, A: 0x77}, color.NRGBA{R: 0xff, G: 0x0, B: 0x0, A: 0x76}, 0x0, false},
		{color.NRGBA{R: 0xff, G: 0x0, B: 0x0, A: 0x77}, color.NRGBA{R: 0xff, G: 0x0, B: 0x0, A: 0x76}, 0x1, true},
	} {
		if match := ColorsMatch(tc.a, tc.b, tc.diff); match != tc.exp {
			t.Errorf("ColorsMatch(%+v, %+v, 0x%02x) = %v; want %v", tc.a, tc.b, tc.diff, match, tc.exp)
		}
	}
}
