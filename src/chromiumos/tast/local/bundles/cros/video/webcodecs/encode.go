// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package webcodecs provides common code for video.WebCodecs* tests
package webcodecs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/devtools"
	"chromiumos/tast/local/media/encoding"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

// HardwareAcceleration represents the preference of used codecs in WebCodecs API.
// See https://www.w3.org/TR/webcodecs/#hardware-acceleration.
type HardwareAcceleration string

const (
	// PreferHardware means hardware accelerated encoder/decoder is preferred.
	PreferHardware HardwareAcceleration = "prefer-hardware"
	// PreferSoftware means software encoder/decoder is preferred.
	PreferSoftware HardwareAcceleration = "prefer-software"
)

// TestArgs is the arguments used in RunEncodeTest.
type TestArgs struct {
	// Codec is the codec of a bitstream produced by an encoder.
	Codec videotype.Codec
	// Acceleration denotes which encoder is used, hardware or software.
	Acceleration HardwareAcceleration
}

// MP4DemuxerDataFiles returns the list of JS files for demuxing MP4 container.
func MP4DemuxerDataFiles() []string {
	return []string{
		"third_party/mp4/mp4_demuxer.js",
		"third_party/mp4/mp4box.all.min.js",
	}
}

const encodeHTML = "webcodecs_encode.html"

// EncodeDataFiles returns the HTML and JS files used in RunEncodeTest.
func EncodeDataFiles() []string {
	return []string{
		encodeHTML,
		"webcodecs_common.js",
		"webcodecs_encode.js",
	}
}

// Crowd720p is 720p video data used in RunEncodeTest.
const Crowd720p = "crowd-1280x720_30frames.vp9.webm"
const crowd720pMP4 = "crowd-1280x720_30frames.h264.mp4"

// VideoDataFiles returns the webm and mp4 files used in RunEncodeTest.
func VideoDataFiles() []string {
	return []string{
		Crowd720p,
		crowd720pMP4,
	}
}

type videoConfig struct {
	width, height, numFrames, framerate int
}

var crowd720pVideoConfig = videoConfig{width: 1280, height: 720, numFrames: 30, framerate: 30}

// toMIMECodec converts videotype.Codec to codec in MIME type.
// See https://developer.mozilla.org/en-US/docs/Web/Media/Formats/codecs_parameter for detail.
func toMIMECodec(codec videotype.Codec) string {
	switch codec {
	case videotype.H264:
		// H.264 Baseline Level 3.1.
		return "avc1.42001E"
	case videotype.VP8:
		return "vp8"
	case videotype.VP9:
		// VP9 profile 0 level 1.0 8-bit depth.
		return "vp09.00.10.08"
	}
	return ""
}

func computeBitstreamQuality(ctx context.Context, videoFile, bitstreamFile, outDir string, codec videotype.Codec, w, h int) (psnr, ssim float64, err error) {
	yuvFile, err := encoding.PrepareYUV(ctx, videoFile, videotype.I420, coords.NewSize(0, 0) /* placeholder size */)
	if err != nil {
		return psnr, ssim, errors.Wrap(err, "failed to prepare YUV file")
	}
	defer os.Remove(yuvFile)

	var decoder encoding.Decoder
	switch codec {
	case videotype.H264:
		decoder = encoding.OpenH264Decoder
	case videotype.VP8, videotype.VP9:
		decoder = encoding.LibvpxDecoder
	}

	psnr, ssim, err = encoding.CompareFiles(ctx, decoder, yuvFile, bitstreamFile, outDir, coords.NewSize(w, h))
	if err != nil {
		return psnr, ssim, errors.Wrap(err, "failed to decode and compare results")
	}

	return psnr, ssim, nil
}

func outputJSLogAndError(ctx context.Context, conn *chrome.Conn, callErr error) error {
	var logs string
	if err := conn.Eval(ctx, "TEST.getLogs()", &logs); err != nil {
		testing.ContextLog(ctx, "Error getting TEST.logs: ", err)
	}
	testing.ContextLog(ctx, "log=", logs)
	return callErr
}

// RunEncodeTest tests encoding in WebCodecs API. It verifies a specified encoder is used and
// the produced bitstream.
func RunEncodeTest(ctx context.Context, cr *chrome.Chrome, fileSystem http.FileSystem, testArgs TestArgs, videoFile, outDir string) error {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		return errors.Wrap(err, "failed to set values for verbose logging")
	}
	defer vl.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to test API")
	}
	defer tconn.Close()

	server := httptest.NewServer(http.FileServer(fileSystem))
	defer server.Close()

	conn, err := cr.NewConn(ctx, server.URL+"/"+encodeHTML)
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

	codec := toMIMECodec(testArgs.Codec)
	if codec == "" {
		return errors.Errorf("unknown codec: %s", testArgs.Codec)
	}

	// Decode video frames of crowd720pMP4. The decoded video frames are input of the following encoding.
	config := crowd720pVideoConfig
	if err := conn.Call(ctx, nil, "DecodeFrames", server.URL+"/"+crowd720pMP4, config.numFrames); err != nil {
		return outputJSLogAndError(cleanupCtx, conn, errors.Wrap(err, "failed executing DecodeFrames"))
	}

	bitrate := config.width * config.height * config.framerate / 10
	if err := conn.Call(ctx, nil, "EncodeAndSave", codec, testArgs.Acceleration, config.width, config.height, bitrate, config.framerate); err != nil {
		return outputJSLogAndError(cleanupCtx, conn, errors.Wrap(err, "failed executing EncodeAndSave"))
	}

	var success bool
	if err := conn.Eval(ctx, "TEST.success()", &success); err != nil || !success {
		return outputJSLogAndError(cleanupCtx, conn, errors.New("WebCodecs encoding is not successfully done"))
	}

	// Check if a preferred encoder is used.
	isPlatform, name, err := devtools.GetVideoEncoder(ctx, observer, server.URL+"/"+encodeHTML)
	if err != nil {
		return errors.Wrap(err, "failed getting encoder type")
	}
	if testArgs.Acceleration == PreferHardware && !isPlatform {
		return errors.Errorf("video is encoded by a software encoder, %s", name)
	} else if testArgs.Acceleration == PreferSoftware && isPlatform {
		return errors.Errorf("video is encoded by a hardware encoder, %s", name)
	}

	// We can get the bitstream at once because the expected bitstream size, 0.34MB (= bitrate * config.numFrames / config.framerate),
	// is under the tast websocket limitation, 1MB declared in session.go in package cdputil.
	var bitstreams [][]byte
	if err := conn.Eval(ctx, "bitstreamSaver.getBitstream()", &bitstreams); err != nil {
		return outputJSLogAndError(cleanupCtx, conn, errors.Wrap(err, "error getting bitstream"))
	}

	bitstreamFile, err := SaveBitstream(bitstreams, testArgs.Codec, config.width, config.height, config.framerate, outDir)
	if err != nil {
		return errors.Wrap(err, "failed saving bitstream")
	}
	defer os.Remove(bitstreamFile)

	psnr, ssim, err := computeBitstreamQuality(ctx, videoFile, bitstreamFile, outDir, testArgs.Codec, config.width, config.height)
	if err != nil {
		return errors.Wrap(err, "failed computing bitstream quality")
	}

	// TODO: Have thresholds and fails the test if SSIM or PSNR is lower than them?
	testing.ContextLog(ctx, "PSNR: ", psnr)
	testing.ContextLog(ctx, "SSIM: ", ssim)

	p := perf.NewValues()
	p.Set(perf.Metric{
		Name:      "SSIM",
		Unit:      "percent",
		Direction: perf.BiggerIsBetter,
	}, ssim*100)
	p.Set(perf.Metric{
		Name:      "PSNR",
		Unit:      "dB",
		Direction: perf.BiggerIsBetter,
	}, psnr)
	if err := p.Save(outDir); err != nil {
		return errors.Wrap(err, "failed to save perf results")
	}

	// TODO: Save bitstream always, if SSIM or PSNR is bad or never?
	return nil
}
