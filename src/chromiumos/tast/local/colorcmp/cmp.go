// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package colorcmp supports comparing colors.
package colorcmp

import (
	"fmt"
	"image"
	"image/color"
)

// RGB returns a fully-opaque color.Color based on the supplied components.
func RGB(r, g, b uint8) color.Color {
	return color.NRGBA{R: r, G: g, B: b, A: 0xff}
}

// ColorStr converts clr to 8-bit-per-channel, non-pre-alpha-multiplied RGBA and
// returns either a "#rrggbb" representation if it's fully opaque or "#rrggbbaa" otherwise.
func ColorStr(clr color.Color) string {
	nrgba := toNRGBA(clr)
	if nrgba.A == 0xff {
		return fmt.Sprintf("#%02x%02x%02x", nrgba.R, nrgba.G, nrgba.B)
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", nrgba.R, nrgba.G, nrgba.B, nrgba.A)
}

// toNRGBA converts clr to a color.NRGBA.
func toNRGBA(clr color.Color) color.NRGBA {
	return color.NRGBAModel.Convert(clr).(color.NRGBA)
}

// DominantColor returns the color that occupies the largest number of pixels
// in the passed in image. It also returns the ratio of that pixel to the number
// of overall pixels in the image.
func DominantColor(im image.Image) (clr color.Color, ratio float64) {
	counts := map[color.NRGBA]int{}
	box := im.Bounds()
	for x := box.Min.X; x < box.Max.X; x++ {
		for y := box.Min.Y; y < box.Max.Y; y++ {
			counts[toNRGBA(im.At(x, y))]++
		}
	}

	var bestClr color.NRGBA
	bestCnt := 0
	for clr, cnt := range counts {
		if cnt > bestCnt {
			bestClr = clr
			bestCnt = cnt
		}
	}
	return bestClr, float64(bestCnt) / float64(box.Dx()*box.Dy())
}

// ColorsMatch takes two colors and returns whether or not each component is within
// maxDiff of each other after conversion to 8-bit-per-channel, non-alpha-premultiplied RGBA.
func ColorsMatch(a, b color.Color, maxDiff uint8) bool {
	an := toNRGBA(a)
	bn := toNRGBA(b)

	allowed := int(maxDiff)
	near := func(x uint8, y uint8) bool {
		d := int(x) - int(y)
		return -allowed <= d && d <= allowed
	}
	return near(an.R, bn.R) && near(an.G, bn.G) && near(an.B, bn.B) && near(an.A, bn.A)
}

// ColorsBrightnessCmp takes two colors and returns whether color b brighter than color a
// in gray scale comparison.
func ColorsBrightnessCmp(a, b color.Color) bool {
	ag := color.GrayModel.Convert(a).(color.Gray).Y
	bg := color.GrayModel.Convert(b).(color.Gray).Y
	return ag < bg
}
