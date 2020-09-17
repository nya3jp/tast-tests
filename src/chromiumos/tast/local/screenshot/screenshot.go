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
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Capture takes a screenshot and saves it as a PNG image to the specified file
// path. It will use the CLI screenshot command to perform the screen capture.
func Capture(ctx context.Context, path string) error {
	cmd := testexec.CommandContext(ctx, "screenshot", path)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Errorf("failed running %q", strings.Join(cmd.Args, " "))
	}
	return nil
}

type skiaScreenshottable interface {
	// SkiaScreenshot reports that a screenshot has been taken to send for skia for diff testing.
	SkiaScreenshot(name string, screenshot *image.RGBA, keyValueMap map[string]string) error
}

// DiffTest takes a screenshot of the ui element described by `params` and sends it off to skia for diff testing.
func DiffTest(ctx context.Context, s skiaScreenshottable, cr *chrome.Chrome, testName string, params ui.FindParams) error {
	return DiffTestWithOptions(ctx, s, cr, testName, DiffTestOptions{Params: params})
}

// DiffTestWithOptions takes a screenshot of a particular ui element and sends it off to skia for diff testing.
func DiffTestWithOptions(ctx context.Context, s skiaScreenshottable, cr *chrome.Chrome, testName string, options DiffTestOptions) error {
	dir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("couldn't get output dir")
	}
	path := filepath.Join(dir, testName+".png")

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	if !options.LeaveNotifications {
		ash.HideAllNotificationsAndWait(ctx, tconn)
	}

	node, err := ui.FindSingleton(ctx, tconn, options.Params)
	if err != nil {
		return err
	}

	boundsDp := node.Location
	base64PNG, err := screenshotBase64PNG(func(code string, out interface{}) error {
		return tconn.EvalPromise(ctx, code, out)
	})
	if err != nil {
		return err
	}

	src, _, err := image.Decode(base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64PNG)))
	if err != nil {
		return err
	}

	info, err := display.FindInfo(ctx, tconn, func(info *display.Info) bool {
		return info.Bounds.Contains(boundsDp)
	})
	if err != nil {
		return err
	}
	scale, err := info.GetEffectiveDeviceScaleFactor()
	if err != nil {
		return err
	}
	boundsPx := coords.ConvertBoundsFromDPToPX(boundsDp, scale)

	// The screenshot returned is of the whole screen. Crop it to only contain the element requested by the user.
	srcOffset := image.Point{X: boundsPx.Left, Y: boundsPx.Top}
	dstSize := image.Rect(0, 0, boundsPx.Width, boundsPx.Height)
	cropped := image.NewRGBA(dstSize)
	draw.Draw(cropped, dstSize, src, srcOffset, draw.Src)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	png.Encode(f, cropped)

	displayMode, err := info.GetSelectedMode()
	if err != nil {
		return err
	}

	// TODO(msta): Determine an appropriate and useful set of parameters to use as key/value pairs.
	return s.SkiaScreenshot(testName, cropped, map[string]string{
		"resolution": fmt.Sprintf("%dx%d", displayMode.WidthInNativePixels, displayMode.HeightInNativePixels),
		"scale":      fmt.Sprintf("%.2f", scale),
	})
}

// DiffTestOptions provides all of the ways which you can configure the DiffTest function.
type DiffTestOptions struct {
	// By default, taking a screenshot will hide any notifications which might be overlaid on top of the element.
	// Set to true if you don't want this behaviour.
	LeaveNotifications bool

	// The params used to find the node that we're looking for.
	Params ui.FindParams
}

// CaptureChrome takes a screenshot of the primary display and saves it as a PNG
// image to the specified file path. It will use Chrome to perform the screen capture.
func CaptureChrome(ctx context.Context, cr *chrome.Chrome, path string) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	return captureInternal(ctx, path, func(code string, out interface{}) error {
		return tconn.EvalPromise(ctx, code, out)
	})
}

// CaptureCDP takes a screenshot and saves it as a PNG image at path, similar to
// CaptureChrome.
// The diff from CaptureChrome is that this function takes *cdputil.Conn, which
// is used by chrome.Conn. Thus, CaptureChrome records logs in case of error,
// while this does not.
func CaptureCDP(ctx context.Context, conn *cdputil.Conn, path string) error {
	return captureInternal(ctx, path, func(code string, out interface{}) error {
		_, err := conn.Eval(ctx, code, true /* awaitPromise */, out)
		return err
	})
}

func screenshotBase64PNG(eval func(code string, out interface{}) error) (string, error) {
	var base64PNG string
	err := eval(
		`new Promise(function(resolve, reject) {
		   chrome.autotestPrivate.takeScreenshot(function(base64PNG) {
		     if (chrome.runtime.lastError === undefined) {
		       resolve(base64PNG);
		     } else {
		       reject(chrome.runtime.lastError.message);
		     }
		   });
		 })`, &base64PNG)
	return base64PNG, err
}

func captureInternal(ctx context.Context, path string, eval func(code string, out interface{}) error) error {
	base64PNG, err := screenshotBase64PNG(eval)
	if err != nil {
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

// CaptureChromeForDisplay takes a screenshot for a given displayID and saves it as a PNG
// image to the specified file path. It will use Chrome to perform the screen capture.
func CaptureChromeForDisplay(ctx context.Context, cr *chrome.Chrome, displayID, path string) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.takeScreenshotForDisplay(%q, function(base64PNG) {
		    if (chrome.runtime.lastError === undefined) {
		      resolve(base64PNG);
		    } else {
		      reject(chrome.runtime.lastError.message);
		    }
		  });
		})`, displayID)

	var base64PNG string
	if err := tconn.EvalPromise(ctx, expr, &base64PNG); err != nil {
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
