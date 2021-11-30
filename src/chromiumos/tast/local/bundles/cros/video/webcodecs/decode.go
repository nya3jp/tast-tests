// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webcodecs

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/devtools"
	"chromiumos/tast/local/media/encoding"
	"chromiumos/tast/testing"
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

type planeLayout struct {
	Offset uint `json:"offset"`
	Stride uint `json:"stride"`
}

type decodedFrameInfo struct {
	Data    []byte        `json:"frameBuffer"`
	Format  string        `json:"format"`
	Layouts []planeLayout `json:"layouts"`
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

// createMD5File creates a temporary file in which md5s are listed. The filepath of the created file is returned.
func createMD5File(md5s []string) (string, error) {
	md5File, err := encoding.CreatePublicTempFile("gotMD5s")
	if err != nil {
		return "", errors.Wrap(err, "failed to create a temporary file for md5 values")
	}

	defer func() {
		if err != nil {
			os.Remove(md5File.Name())
		}
	}()

	closed := false
	defer func() {
		if !closed {
			md5File.Close()
		}
	}()

	for _, md5 := range md5s {
		if sz, err := md5File.WriteString(md5 + "\n"); err != nil {
			return "", errors.Wrap(err, "failed to write md5 value to file")
		} else if sz != len(md5)+1 {
			return "", errors.Wrapf(err, "unexpected write size: %d", sz)
		}
	}

	closed = true
	if err := md5File.Close(); err != nil {
		return "", errors.Wrap(err, "failed to close a md5 file")
	}

	return md5File.Name(), nil
}

// validateMD5s compares gotMD5s with golden md5 checksums in jsonFilePath.
func validateMD5s(ctx context.Context, gotMD5s []string, jsonFilePath, outDir string) error {
	const validatePath = "/usr/local/graphics/validate"

	md5FilePath, err := createMD5File(gotMD5s)
	if err != nil {
		return errors.Wrap(err, "failed to create md5 file")
	}
	defer os.Remove(md5FilePath)

	stdout, stderr, err := testexec.CommandContext(
		ctx,
		validatePath,
		"--exec=cat",
		"--args="+md5FilePath,
		"--metadata="+jsonFilePath).SeparatedOutput(testexec.DumpLogOnError)
	if err != nil {
		testing.ContextLog(ctx, "md5 validation fails, dumping validate log to validate.log")

		if err := ioutil.WriteFile(filepath.Join(outDir, "validate.log"),
			append(stdout, append([]byte("\n === stderr ===\n"), stderr...)...), 0644); err != nil {
			return errors.Wrap(err, "failed to dump output of validate")
		}
		return errors.Wrap(err, "md5 validation fails")
	}

	return nil
}

// RunDecodeTest tests encoding in WebCodecs API. It verifies a specified encoder is used and
// the decoded frames with md5 checksums in jsonFilePath.
func RunDecodeTest(ctx context.Context, cr *chrome.Chrome, fileSystem http.FileSystem, testArgs TestDecodeArgs, jsonFilePath, outDir string) error {
	cleanupCtx, server, conn, observer, deferFunc, err := prepareWebCodecsTest(ctx, cr, fileSystem, decodeHTML)
	if err != nil {
		return err
	}
	defer deferFunc()

	// width, height and numFrames for bear320p.
	const width = 320
	const height = 240
	const numFrames = 82
	decodedFrames := &chrome.JSObject{}
	if err := conn.Call(ctx, decodedFrames, "DecodeFrames", server.URL+"/"+testArgs.VideoFile, width, height, numFrames, testArgs.Acceleration); err != nil {
		return outputJSLogAndError(cleanupCtx, conn, errors.Wrap(err, "failed executing DecodeFrames"))
	}
	defer decodedFrames.Release(cleanupCtx)

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

	numDecodedFrames := 0
	if decodedFrames.Call(ctx, &numDecodedFrames, `function() { return this.length(); }`); err != nil {
		return outputJSLogAndError(cleanupCtx, conn, errors.Wrap(err, "failed getting the number of decoded frames"))
	}
	if numDecodedFrames != numFrames {
		return outputJSLogAndError(cleanupCtx, conn, errors.Errorf("unexpected number of decoded frames, %d", numDecodedFrames))
	}

	var gotMD5s []string
	for i := 0; i < numFrames; i++ {
		var frameInfo decodedFrameInfo
		// We can get a raw frame because the size (320 * 240 * 3 / 2 + base64 conversion) is
		// less than the tast websocket limitation, 1MB declared in session.go in package cdputil.
		if err := decodedFrames.Call(ctx, &frameInfo, "function(index) { return this.getFrameInfo(index); }", i); err != nil {
			return outputJSLogAndError(cleanupCtx, conn, errors.Wrap(err, "failed getting frame"))
		}

		if isRGBFormat(frameInfo.Format) {
			testing.ContextLogf(ctx, "skip md5 validation skip because decoded frames are RGB format and thus decoded content is device dependent: %s", frameInfo.Format)
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

	return validateMD5s(ctx, gotMD5s, jsonFilePath, outDir)
}
