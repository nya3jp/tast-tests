// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package screenshot supports taking and examining screenshots.
package screenshot

import (
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"io"
	"os"
	"strings"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
)

// Color contains a 48-bit RGB color (16 bits per channel).
type Color struct{ R, G, B uint16 }

// RGB returns a Color representing the requested RGB color.
func RGB(r, g, b uint16) Color { return Color{r, g, b} }

func (c Color) String() string {
	return fmt.Sprintf("#%02x%02x%02x", c.R/0x101, c.G/0x101, c.B/0x101)
}

// Capture takes a screenshot and saves it as a PNG image to the specified file
// path. It will use the CLI screenshot command to perform the screen capture.
func Capture(ctx context.Context, path string) error {
	cmd := testexec.CommandContext(ctx, "screenshot", "--internal", path)
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return fmt.Errorf("failed running %q", strings.Join(cmd.Args, " "))
	}
	return nil
}

// CaptureChrome takes a screenshot and saves it as a PNG image to the specified
// file path. It will use Chrome to perform the screen capture.
func CaptureChrome(ctx context.Context, cr *chrome.Chrome, path string) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	var base64PNG string
	if err = tconn.EvalPromise(ctx,
		`new Promise(function(resolve, reject) {
		   chrome.autotestPrivate.takeScreenshot(function(base64PNG) {
		     if (chrome.runtime.lastError === undefined) {
		       resolve(base64PNG);
		     } else {
		       reject(chrome.runtime.lastError.message);
		     }
		   });
		 })`, &base64PNG); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	sr := strings.NewReader(base64PNG)
	if _, err = io.Copy(f, base64.NewDecoder(base64.StdEncoding, sr)); err != nil {
		return err
	}
	return nil
}

// TODO(derat): Refactor the comparison functions into their own package.

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
