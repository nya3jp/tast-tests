// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package imagehelpers provides helper functions related to image processing.
package imagehelpers

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
)

// GetJPEGBytesFromFilePath returns bytes in the JPEG format of the image with the filePath.
func GetJPEGBytesFromFilePath(filePath string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	image, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, image, nil)
	if err != nil {
		return nil, err
	}
	jpegBytes := buf.Bytes()
	return jpegBytes, nil
}
