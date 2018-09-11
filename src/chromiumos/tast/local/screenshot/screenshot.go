// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package screenshot supports taking and examining screenshots.
package screenshot

import (
	"context"
	"fmt"
	"image"
	"strings"

	"chromiumos/tast/local/testexec"
)

// Color contains a 48-bit RGB color (16 bits per channel).
type Color struct{ R, G, B uint16 }

func (c Color) String() string {
	return fmt.Sprintf("#%02x%02x%02x", c.R/0x101, c.G/0x101, c.B/0x101)
}

// Capture takes a screenshot and saves it as a PNG image to the specified file
// path.
func Capture(ctx context.Context, path string) error {
	cmd := testexec.CommandContext(ctx, "screenshot", "--internal", path)
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return fmt.Errorf("failed running %q", strings.Join(cmd.Args, " "))
	}
	return nil
}

// DominantColor returns the color that occupies the largest number of pixels
// in the passed in image. It also returns the ratio of that pixel to the number
// of overall pixels in the image.
func DominantColor(im image.Image) (color Color, ratio float64) {
	counter := map[Color]int{}
	box := im.Bounds()
	for x := box.Min.X; x < box.Max.X; x++ {
		for y := box.Min.Y; y < box.Max.Y; y++ {
			r, g, b, _ := im.At(x, y).RGBA()
			counter[Color{uint16(r), uint16(g), uint16(b)}] += 1
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

// MaxKnownColorDiff is the largest differing color known to date and is #ba8b4a
// on sumo in comparison to a target of #cc8844, so this value should be no less
// than 0x1212.
const MaxKnownColorDiff = 0x1300

// ColorsMatch takes two color arguments and returns whether or not each color
// component is within maxDiff of each other. Color components are 16-bit
// values.
func ColorsMatch(a, b Color, maxDiff uint16) bool {
	allowed := int32(maxDiff)
	near := func(x uint16, y uint16) bool {
		d := int32(x) - int32(y)
		return -allowed <= d && d <= allowed
	}
	return near(a.R, b.R) && near(a.G, b.G) && near(a.B, b.B)
}
