// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// The Size object records sizes of various display-related objects (e.g. the
// screen resolution, a window's size).
type Size struct {
	W int `json:"width"`
	H int `json:"height"`
}

// MatchScreenshotDominantColor takes a screenshot and attempts to verify if it
// mostly (>= 1/2) contains the expected color. Will retry for up to 10 seconds
// if it fails. For logging purposes, the screenshot will be saved at the given
// path.
func MatchScreenshotDominantColor(ctx context.Context, cr *chrome.Chrome, expectedColor color.Color, screenshotPath string) error {
	if !strings.HasSuffix(screenshotPath, ".png") {
		return errors.New("Screenshots must have the '.png' extension, got: " + screenshotPath)
	}
	// Largest differing color known to date, we will be changing this over time
	// based on testing results.
	const maxKnownColorDiff = 0x1

	// Allow up to 10 seconds for the target screen to render.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := screenshot.CaptureChrome(ctx, cr, screenshotPath); err != nil {
			return err
		}
		f, err := os.Open(screenshotPath)
		if err != nil {
			return errors.Wrapf(err, "failed opening the screenshot image %v", screenshotPath)
		}
		defer f.Close()
		im, err := png.Decode(f)
		if err != nil {
			return errors.Wrapf(err, "failed decoding the screenshot image %v", screenshotPath)
		}
		color, ratio := colorcmp.DominantColor(im)
		if ratio >= 0.5 && colorcmp.ColorsMatch(color, expectedColor, maxKnownColorDiff) {
			return nil
		}
		return errors.Errorf("screenshot did not have matching dominant color, got %v at ratio %0.2f but expected %v",
			colorcmp.ColorStr(color), ratio, colorcmp.ColorStr(expectedColor))
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return err
	}
	return nil
}

// PollWindowSize returns the the width and the height of the window in pixels
// with polling to wait for asynchronous rendering on the DUT.
func PollWindowSize(ctx context.Context, tconn *chrome.Conn, name string) (sz Size, err error) {
	// Allow up to 10 seconds for the target screen to render.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		sz, err = getWindowSize(ctx, tconn, name)
		return err
	}, &testing.PollOptions{Timeout: 10 * time.Second})
	return sz, err
}

// getWindowSize returns the the width and the height of the window in pixels.
func getWindowSize(ctx context.Context, tconn *chrome.Conn, name string) (sz Size, err error) {
	expr := fmt.Sprintf(
		`new Promise((resolve, reject) => {
			chrome.automation.getDesktop(root => {
				const appWindow = root.find({ attributes: { name: %q}});
				if (!appWindow) {
					reject("Failed to locate the app window");
				}
				const view = appWindow.find({ attributes: { className: 'ClientView'}});
				if (!view) {
					reject("Failed to find client view");
				}
				resolve(view.location);
			})
		})`, name)
	err = tconn.EvalPromise(ctx, expr, &sz)
	return sz, err
}

// PrimaryDisplayScaleFactor returns the primary display's scale factor.
func PrimaryDisplayScaleFactor(ctx context.Context, tconn *chrome.Conn) (factor float64, err error) {
	err = tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.getPrimaryDisplayScaleFactor)()`, &factor)
	return factor, err
}

// TabletModeEnabled returns whether tablet mode is enabled on the device.
func TabletModeEnabled(ctx context.Context, tconn *chrome.Conn) (tabletMode bool, err error) {
	err = tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.isTabletModeEnabled)()`, &tabletMode)
	return tabletMode, err
}
