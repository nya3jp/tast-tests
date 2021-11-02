// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webcodecs

import (
	"context"
	"net/http"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/devtools"
)

// TestDecodeArgs is the arguments used in RunDecodeTest.
type TestDecodeArgs struct {
	// VideoFile is the video file used in RunDecodeTest.
	VideoFile string
	// Acceleration denotes which decoder is used, hardware or software.
	Acceleration HardwareAcceleration
}

const decodeHTML = "webcodecs_decode.html"

// DecodeDataFiles returns the HTML and JS files used in RunDecodeTest.
func DecodeDataFiles() []string {
	return []string{
		decodeHTML,
		"webcodecs_common.js",
		"webcodecs_decode.js",
	}
}

// RunDecodeTest tests decoding in WebCodecs API. It verifies a specified decoder is used.
func RunDecodeTest(ctx context.Context, cr *chrome.Chrome, fileSystem http.FileSystem, testArgs TestDecodeArgs) error {

	cleanupCtx, server, conn, observer, deferFunc, err := prepareWebCodecsTest(ctx, cr, fileSystem, decodeHTML)
	if err != nil {
		return err
	}
	defer deferFunc()

	// width, height and numFrames for bear320p.
	const width = 320
	const height = 240
	const numFrames = 82
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
