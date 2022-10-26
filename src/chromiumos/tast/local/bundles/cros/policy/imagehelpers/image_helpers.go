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

// GetImgBytesFromFilePath returns bytes of the image with the filePath.
func GetImgBytesFromFilePath(filePath string) ([]byte, error) {
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
	imgBytes := buf.Bytes()
	return imgBytes, nil
}
