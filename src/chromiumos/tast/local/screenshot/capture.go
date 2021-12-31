// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package screenshot

import (
	"context"
	"encoding/base64"
	"image"
	_ "image/png" // PNG decoder
	"io"
	"io/ioutil"
	"os"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/coords"
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

// CaptureWithStderr differs from Capture in that it returns the stderr when
// capturing a screenshot fails. This is useful for verification on whether turning display
// on/off is successful by matching with the message, "CRTC not found. Is the screen on?".
func CaptureWithStderr(ctx context.Context, path string) error {
	_, stderr, err := testexec.CommandContext(ctx, "screenshot", path).SeparatedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed running %q", stderr)
	}
	return nil
}

// CaptureChrome takes a screenshot of the primary display and saves it as a PNG
// image to the specified file path. It will use Chrome to perform the screen capture.
func CaptureChrome(ctx context.Context, cr *chrome.Chrome, path string) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	return captureInternal(ctx, path, func(code string, out interface{}) error {
		return tconn.Eval(ctx, code, out)
	})
}

const (
	// Do not use tast.promisify(), because this may be evaluated on the connection
	// other than TestAPIConn.
	takeScreenshot = `new Promise(function(resolve, reject) {
		chrome.autotestPrivate.takeScreenshot(function(base64PNG) {
		  if (chrome.runtime.lastError === undefined) {
			resolve(base64PNG);
		  } else {
			reject(chrome.runtime.lastError.message);
		  }
		});
	  })`
)

// CaptureChromeImage takes a screenshot of the primary display and returns
// it as an image.Image. It will use Chrome to perform the screen capture.
func CaptureChromeImage(ctx context.Context, cr *chrome.Chrome) (image.Image, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}
	return CaptureChromeImageWithTestAPI(ctx, tconn)
}

// CaptureChromeImageWithTestAPI takes a screenshot of the primary display and
// returns it as an image.Image. It will use Test API to perform the screen
// capture.
func CaptureChromeImageWithTestAPI(ctx context.Context, tconn *chrome.TestConn) (image.Image, error) {
	var base64PNG string
	if err := tconn.Eval(ctx, takeScreenshot, &base64PNG); err != nil {
		return nil, err
	}
	sr := strings.NewReader(base64PNG)
	img, _, err := image.Decode(base64.NewDecoder(base64.StdEncoding, sr))
	return img, err
}

// CaptureCDP takes a screenshot and saves it as a PNG image at path, similar to
// CaptureChrome.
// The diff from CaptureChrome is that this function takes *cdputil.Conn, which
// is used by chrome.Conn. Thus, CaptureChrome records logs in case of error,
// while this does not. XXX
func CaptureCDP(ctx context.Context, conn *ash.DevtoolsConn, path string) error {
	return captureInternal(ctx, path, func(code string, out interface{}) error {
		_, err := conn.Eval(ctx, code, true /* awaitPromise */, out)
		return err
	})
}

func captureInternal(ctx context.Context, path string, eval func(code string, out interface{}) error) error {
	var base64PNG string
	if err := eval(takeScreenshot, &base64PNG); err != nil {
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
	var base64PNG string
	if err := tconn.Call(ctx, &base64PNG, "tast.promisify(chrome.autotestPrivate.takeScreenshotForDisplay)", displayID); err != nil {
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

// GrabAndCropScreenshot grabs a screenshot and crops it to the specified bounds.
func GrabAndCropScreenshot(ctx context.Context, cr *chrome.Chrome, bounds coords.Rect) (image.Image, error) {
	img, err := GrabScreenshot(ctx, cr)
	if err != nil {
		return nil, err
	}

	subImage := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(image.Rect(bounds.Left, bounds.Top, bounds.Right(), bounds.Bottom()))

	return subImage, nil
}

// GrabScreenshot creates a screenshot and returns an image.Image.
// The path of the image is generated ramdomly in /tmp.
func GrabScreenshot(ctx context.Context, cr *chrome.Chrome) (image.Image, error) {
	fd, err := ioutil.TempFile("", "screenshot")
	if err != nil {
		return nil, errors.Wrap(err, "error opening screenshot file")
	}
	defer os.Remove(fd.Name())
	defer fd.Close()

	if err := CaptureChrome(ctx, cr, fd.Name()); err != nil {
		return nil, errors.Wrap(err, "failed to capture screenshot")
	}

	img, _, err := image.Decode(fd)
	if err != nil {
		return nil, errors.Wrap(err, "error decoding image file")
	}
	return img, nil
}
