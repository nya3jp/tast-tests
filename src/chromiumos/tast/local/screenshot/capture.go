// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package screenshot

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"image"
	"image/draw"
	"image/png"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
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

// KeysFile is the name of the file containing the relevant key-value pairs for generating the screenshot (eg. resolution, language).
const KeysFile = "keys.json"

// ScreenshotFile is the name of the file containing the screenshot.
const ScreenshotFile = "screenshot.png"

// Capture takes a screenshot and saves it as a PNG image to the specified file
// path. It will use the CLI screenshot command to perform the screen capture.
func Capture(ctx context.Context, path string) error {
	cmd := testexec.CommandContext(ctx, "screenshot", path)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Errorf("failed running %q", strings.Join(cmd.Args, " "))
	}
	return nil
}

type state interface {
	TestName() string
}

// Diff writes a screenshot of a ui element to ScreenshotFile and any relevant parameters (eg. screen resolution, font size) to KeysFile.
// In order to actually get the diff results, either use the ScreenDiffFixture or call UploadGoldDiffs()
func Diff(ctx context.Context, s state, cr *chrome.Chrome, testName string, params ui.FindParams) error {
	return DiffWithOptions(ctx, s, cr, testName, DiffTestOptions{FindParams: params})
}

// DiffWithOptions writes a screenshot of a ui element to ScreenshotFile and any relevant parameters (eg. screen resolution, font size) to KeysFile.
// In order to actually get the diff results, either use the ScreenDiffFixture or call UploadGoldDiffs()
func DiffWithOptions(ctx context.Context, s state, cr *chrome.Chrome, testName string, options DiffTestOptions) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}

	node, err := ui.FindSingleton(ctx, tconn, options.FindParams)
	if err != nil {
		return err
	}
	defer node.Release(ctx)
	boundsDp := node.Location

	info, err := display.FindInfo(ctx, tconn, func(info *display.Info) bool {
		return info.Bounds.Contains(boundsDp)
	})
	if err != nil {
		return err
	}

	displayMode, err := info.GetSelectedMode()
	if err != nil {
		return err
	}

	scale, err := info.GetEffectiveDeviceScaleFactor()
	if err != nil {
		return err
	}
	boundsPx := coords.ConvertBoundsFromDPToPX(boundsDp, scale)

	params := map[string]string{
		"resolution": fmt.Sprintf("%dx%d", displayMode.WidthInNativePixels, displayMode.HeightInNativePixels),
		"scale":      fmt.Sprintf("%.2f", scale),
		"region":     cr.Region(),
	}

	dir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("couldn't get output dir")
	}

	// Include the hash of the params for 2 reasons:
	// 1) To allows creating the same screenshot with different params without colliding (eg. en/jp versions).
	// 2) To ensure that each test only runs with the same name & params a single time.
	dir = filepath.Join(dir, "screenshots", fmt.Sprintf("%s.%s-%x", s.TestName(), testName, hash(params)))
	if _, err := os.Stat(dir); err == nil {
		return errors.Errorf("screenshot has already been taken for test %s with name %s and params %+v", s.TestName(), testName, params)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	jsonString, err := json.Marshal(params)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(dir, KeysFile), jsonString, 0644); err != nil {
		return err
	}

	if !options.LeaveNotifications {
		ash.HideVisibleNotificationsAndWait(ctx, tconn)
	}

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

	// The screenshot returned is of the whole screen. Crop it to only contain the element requested by the user.
	srcOffset := image.Point{X: boundsPx.Left, Y: boundsPx.Top}
	dstSize := image.Rect(0, 0, boundsPx.Width, boundsPx.Height)
	cropped := image.NewRGBA(dstSize)
	draw.Draw(cropped, dstSize, src, srcOffset, draw.Src)

	f, err := os.Create(filepath.Join(dir, ScreenshotFile))
	if err != nil {
		return err
	}
	png.Encode(f, cropped)
	return nil
}

// DiffTestOptions provides all of the ways which you can configure the DiffTest function.
type DiffTestOptions struct {
	// By default, taking a screenshot will hide any notifications which might be overlaid on top of the element.
	// Set to true if you don't want this behaviour.
	LeaveNotifications bool

	// The params used to find the node that we're looking for.
	FindParams ui.FindParams
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

const (
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
	var base64PNG string
	if err := tconn.EvalPromise(ctx, takeScreenshot, &base64PNG); err != nil {
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

func screenshotBase64PNG(eval func(code string, out interface{}) error) (string, error) {
	var base64PNG string
	err := eval(takeScreenshot, &base64PNG)
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

func hash(m map[string]string) uint64 {
	h := fnv.New64()
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		h.Write([]byte(key))
		h.Write([]byte(m[key]))
	}
	return h.Sum64()
}
