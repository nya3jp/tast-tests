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
	"image/color"
	"io"
	"os"
	"strings"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
)

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

// RGB returns a fully-opaque color.Color based on the supplied components.
func RGB(r, g, b uint8) color.Color {
	return color.NRGBA{R: r, G: g, B: b, A: 0xff}
}

// ColorStr converts clr to 8-bit-per-channel, non-pre-alpha-multiplied RGBA and
// returns either a "#rrggbb" representation if it's fully opaque or "#rrggbbaa" otherwise.
func ColorStr(clr color.Color) string {
	nrgba := toNRGBA(clr)
	if nrgba.A == 0xff {
		return fmt.Sprintf("#%02x%02x%02x", nrgba.R, nrgba.G, nrgba.B)
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", nrgba.R, nrgba.G, nrgba.B, nrgba.A)
}

// toNRGBA converts clr to a color.NRGBA.
func toNRGBA(clr color.Color) color.NRGBA {
	return color.NRGBAModel.Convert(clr).(color.NRGBA)
}

// DominantColor returns the color that occupies the largest number of pixels
// in the passed in image. It also returns the ratio of that pixel to the number
// of overall pixels in the image.
func DominantColor(im image.Image) (clr color.Color, ratio float64) {
	counts := map[color.NRGBA]int{}
	box := im.Bounds()
	for x := box.Min.X; x < box.Max.X; x++ {
		for y := box.Min.Y; y < box.Max.Y; y++ {
			counts[toNRGBA(im.At(x, y))]++
		}
	}

	var bestClr color.NRGBA
	bestCnt := 0
	for clr, cnt := range counts {
		if cnt > bestCnt {
			bestClr = clr
			bestCnt = cnt
		}
	}
	return bestClr, float64(bestCnt) / float64(box.Dx()*box.Dy())
}

// ColorsMatch takes two colors and returns whether or not each component is within
// maxDiff of each other after conversion to 8-bit-per-channel, non-alpha-premultiplied RGBA.
func ColorsMatch(a, b color.Color, maxDiff uint8) bool {
	an := toNRGBA(a)
	bn := toNRGBA(b)

	allowed := int(maxDiff)
	near := func(x uint8, y uint8) bool {
		d := int(x) - int(y)
		return -allowed <= d && d <= allowed
	}
	return near(an.R, bn.R) && near(an.G, bn.G) && near(an.B, bn.B) && near(an.A, bn.A)
}
