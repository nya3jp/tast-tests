// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webcodecs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/devtools"
	"chromiumos/tast/local/media/logging"
)

// TestDecodeArgs is the arguments used in RunDecodeTest.
type TestDecodeArgs struct {
	// VideoFile is the video file used in RunDecodeTest.
	VideoFile string
	// Acceleration denotes which decoder is used, hardware or software.
	Acceleration HardwareAcceleration
}

var bear320pVideoConfig = videoConfig{width: 320, height: 240, numFrames: 82, framerate: 30}

const decodeHTML = "webcodecs_decode.html"

// DecodeDataFiles returns the HTML and JS files used in RunDecodeTest.
func DecodeDataFiles() []string {
	return []string{
		decodeHTML,
		"webcodecs_common.js",
		"webcodecs_decode.js",
	}
}

// RunDecodeTest tests encoding in WebCodecs API. It verifies a specified encoder is used.
func RunDecodeTest(ctx context.Context, cr *chrome.Chrome, fileSystem http.FileSystem, testArgs TestDecodeArgs, outDir string) error {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		return errors.Wrap(err, "failed to set values for verbose logging")
	}
	defer vl.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 90*time.Second)
	defer cancel()

	server := httptest.NewServer(http.FileServer(fileSystem))
	defer server.Close()

	conn, err := cr.NewConn(ctx, server.URL+"/"+decodeHTML)
	if err != nil {
		return errors.Wrap(err, "failed to open webcodecs page")
	}
	defer conn.Close()
	defer conn.CloseTarget(cleanupCtx)

	if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		return errors.Wrap(err, "timed out waiting for page loading")
	}
	observer, err := conn.GetMediaPropertiesChangedObserver(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to retrieve a media DevTools observer")
	}

	width := bear320pVideoConfig.width
	height := bear320pVideoConfig.height
	numFrames := bear320pVideoConfig.numFrames
	if err := conn.Call(ctx, nil, "DecodeFrames", server.URL+"/"+testArgs.VideoFile, width, height, numFrames, testArgs.Acceleration); err != nil {
		return outputJSLogAndError(cleanupCtx, conn, errors.Wrap(err, "failed executing DecodeFrames"))
	}

	// Check if a preferred decoder is used.
	isPlatform, name, err := devtools.GetVideoDecoder(ctx, observer, server.URL+"/"+decodeHTML)
	if err != nil {
		return errors.Wrap(err, "failed getting decoder type")
	}
	if testArgs.Acceleration == PreferHardware && !isPlatform {
		return errors.Errorf("video is decoded by a software decoder, %s", name)
	} else if testArgs.Acceleration == PreferSoftware && isPlatform {
		return errors.Errorf("video is decoded by a hardware decoder, %s", name)
	}

	// TODO(b/173164490): Add md5 check supports
	return nil
}
