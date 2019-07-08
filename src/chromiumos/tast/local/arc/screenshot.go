// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"image"
	"image/color"
	"os"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/screenshot"
)

// CountBlackPixels returns how many black pixels are contained in image.
func CountBlackPixels(image image.Image) int {
	// TODO(ricardoq): At least on Eve, Nocturne, Caroline, Kevin and Dru the color
	// that we are looking for is RGBA(0,0,0,255). But it might be possible that
	// on certain devices the color is slightly different. In that case we should
	// adjust the colorMaxDiff.
	const colorMaxDiff = 0
	black := color.RGBA{0, 0, 0, 255}
	rect := image.Bounds()
	blackPixels := 0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if colorcmp.ColorsMatch(image.At(x, y), black, colorMaxDiff) {
				blackPixels++
			}
		}
	}
	return blackPixels
}

// GrabScreenshot creates a screenshot in path, and returns an image.Image.
func GrabScreenshot(ctx context.Context, cr *chrome.Chrome, path string) (image.Image, error) {
	if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
		return nil, errors.Wrap(err, "failed to capture screenshot")
	}

	fd, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "error opening screenshot file")
	}
	defer fd.Close()

	img, _, err := image.Decode(fd)
	if err != nil {
		return nil, errors.Wrap(err, "error decoding image file")
	}
	return img, nil
}
