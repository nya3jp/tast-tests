// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package screenshot

import (
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

type Color struct{ R, G, B uint32 }

// Takes a screenshot and checks that dominant pixel color present exists with
// the passed in ratio and matches the passed in color by a predetermined
// threshold value. Returns true if the the parameters are met and false
// otherwise.
func Screenshot(s *testing.State, targetRatio float64, targetColor Color) bool {
	const screenshotName = "screenshot.png"
	ctx := s.Context()

	// verify takes a screenshot and checks if the specified color fills up more
	// than half of the screen.
	verify := func() bool {
		path := filepath.Join(s.OutDir(), screenshotName)
		cmd := testexec.CommandContext(ctx, "screenshot", "--internal", path)
		if err := cmd.Run(); err != nil {
			// We do not abort here because:
			// - screenshot command might have failed just because the internal display is not on yet
			// - Context deadline might be reached while taking a screenshot, which should be
			//   reported as "Screenshot does not contain expected pixels" rather than
			//   "screenshot command failed".
			cmd.DumpLog(ctx)
			return false
		}

		f, err := os.Open(path)
		if err != nil {
			s.Fatal("Failed opening the screenshot image: ", err)
		}
		defer f.Close()

		im, err := png.Decode(f)
		if err != nil {
			s.Fatal("Failed decoding the screenshot image: ", err)
		}

		getPopularColor := func(im image.Image) (color Color, ratio float64) {
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

		color, ratio := getPopularColor(im)

		s.Logf("Most popular color: #%02x%02x%02x (ratio=%v)", color.R/0x101, color.G/0x101, color.B/0x101, ratio)

		near := func(x uint32, y uint32) bool {
			// r is allowed color component difference in 16bit value.
			// Most differing color known to the date is #ba8b4a on sumo, so this value should be
			// no less than 0x1212.
			const r = 0x1300
			d := int32(x) - int32(y)
			return -r <= d && d <= r
		}
		isMatched := near(color.R, targetColor.R) && near(color.G, targetColor.G) && near(color.B, targetColor.B)
		return isMatched && ratio >= targetRatio
	}

	// Allow up to 10 seconds for the target screen to render.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for {
		if verify() {
			return true
		}
		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
			s.Error("Screenshot does not contain expected pixels. See: ", screenshotName)
			return false
		}
	}
}
