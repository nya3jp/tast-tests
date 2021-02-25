// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package imgcmp is a package for common code related to image comparisons.
package imgcmp

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/colorcmp"
)

// CountPixelsWithDiff returns how many pixels in the specified color are contained in image with max diff.
func CountPixelsWithDiff(image image.Image, clr color.Color, colorMaxDiff uint8) int {
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

// CountPixels returns how many pixels in the specified color are contained in image.
func CountPixels(image image.Image, clr color.Color) int {
	// TODO(ricardoq): At least on Eve, Nocturne, Caroline, Kevin and Dru the color
	// in black flashes is RGBA(0,0,0,255). But it might be possible that on certain
	// devices the color is slightly different. In that case we should adjust the colorMaxDiff.
	return CountPixelsWithDiff(image, clr, 0)
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

// CountDiffPixels takes two images of the same size, and returns how many pixels are different.
func CountDiffPixels(imageA, imageB image.Image, threshold uint8) (int, error) {
	rect := imageA.Bounds()
	if rect != imageB.Bounds() {
		return 0, errors.Errorf("the images have different sizes; imageA=%v, imageB=%v", imageA.Bounds(), imageB.Bounds())
	}
	numPixels := 0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if !colorcmp.ColorsMatch(imageB.At(x, y), imageA.At(x, y), threshold) {
				numPixels++
			}
		}
	}
	return numPixels, nil
}

// DumpImageToPNG saves the image to path.
func DumpImageToPNG(ctx context.Context, image *image.Image, path string) error {
	fd, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fd.Close()
	return png.Encode(fd, *image)
}
