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
	_ "image/png" // PNG decoder
	"io"
	"os"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cdputil"
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

// CaptureChromeImage takes a screenshot of the primary display and returns
// it as an image.Image. It will use Chrome to perform the screen capture.
func CaptureChromeImage(ctx context.Context, cr *chrome.Chrome) (image.Image, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
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
