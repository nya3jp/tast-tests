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
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/testexec"
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

// CaptureElementOptions provides all of the ways which you can configure the CaptureElement function.
type CaptureElementOptions struct {
	// By default, taking a screenshot will hide any notifications which might be overlaid on top of the element.
	// Set to true if you don't want this behaviour.
	LeaveNotifications bool

	// The params to be passed to the GetNode function.
	Params ui.FindParams
	// A function that attempts to find a node. If not provided, defaults to ui.FindSingleton.
	GetNode func(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams) (*ui.Node, error)
}

// FillDefaults fills defaults for fields in CaptureElementOptions.
func (options *CaptureElementOptions) FillDefaults() {
	if options.GetNode == nil {
		options.GetNode = ui.FindSingleton
	}
}

// CaptureElement takes a screenshot of the ui element described by ui.FindParams and saves it as a PNG image to the specified file path.
func CaptureElement(ctx context.Context, cr *chrome.Chrome, path string, params ui.FindParams) error {
	return CaptureElementWithOptions(ctx, cr, path, CaptureElementOptions{Params: params})
}

// CaptureElementWithOptions takes a screenshot of the ui element described by ui.FindParams and saves it as a PNG image to the specified file path.
func CaptureElementWithOptions(ctx context.Context, cr *chrome.Chrome, path string, options CaptureElementOptions) error {
	options.FillDefaults()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}

	node, err := options.GetNode(ctx, tconn, options.Params)
	if err != nil {
		return err
	}
	boundsDp := node.Location
	if !options.LeaveNotifications {
		ash.HideAllNotificationsAndWait(ctx, tconn)
	}
	var base64PNG string
	if err := tconn.EvalPromise(ctx,
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
	return png.Encode(f, cropped)
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

func captureInternal(ctx context.Context, path string, eval func(code string, out interface{}) error) error {
	var base64PNG string
	if err := eval(
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
