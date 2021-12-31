// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// ReadImage reads a PNG image and returns it in []byte.
func ReadImage(imgFile string) ([]byte, error) {
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

// TakeScreenshot takes a screentshot in PNG format and reads it to []byte.
func TakeScreenshot(ctx context.Context, tconn *chrome.TestConn) ([]byte, error) {
	imgPNG, err := screenshot.CaptureChromeImageWithTestAPI(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to take the screenshot")
	}

	imgBuf := new(bytes.Buffer)
	if err := png.Encode(imgBuf, imgPNG); err != nil {
		return nil, errors.Wrap(err, "failed to write the PNG image into byte buffer")
	}

	return imgBuf.Bytes(), nil
}

// TakeStableScreenshot takes a stable screenshot that doesn't changed between two pollings.
func TakeStableScreenshot(ctx context.Context, tconn *chrome.TestConn, pollOpts testing.PollOptions) ([]byte, error) {
	var lastScreen []byte
	var currentScreen []byte
	start := time.Now()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		currentScreen, err = TakeScreenshot(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to take immediate screenshot")
		}
		if bytes.Compare(currentScreen, lastScreen) != 0 {
			lastScreen = currentScreen
			elapsed := time.Since(start)
			return errors.Errorf("screen has not stopped changing after %s, perhaps increase timeout or use TakeScreenshot", elapsed)
		}
		return nil
	}, &pollOpts); err != nil {
		return nil, errors.Wrap(err, "failed to take stable screenshot")
	}
	return currentScreen, nil
}
