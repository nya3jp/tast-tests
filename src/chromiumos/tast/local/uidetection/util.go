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
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	screenshotFile    = "uidetection_screenshot.png"
	oldScreenshotFile = "old_uidetection_screenshot.png"
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
	if cropped.Bounds().Empty() {
		return nil, errors.New(ErrEmptyBoundingBox)
	}
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
		return nil, err
	}

	imgBuf := new(bytes.Buffer)
	if err := png.Encode(imgBuf, cropped); err != nil {
		return nil, errors.Wrap(err, "failed to write the PNG image into byte buffer")
	}

	return imgBuf.Bytes(), nil
}

// takeStableScreenshot takes a stable screenshot that doesn't changed between two pollings.
func takeStableScreenshot(ctx context.Context, tconn *chrome.TestConn, pollOpts testing.PollOptions, boundingBox coords.Rect) ([]byte, error) {
	var currentScreen image.Image
	var lastScreen image.Image
	start := time.Now()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		lastScreen = currentScreen
		uncropped, err := screenshot.CaptureChromeImageWithTestAPI(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to take immediate screenshot")
		}
		currentScreen, err = crop(uncropped, boundingBox)
		if err != nil {
			return err
		}
		if err = equal(currentScreen, lastScreen); err != nil {
			return errors.Wrapf(err, "screen has not stopped changing after %s, perhaps increase timeout or use immediate-screenshot strategy", time.Since(start))
		}
		return nil
	}, &pollOpts); err != nil {
		// Save two screenshots to output dir in case of error.
		if err := saveImageToOutput(ctx, currentScreen, screenshotFile); err != nil {
			testing.ContextLogf(ctx, "INFO: couldn't save the screenshot to %s for debugging takeStableScreenshot: %s", screenshotFile, err)
		}
		if err := saveImageToOutput(ctx, lastScreen, oldScreenshotFile); err != nil {
			testing.ContextLogf(ctx, "INFO: couldn't save the old screenshot to %s for debugging takeStableScreenshot: %s", oldScreenshotFile, err)
		}
		return nil, errors.Wrap(err, "failed to take stable screenshot")
	}

	// Convert image.Image to []byte.
	imgBuf := new(bytes.Buffer)
	if err := png.Encode(imgBuf, currentScreen); err != nil {
		return nil, errors.Wrap(err, "failed to write the PNG image into byte buffer")
	}
	return imgBuf.Bytes(), nil
}

// equal returns error if two images are not the same.
// The error message contains information about how they differ.
func equal(imgA, imgB image.Image) error {
	// Two images are considered equal if the colors at every pixel is the same.
	// Two nil images are also considered equal.
	if imgA == nil && imgB == nil {
		return nil
	} else if imgA == nil || imgB == nil {
		return errors.New("one image is nil while the other is not")
	}
	if imgA.Bounds() != imgB.Bounds() {
		return errors.New("two images are in different sizes")
	}
	for y := imgA.Bounds().Min.Y; y < imgA.Bounds().Max.Y; y++ {
		for x := imgA.Bounds().Min.X; x < imgA.Bounds().Max.X; x++ {
			if imgA.At(x, y) != imgB.At(x, y) {
				return errors.Errorf("Screen has changed since the last screenshot. Images %s and %s differ at (%d, %d)", oldScreenshotFile, screenshotFile, x, y)
			}
		}
	}
	return nil
}

// saveImageToOutput saves an image in image.Image format to the testing output
// dir that will be uploaded to the test log folder.
func saveImageToOutput(ctx context.Context, img image.Image, filename string) error {
	outputDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("failed to get the output dir")
	}
	if err := saveImage(img, filepath.Join(outputDir, filename)); err != nil {
		return errors.Wrapf(err, "failed to save file %s", filename)
	}
	return nil
}

// saveBytesImageToOutput saves an image in []byte format to the testing output
// dir that will be uploaded to the test log folder.
func saveBytesImageToOutput(ctx context.Context, img []byte, filename string) error {
	outputDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("failed to get the output dir")
	}
	if err := ioutil.WriteFile((filepath.Join(outputDir, filename)), img, 0644); err != nil {
		return errors.Wrapf(err, "failed to save file %s", filename)
	}
	return nil
}

// saveImage saves an image in image.Image format to a specified path.
func saveImage(img image.Image, path string) error {
	if img == nil {
		return errors.New("image is nil")
	}

	f, err := os.Create(path)
	if err != nil {
		return errors.Wrap(err, "failed to create the PNG file")
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return errors.Wrap(err, "failed to write the PNG image into file")
	}
	return nil
}
