// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package screenshot supports taking and examining screenshots.
package screenshot

import (
	"fmt"
	"image"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Color contains a 48-bit RGB color (16 bpp).
type Color struct{ R, G, B uint32 }

// SaveScreenshot takes a screenshot and saves it as a PNG image to the
// specified file path.
func SaveScreenshot(s *testing.State, path string) error {
	cmd := testexec.CommandContext(s.Context(), "screenshot", "--internal", path)
	if err := cmd.Run(); err != nil {
		// We do not abort here because:
		// - screenshot command might have failed just because the internal display is not on yet
		// - Context deadline might be reached while taking a screenshot, which should be
		//   reported as "Screenshot does not contain expected pixels" rather than
		//   "screenshot command failed".
		cmd.DumpLog(s.Context())
		return fmt.Errorf("Failed executing screenshot command")
	}
	return nil
}

// DominantColor returns the color that occupies the largest number of pixels
// in the passed in image. It also returns the ratio of that pixel to the number
// of overall pixels in the image.
func DominantColor(s *testing.State, im image.Image) (color Color, ratio float64) {
	counter := map[Color]int{}
	box := im.Bounds()
	for x := box.Min.X; x < box.Max.X; x++ {
		for y := box.Min.Y; y < box.Max.Y; y++ {
			r, g, b, _ := im.At(x, y).RGBA()
			counter[Color{r, g, b}] += 1
		}
	}

	best := 0
	for c, cnt := range counter {
		if cnt > best {
			color = c
			best = cnt
		}
	}
	ratio = float64(best) / float64((box.Max.X-box.Min.X)*(box.Max.Y-box.Min.Y))
	return
}

// AreMatchingColors takes two color arguments and returns whether or not they
// should be considered the same for comparison purposes. This then provides a
// universal location for having the threshold we use for comparison.
func AreMatchingColors(a, b Color) bool {
	near := func(x uint32, y uint32) bool {
		// r is allowed color component difference in 16bit value.
		// Most differing color known to the date is #ba8b4a on sumo, so this value should be
		// no less than 0x1212.
		const r = 0x1300
		d := int32(x) - int32(y)
		return -r <= d && d <= r
	}
	return near(a.R, b.R) && near(a.G, b.G) && near(a.B, b.B)
}
