// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package screenshot provides functions to assist with taking screenshots and analyzing them
// in ARC window management tests.
package screenshot

import (
	"context"
	"image"
	"image/color"
	"io/ioutil"
	"os"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/screenshot"
)

// CountPixels returns how many pixels in the specified color are contained in image.
func CountPixels(image image.Image, clr color.Color) int {
	// TODO(ricardoq): At least on Eve, Nocturne, Caroline, Kevin and Dru the color
	// in black flashes is RGBA(0,0,0,255). But it might be possible that on certain
	// devices the color is slightly different. In that case we should adjust the colorMaxDiff.
	const colorMaxDiff = 0
	rect := image.Bounds()
	numPixels := 0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if colorcmp.ColorsMatch(image.At(x, y), clr, colorMaxDiff) {
				numPixels++
			}
		}
	}
	return numPixels
}

// CountBrighterPixels takes two same size image, return how many pixels in
// countImage brighter than baseImage.
func CountBrighterPixels(baseImage, countImage image.Image) (int, error) {
	rect := baseImage.Bounds()
	if rect != countImage.Bounds() {
		return 0, errors.New("two images have different size")
	}
	numPixels := 0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if colorcmp.Brighter(countImage.At(x, y), baseImage.At(x, y)) {
				numPixels++
			}
		}
	}
	return numPixels, nil
}

// CountDiffPixels takes two same size image, return how many pixels are different from another image.
func CountDiffPixels(baseImage, countImage image.Image, maxDiff uint8) (int, error) {
	rect := baseImage.Bounds()
	if rect != countImage.Bounds() {
		return 0, errors.New("two images have different size")
	}
	numPixels := 0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if !colorcmp.ColorsMatch(countImage.At(x, y), baseImage.At(x, y), maxDiff) {
				numPixels++
			}
		}
	}
	return numPixels, nil
}

// GrabScreenshot creates a screenshot and returns an image.Image.
// The path of the image is generated ramdomly in /tmp.
func GrabScreenshot(ctx context.Context, cr *chrome.Chrome) (image.Image, error) {
	fd, err := ioutil.TempFile("", "screenshot")
	if err != nil {
		return nil, errors.Wrap(err, "error opening screenshot file")
	}
	defer os.Remove(fd.Name())
	defer fd.Close()

	if err := screenshot.CaptureChrome(ctx, cr, fd.Name()); err != nil {
		return nil, errors.Wrap(err, "failed to capture screenshot")
	}

	img, _, err := image.Decode(fd)
	if err != nil {
		return nil, errors.Wrap(err, "error decoding image file")
	}
	return img, nil
}
