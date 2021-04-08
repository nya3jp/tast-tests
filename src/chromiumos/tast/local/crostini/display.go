// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"bytes"
	"context"
	"image/color"
	"image/png"
	"os"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/crostini/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

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
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}
	return nil
}

// PollWindowSize returns the the width and the height of the window in pixels
// with polling to wait for asynchronous rendering on the DUT.
func PollWindowSize(ctx context.Context, tconn *chrome.TestConn, name string, timeout time.Duration) (sz coords.Size, err error) {
	// Allow up to 10 seconds for the target screen to render.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		sz, err = windowSize(ctx, tconn, name)
		return err
	}, &testing.PollOptions{Timeout: timeout})
	if err != nil {
		faillog.DumpUITreeAndScreenshot(ctx, tconn, "poll_window", err)
	}
	return sz, err
}

// windowSize returns the the width and the height of the window in pixels.
func windowSize(ctx context.Context, tconn *chrome.TestConn, name string) (sz coords.Size, err error) {
	ui := uiauto.New(tconn)
	appWindow := nodewith.Name(name).First()
	if err := ui.WaitUntilExists(appWindow)(ctx); err != nil {
		return coords.Size{}, errors.Wrap(err, "failed to locate the app window")
	}

	// Apps can open extra "degenerate" windows. We look for the first window with
	// a client view that has a non-empty location node.
	for i := 0; i < 4; i++ {
		view := nodewith.ClassName("ClientView").Nth(i)
		loc, err := ui.WithTimeout(15*time.Second).Location(ctx, view)
		if err == nil {
			if loc.Empty() {
				continue
			}
			return loc.Size(), nil
		}
	}

	return coords.Size{}, errors.Wrap(err, "failed to find client view location node")
}

// PrimaryDisplayScaleFactor returns the primary display's scale factor.
func PrimaryDisplayScaleFactor(ctx context.Context, tconn *chrome.TestConn) (factor float64, err error) {
	err = tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.getPrimaryDisplayScaleFactor)()`, &factor)
	return factor, err
}

// VerifyWindowDensities compares the sizes, which should be from
// PollWindowSize() at low and high density. It returns an error if
// something is wrong with the sizes (not just if the high-density
// window is bigger).
func VerifyWindowDensities(ctx context.Context, tconn *chrome.TestConn, sizeHighDensity, sizeLowDensity coords.Size) error {
	if sizeHighDensity.Width > sizeLowDensity.Width || sizeHighDensity.Height > sizeLowDensity.Height {
		return errors.Errorf("app high density size %v greater than low density size %v", sizeHighDensity, sizeLowDensity)
	}

	tabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed getting tablet mode")
	}

	factor, err := PrimaryDisplayScaleFactor(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed getting primary display scale factor")
	}

	if factor != 1.0 && !tabletMode && (sizeHighDensity.Width == sizeLowDensity.Width || sizeHighDensity.Height == sizeLowDensity.Height) {
		return errors.Errorf("app has high density and low density windows with the same size of %v while the scale factor is %v and tablet mode is %v", sizeHighDensity, factor, tabletMode)
	}
	return nil
}

// RunWindowedApp Runs the command cmdline in the container, waits
// for the window windowName to open, sends it a key press event,
// runs condition, and then closes all open windows. Note that this
// will close windows other then the one with title windowName! The
// return value is a string containing the what program wrote to
// stdout. The intended use of condition is to delay closing the
// application window until some event has occurred. If condition
// returns an error then the call will be considered a failure and the
// error will be propagated.
func RunWindowedApp(ctx context.Context, tconn *chrome.TestConn, cont *vm.Container, keyboard *input.KeyboardEventWriter, timeout time.Duration, condition func(context.Context) error, closeWindow bool, windowName string, cmdline []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	testing.ContextLogf(ctx, "Starting %v application", windowName)
	cmd := cont.Command(ctx, cmdline...)
	var buf bytes.Buffer
	cmd.Stdout = &buf

	if err := cmd.Start(); err != nil {
		return "", errors.Wrapf(err, "failed to start command %v", cmdline)
	}
	defer cmd.Wait(testexec.DumpLogOnError)

	size, err := PollWindowSize(ctx, tconn, windowName, timeout)
	if err != nil {
		return "", errors.Wrapf(err, "failed to find window %q while running %v", windowName, cmdline)
	}
	testing.ContextLogf(ctx, "Window %q is visible with size %v", windowName, size)
	testing.ContextLog(ctx, "Sending keypress to ", windowName)
	if err := keyboard.Type(ctx, " "); err != nil {
		return "", errors.Wrapf(err, "failed to send keypress to window while running %v", cmdline)
	}

	if condition != nil {
		if err := condition(ctx); err != nil {
			return "", errors.Wrapf(err, "failed to check condition closure while running %v", cmdline)
		}
	}

	if closeWindow {
		// TODO(crbug.com/996609) Change this to only close the window that just got opened.
		testing.ContextLog(ctx, "Closing all windows")
		if err := CloseAllWindows(ctx, tconn); err != nil {
			return "", errors.Wrapf(err, "failed to close all windows while running %v", cmdline)
		}
	}

	if err := cmd.Wait(testexec.DumpLogOnError); err != nil {
		return "", errors.Wrapf(err, "command %v failed to terminate properly", cmdline)
	}

	return string(buf.Bytes()), nil
}

// CloseAllWindows closes all currently open windows by iterating over
// the shelf icons and calling autotestPrivate.closeApp on each one.
func CloseAllWindows(ctx context.Context, tconn *chrome.TestConn) error {
	return tconn.Eval(ctx, `(async () => {
		  let items = await tast.promisify(chrome.autotestPrivate.getShelfItems)();
		  await Promise.all(items.map(item =>
		      tast.promisify(chrome.autotestPrivate.closeApp)(
		          item.appId.toString())));
		})()`, nil)
}
