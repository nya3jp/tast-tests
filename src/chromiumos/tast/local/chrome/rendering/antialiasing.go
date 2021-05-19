// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package rendering supports controlling the rendering options provided when starting chrome.
package rendering

import (
	"os"
	"path/filepath"
)

// SubPixelAntialiasingMethod is an enum containing the method of subpixel antialiasing used by the system.
type SubPixelAntialiasingMethod string

// The following correspond to different types of subpixel antialiasing supported by CrOS.
const (
	NoSPAA   SubPixelAntialiasingMethod = "none"
	BGRSPAA  SubPixelAntialiasingMethod = "BGR"
	RGBSPAA  SubPixelAntialiasingMethod = "RGB"
	VBGRSPAA SubPixelAntialiasingMethod = "VGBR"
	VRGBSPAA SubPixelAntialiasingMethod = "VRGB"
)

// FontConfigDir contains the directory where fonts are installed.
const FontConfigDir = "/etc/fonts/conf.d"

// SPAAFiles contains the file that each subpixel antialiasing option
// corresponds to. Whichever one is stored in the FontConfigDir will be the
// method that is used by the system the next time chrome restarts.
var SPAAFiles = map[SubPixelAntialiasingMethod]string{
	NoSPAA:   "10-no-sub-pixel.conf",
	BGRSPAA:  "10-sub-pixel-bgr.conf",
	RGBSPAA:  "10-sub-pixel-rgb.conf",
	VBGRSPAA: "10-sub-pixel-vbgr.conf",
	VRGBSPAA: "10-sub-pixel-vrgb.conf"}

// CurrentSubPixelAntialiasingMethod returns the current subpixel antialiasing
// method, if it was able to be determined, or "" otherwise.
func CurrentSubPixelAntialiasingMethod() SubPixelAntialiasingMethod {
	for method, fname := range SPAAFiles {
		if _, err := os.Stat(filepath.Join(FontConfigDir, fname)); !os.IsNotExist(err) {
			return method
		}
	}
	return ""
}
