// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webcodecs

import (
	"bufio"
	"context"
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/devtools"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/testing"
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

type planeLayout struct {
	Offset uint `json:"offset"`
	Stride uint `json:"stride"`
}

type decodedFrameInfo struct {
	Data    []byte        `json:"frameBuffer"`
	Format  string        `json:"format"`
	Layouts []planeLayout `json:"layouts"`
}

// readExpectedMD5s reads ".md5" file, where md5 values are listed, and returns
// them. Fails if the number of md5 values is not numMD5s.
func readExpectedMD5s(md5FilePath string, numMD5s int) ([]string, error) {
	f, err := os.Open(md5FilePath)
	if err != nil {
		return []string{}, err
	}
	defer f.Close()

	var md5s []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		md5s = append(md5s, scanner.Text())
	}

	if len(md5s) != numMD5s {
		return md5s, errors.Errorf("the number of read md5 values mismatch, got=%d, expected=%d", len(md5s), numMD5s)
	}
	return md5s, nil
}

// computeMD5 returns the md5sum of a visible rectangle of the YUV frame. The
// visible rectangle of the frame must be (0, 0, vw, vh).
func computeMD5(frameInfo decodedFrameInfo, vw, vh int) (string, error) {
	if frameInfo.Format != "I420" {
		return "unsupported", errors.Errorf("unsupported pixel format %s", frameInfo.Format)
	}

	hasher := md5.New()
	for i, layout := range frameInfo.Layouts {
		offset := layout.Offset
		stride := layout.Stride
		cvw, cvh := vw, vh
		if i > 0 {
			cvw /= 2
			cvh /= 2
		}
		for h := 0; h < cvh; h++ {
			hasher.Write(frameInfo.Data[offset+uint(h)*stride : offset+uint(h)*stride+uint(cvw)])
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// isRGBFormat returns true if and only if the format is one of RGB formats defined in VideoPixelFormat.
// https://www.w3.org/TR/webcodecs/#pixel-format
func isRGBFormat(format string) bool {
	switch format {
	case "RGBA", "RGBX", "BGRA", "BGRX":
		return true
	default:
		return false
	}
}

// RunDecodeTest tests encoding in WebCodecs API. It verifies a specified encoder is used and
// the decoded frames with md5 checksums in md5FilePath.
func RunDecodeTest(ctx context.Context, cr *chrome.Chrome, fileSystem http.FileSystem, testArgs TestDecodeArgs, md5FilePath, outDir string) error {
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

	var gotMD5s []string
	for i := 0; i < numFrames; i++ {
		var frameInfo decodedFrameInfo
		// We can get a raw frame because the size (320 * 240 * 3 / 2 + base64 conversion) is
		// less than the tast websocket limitation, 1MB declared in session.go in package cdputil.
		if err := conn.Call(ctx, &frameInfo, "GetFrame", i); err != nil {
			return outputJSLogAndError(cleanupCtx, conn, errors.Wrap(err, "failed getting frame"))
		}

		if isRGBFormat(frameInfo.Format) {
			// Skip md5 validation.
			return nil
		}
		md5, err := computeMD5(frameInfo, width, height)
		if err != nil {
			return errors.Wrap(err, "failed computing md5")
		}
		gotMD5s = append(gotMD5s, md5)
	}

	if len(gotMD5s) != numFrames {
		return errors.Errorf("the number of md5 values mismatch: got=%d, expected=%d", len(gotMD5s), numFrames)
	}

	expectedMD5s, err := readExpectedMD5s(md5FilePath, numFrames)
	if err != nil {
		return errors.Wrapf(err, "failed reading expected MD5 values, %s", md5FilePath)
	}

	mismatchMD5cnt := 0
	for i := 0; i < numFrames; i++ {
		if gotMD5s[i] != expectedMD5s[i] {
			mismatchMD5cnt++
			testing.ContextLogf(ctx, "%d: %s (got), %s (expected)", i, gotMD5s[i], expectedMD5s[i])
		}
	}

	if mismatchMD5cnt > 0 {
		return errors.Errorf("%d frames are invalid", mismatchMD5cnt)
	}

	return outputJSLogAndError(cleanupCtx, conn, nil)
}
