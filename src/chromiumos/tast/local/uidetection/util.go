// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import (
	"bytes"
	"context"
	"image"
	"image/draw"
	"image/png"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// readImage reads a PNG image and returns it in []byte.
func readImage(imgFile string) ([]byte, error) {
	// Read an image in PNG format.
	f, err := os.Open(imgFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open the image: %q", imgFile)
	}
	defer f.Close()

	imgPNG, _, err := image.Decode(f)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode the image")
	}

	imgBuf := new(bytes.Buffer)
	if err := png.Encode(imgBuf, imgPNG); err != nil {
		return nil, errors.Wrap(err, "failed to write the PNG image into byte buffer")
	}

	return imgBuf.Bytes(), nil
}

func crop(img image.Image, boundingBox coords.Rect) (image.Image, error) {
	bounds := img.Bounds().Intersect(image.Rect(boundingBox.Left, boundingBox.Top, boundingBox.Right(), boundingBox.Bottom()))
	cropped := image.NewRGBA(bounds)
	draw.Draw(cropped, bounds, img, bounds.Min, draw.Src)
	return cropped, nil
}

// takeScreenshot takes a screentshot in PNG format and reads it to []byte.
func takeScreenshot(ctx context.Context, tconn *chrome.TestConn, boundingBox coords.Rect) ([]byte, error) {
	uncropped, err := screenshot.CaptureChromeImageWithTestAPI(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to take the screenshot")
	}

	cropped, err := crop(uncropped, boundingBox)
	if err != nil {
		return nil, errors.Wrap(err, "unable to crop image")
	}

	imgBuf := new(bytes.Buffer)
	if err := png.Encode(imgBuf, cropped); err != nil {
		return nil, errors.Wrap(err, "failed to write the PNG image into byte buffer")
	}

	return imgBuf.Bytes(), nil
}

// takeStableScreenshot takes a stable screenshot that doesn't changed between two pollings.
func takeStableScreenshot(ctx context.Context, tconn *chrome.TestConn, pollOpts testing.PollOptions, boundingBox coords.Rect) ([]byte, error) {
	var lastScreen image.Image
	var currentScreen image.Image
	start := time.Now()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		uncropped, err := screenshot.CaptureChromeImageWithTestAPI(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to take immediate screenshot")
		}
		currentScreen, err = crop(uncropped, boundingBox)
		if err != nil {
			return errors.Wrap(err, "unable to crop image")
		}
		if !equal(currentScreen, lastScreen) {
			lastScreen = currentScreen
			elapsed := time.Since(start)
			return errors.Errorf("screen has not stopped changing after %s, perhaps increase timeout or use TakeScreenshot", elapsed)
		}
		return nil
	}, &pollOpts); err != nil {
		return nil, errors.Wrap(err, "failed to take stable screenshot")
	}

	// Convert image.Image to []byte.
	imgBuf := new(bytes.Buffer)
	if err := png.Encode(imgBuf, currentScreen); err != nil {
		return nil, errors.Wrap(err, "failed to write the PNG image into byte buffer")
	}
	return imgBuf.Bytes(), nil
}

func equal(imgA, imgB image.Image) bool {
	// Two images are considered equal if the colors at every pixel is the same.
	// Two nil images are also considered equal.
	if imgA == nil && imgB == nil {
		return true
	} else if imgA == nil || imgB == nil {
		return false
	}
	if imgA.Bounds() != imgB.Bounds() {
		return false
	}
	for y := imgA.Bounds().Min.Y; y < imgA.Bounds().Max.Y; y++ {
		for x := imgA.Bounds().Min.X; x < imgA.Bounds().Max.X; x++ {
			if imgA.At(x, y) != imgB.At(x, y) {
				return false
			}
		}
	}
	return true
}
