// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import (
	"bytes"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/screenshot"
	"context"
	"image"
	"image/png"
	"os"
)

// ReadImage reads a PNG image and returns it in []byte.
func ReadImage(imgFile string) ([]byte, error) {
	// Read an image in PNG format.
	f, err := os.Open(imgFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	imgPNG, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}

	imgBuf := new(bytes.Buffer)
	if err := png.Encode(imgBuf, imgPNG); err != nil {
		return nil, err
	}

	return imgBuf.Bytes(), nil
}

// TakeScreenshot takes a screentshot in PNG format and reads it to []byte.
func TakeScreenshot(ctx context.Context) ([]byte, error) {
	tmpFile := screenshotSaveDir
	if err := screenshot.Capture(ctx, tmpFile); err != nil {
		return nil, errors.Wrap(err, "failed to take screenshot")
	}
	defer os.Remove(tmpFile)

	return ReadImage(tmpFile)
}

// Contains checks if a string slice contains a specific string.
func Contains(strs []string, s string) bool {
	for _, str := range strs {
		if str == s {
			return true
		}
	}
	return false
}

// Equals checks if two strings slices contains exactly the same strings.
func Equals(strsA []string, strsB []string) bool {
	if len(strsA) != len(strsB) {
		return false
	}
	for _, str := range strsB {
		if !Contains(strsA, str) {
			return false
		}
	}
	return true
}
