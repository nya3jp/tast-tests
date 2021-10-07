// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package webcodecs provides common code for video.WebCodecs* tests
package webcodecs

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	// ScalabilityMode is a "scalabilityMode" identifier.
	// https://www.w3.org/TR/webrtc-svc/#scalabilitymodes
	ScalabilityMode string
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

// computeBitstreamQuality computes SSIM and PSNR of bitstreams comparing with yuvFile.
// If numTemporalLayers is more than 1, then this computes SSIM and PSNR of bitstreams
// whose represented frames are in temporal layers up to tid.
func computeBitstreamQuality(ctx context.Context, yuvFile, outDir string, bitstreams [][]byte,
	codec videotype.Codec, w, h, framerate, tid, numTemporalLayers int, tids []int) (psnr, ssim float64, err error) {
	var bitstreamFile string
	if tid == numTemporalLayers-1 {
		bitstreamFile, err = saveBitstream(bitstreams, codec, w, h, framerate)
		if err != nil {
			return psnr, ssim, errors.Wrap(err, "failed preparing bitstream")
		}
		defer os.Remove(bitstreamFile)
	} else {
		yuvFile, err = prepareYUVFileWithTL(ctx, yuvFile, w, h, tid, tids)
		if err != nil {
			return psnr, ssim, errors.Wrap(err, "failed preparing yuv")
		}
		defer os.Remove(yuvFile)

		bitstreamFile, err = saveBitstreamWithTL(bitstreams, codec, w, h, framerate, tid, tids)
		if err != nil {
			return psnr, ssim, errors.Wrap(err, "failed preparing bitstream")
		}
		defer os.Remove(bitstreamFile)
	}

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
	testing.ContextLog(ctx, "JS log=", logs)
	return callErr
}

// verifyTLStruct verifies the number of frames in each temporal layer structure matches
// the ones in an expected temporal layer structure. See https://www.w3.org/TR/webrtc-svc/#dependencydiagrams*
// for the expected temporal layer structures.
func verifyTLStruct(numTemporalLayers, numFrames int, numFramesInTL []int) error {
	expectedNumFramesInTL := make([]int, len(numFramesInTL))
	switch numTemporalLayers {
	case 2:
		expectedNumFramesInTL[0] = (numFrames + 1) / 2
		expectedNumFramesInTL[1] = numFrames / 2
	case 3:
		expectedNumFramesInTL[0] = numFrames / 4
		expectedNumFramesInTL[1] = numFrames / 4
		expectedNumFramesInTL[2] = numFrames / 4 * 2
		if numFrames%4 >= 1 {
			expectedNumFramesInTL[0]++
		}
		if numFrames%4 >= 2 {
			expectedNumFramesInTL[2]++
		}
		if numFrames%4 >= 3 {
			expectedNumFramesInTL[1]++
		}
	default:
		return nil
	}

	for i := 0; i < len(numFramesInTL); i++ {
		if expectedNumFramesInTL[i] != numFramesInTL[i] {
			return errors.Errorf("unexpected temporal layer structure: expected numFramesInTL=%v, actual numFramesInTL=%v", expectedNumFramesInTL, numFramesInTL)
		}
	}

	return nil
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
	if err := conn.Call(ctx, nil, "EncodeAndSave", codec, testArgs.Acceleration, config.width, config.height,
		bitrate, config.framerate, testArgs.ScalabilityMode); err != nil {
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

	var numTemporalLayers int
	switch testArgs.ScalabilityMode {
	case "":
		numTemporalLayers = 1
	case "L1T2":
		numTemporalLayers = 2
	case "L1T3":
		numTemporalLayers = 3
	default:
		return errors.Errorf("unknown scalabilityMode: %s", testArgs.ScalabilityMode)
	}

	isTLEncoding := numTemporalLayers > 1
	var temporalLayerIds []int
	if isTLEncoding {
		if err := conn.Eval(ctx, "bitstreamSaver.getTemporalLayerIds()", &temporalLayerIds); err != nil {
			return outputJSLogAndError(cleanupCtx, conn, errors.Wrap(err, "error getting temporal layer ids"))
		}

		if len(temporalLayerIds) != config.numFrames {
			return errors.Errorf("temporal layer ids mismatch: expected=%d, actual=%d", config.numFrames, len(temporalLayerIds))
		}

		numFramesInTL := make([]int, numTemporalLayers)
		for _, tid := range temporalLayerIds {
			if tid >= numTemporalLayers || tid < 0 {
				return errors.Errorf("invalid temporal layer id: %d", tid)
			}
			numFramesInTL[tid]++
		}
		for tid, frames := range numFramesInTL {
			if frames == 0 {
				return errors.Errorf("no frame with tid=%d", tid)
			}
		}
		if err := verifyTLStruct(numTemporalLayers, config.numFrames, numFramesInTL); err != nil {
			return err
		}
	}

	yuvFile, err := encoding.PrepareYUV(ctx, videoFile, videotype.I420, coords.NewSize(0, 0) /* placeholder size */)
	if err != nil {
		return errors.Wrap(err, "failed to prepare YUV file")
	}
	defer os.Remove(yuvFile)

	p := perf.NewValues()
	for tid := 0; tid < numTemporalLayers; tid++ {
		psnr, ssim, err := computeBitstreamQuality(ctx, yuvFile, outDir, bitstreams,
			testArgs.Codec, config.width, config.height, config.framerate,
			tid, numTemporalLayers, temporalLayerIds)
		if err != nil {
			if isTLEncoding {
				return errors.Wrapf(err, "failed computing bitstream quality: tid=%d", tid)
			}
			return errors.Wrap(err, "failed computing bitstream quality")
		}

		psnrStr := "PSNR"
		ssimStr := "SSIM"
		if isTLEncoding {
			psnrStr += "." + fmt.Sprintf("L1T%d", tid+1)
			ssimStr += "." + fmt.Sprintf("L1T%d", tid+1)
		}
		testing.ContextLogf(ctx, "%s: %f", psnrStr, psnr)
		testing.ContextLogf(ctx, "%s: %f", ssimStr, ssim)
		p.Set(perf.Metric{
			Name:      ssimStr,
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}, ssim*100)
		p.Set(perf.Metric{
			Name:      psnrStr,
			Unit:      "dB",
			Direction: perf.BiggerIsBetter,
		}, psnr)
	}

	if err := p.Save(outDir); err != nil {
		return errors.Wrap(err, "failed to save perf results")
	}

	// TODO: Save bitstream always, if SSIM or PSNR is bad or never?
	return nil
}

// prepareYUVFileWithTL creates a file that contains YUV frames whose temporal layer id is not more than tid.
// yuvFilePath is the source of YUV frames and tids are the temporal layer ids of them.
// The filepath of the created file is returned.
func prepareYUVFileWithTL(ctx context.Context, yuvFilePath string, w, h, tid int, tids []int) (string, error) {
	yuvFile, err := os.Open(yuvFilePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to open YUV file")
	}
	defer yuvFile.Close()
	if _, err := yuvFile.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	newYUVFile, err := encoding.CreatePublicTempFile(filepath.Base(yuvFilePath) + fmt.Sprintf("L1T%d", tid+1))
	if err != nil {
		return "", errors.Wrap(err, "failed to create a temporary YUV file")
	}
	keep := false
	defer func() {
		newYUVFile.Close()
		if !keep {
			os.Remove(newYUVFile.Name())
		}
	}()

	// This code assumes yuvFile contains YUV 4:2:0
	planeLen := int(w*h) + int(math.RoundToEven(float64(w))*math.RoundToEven(float64(h))/2.0)
	numYUVFrames := len(tids)
	if stat, err := yuvFile.Stat(); err != nil {
		return "", errors.Wrap(err, "failed to getting a YUV file size")
	} else if stat.Size() != int64(planeLen*numYUVFrames) {
		return "", errors.Errorf("unexpected file size: expected=%d, actual=%d", planeLen*numYUVFrames, stat.Size())
	}

	endOfFile := false
	buf := make([]byte, planeLen)
	for i := 0; i < numYUVFrames && !endOfFile; i++ {
		readSize, err := yuvFile.Read(buf)
		if err == io.EOF {
			endOfFile = true
		} else if err != nil {
			return "", err
		} else if readSize != 0 && readSize != planeLen {
			return "", errors.Errorf("unexpected read size, expected=%d, actual=%d", planeLen, readSize)
		}

		if tids[i] > tid {
			continue
		}

		writeSize, err := newYUVFile.Write(buf)
		if err != nil {
			return "", err
		} else if writeSize != planeLen {
			return "", errors.Errorf("invalid writing size, got=%d, want=%d",
				writeSize, planeLen)
		}
	}

	keep = true
	return newYUVFile.Name(), nil
}
