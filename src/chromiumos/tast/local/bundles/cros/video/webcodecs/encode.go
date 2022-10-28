// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webcodecs

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/chromeproc"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/devtools"
	"chromiumos/tast/local/media/encoding"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

// TestEncodeArgs is the arguments used in RunEncodeTest.
type TestEncodeArgs struct {
	// Codec is the codec of a bitstream produced by an encoder.
	Codec videotype.Codec
	// ScalabilityMode is a "scalabilityMode" identifier.
	// https://www.w3.org/TR/webrtc-svc/#scalabilitymodes
	ScalabilityMode string
	// BitrateMode is a bitrate mode identifier.
	// https://www.w3.org/TR/mediastream-recording/#bitratemode
	BitrateMode string
	// Acceleration denotes which encoder is used, hardware or software.
	Acceleration HardwareAcceleration
	// OutOfProcessEnabled denotes if it is using out-of-process video
	// encoding
	OutOfProcessEnabled bool
}

const encodeHTML = "webcodecs_encode.html"
const videoEncoderUtilProcName = "media.mojom.VideoEncodeAcceleratorProviderFactory"

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
		yuvFile, err = peelLayersFromYUVFile(ctx, yuvFile, w, h, tid, tids)
		if err != nil {
			return psnr, ssim, errors.Wrap(err, "failed preparing yuv")
		}
		defer os.Remove(yuvFile)

		bitstreamFile, err = saveTemporalLayerBitstream(bitstreams, codec, w, h, framerate, tid, tids)
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
	case videotype.AV1:
		decoder = encoding.LibaomDecoder
	}

	psnr, ssim, err = encoding.CompareFiles(ctx, decoder, yuvFile, bitstreamFile, outDir, coords.NewSize(w, h))
	if err != nil {
		return psnr, ssim, errors.Wrap(err, "failed to decode and compare results")
	}

	return psnr, ssim, nil
}

// verifyTLStruct verifies temporalLayerIDs matches the expected temporal layer structures.
// See https://www.w3.org/TR/webrtc-svc/#dependencydiagrams* for the expected temporal layer structures.
func verifyTLStruct(numTemporalLayers int, temporalLayerIDs []int) error {
	var expectedTLIDs []int
	switch numTemporalLayers {
	case 2:
		expectedTLIDs = []int{0, 1}
	case 3:
		expectedTLIDs = []int{0, 2, 1, 2}
	default:
		return nil
	}

	for i, tid := range temporalLayerIDs {
		expectedTID := expectedTLIDs[i%len(expectedTLIDs)]
		if tid != expectedTID {
			return errors.Errorf("unexpected temporal layer structure: %v", temporalLayerIDs)
		}
	}

	return nil
}

// RunEncodeTest tests encoding in WebCodecs API. It verifies a specified encoder is used and
// the produced bitstream.
func RunEncodeTest(ctx context.Context, cr *chrome.Chrome, fileSystem http.FileSystem, testArgs TestEncodeArgs, videoFile, outDir string) error {
	var crowd720pVideoConfig = videoConfig{width: 1280, height: 720, numFrames: 30, framerate: 30}

	cleanupCtx, server, conn, observer, deferFunc, err := prepareWebCodecsTest(ctx, cr, fileSystem, encodeHTML)
	if err != nil {
		return err
	}
	defer deferFunc()

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
		bitrate, config.framerate, testArgs.ScalabilityMode, testArgs.BitrateMode); err != nil {
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

	tlEncoding := numTemporalLayers > 1
	var temporalLayerIds []int
	if tlEncoding {
		if err := conn.Eval(ctx, "bitstreamSaver.getTemporalLayerIds()", &temporalLayerIds); err != nil {
			return outputJSLogAndError(cleanupCtx, conn, errors.Wrap(err, "error getting temporal layer ids"))
		}

		if len(temporalLayerIds) != config.numFrames {
			return errors.Errorf("temporal layer ids mismatch: expected=%d, actual=%d", config.numFrames, len(temporalLayerIds))
		}

		if err := verifyTLStruct(numTemporalLayers, temporalLayerIds); err != nil {
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
			if tlEncoding {
				return errors.Wrapf(err, "failed computing bitstream quality: tid=%d", tid)
			}
			return errors.Wrap(err, "failed computing bitstream quality")
		}

		psnrStr := "PSNR"
		ssimStr := "SSIM"
		if tlEncoding {
			// +1 because tid is 0-indexed and scalabilityMode identifier
			// (https://www.w3.org/TR/webrtc-svc/#scalabilitymodes) is 1-indexed.
			psnrStr = fmt.Sprintf("%s.L1T%d", psnrStr, tid+1)
			ssimStr = fmt.Sprintf("%s.L1T%d", ssimStr, tid+1)
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

	if testArgs.OutOfProcessEnabled {
		if err := findVideoEncoderUtilityProcess(); err != nil {
			return errors.Wrap(err, "error occured finding the video encoder utility processes")
		}

		if err := multipleEncoders(ctx, cr, fileSystem, testArgs, videoFile, outDir); err != nil {
			return errors.Wrap(err, "error occured testing multiple encoders per utility process")
		}
	}

	// TODO: Save bitstream always, if SSIM or PSNR is bad or never?
	return nil
}

func findVideoEncoderUtilityProcess() error {
	procs, err := chromeproc.GetUtilityProcesses()

	if err != nil {
		return errors.Wrap(err, "failed to FindAll()")
	}

	re := regexp.MustCompile(` --?utility-sub-type=([\w\.]+)(?: |$)`)

	// Store utility process names for generating an error messages
	for _, proc := range procs {
		cmdline, err := proc.Cmdline()
		if err != nil {
			return errors.Wrap(err, "failed to get cmdline")
		}

		matches := re.FindStringSubmatch(cmdline)
		if len(matches) < 2 {
			continue
		}

		procName := matches[1]
		if procName == videoEncoderUtilProcName {
			return nil
		}
	}

	return errors.Errorf("%s process was not found", videoEncoderUtilProcName)
}

func multipleEncoders(ctx context.Context, cr *chrome.Chrome, fileSystem http.FileSystem, testArgs TestEncodeArgs, videoFile, outDir string) error {
	var crowd720pVideoConfig = videoConfig{width: 1280, height: 720, numFrames: 30, framerate: 30}

	cleanupCtx, server, conn, _, deferFunc, err := prepareWebCodecsTest(ctx, cr, fileSystem, encodeHTML)
	if err != nil {
		return err
	}
	defer deferFunc()

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
	if err := conn.Call(ctx, nil, "MultipleEncoders", codec, testArgs.Acceleration, config.width, config.height,
		bitrate, config.framerate, testArgs.ScalabilityMode, testArgs.BitrateMode); err != nil {
		return outputJSLogAndError(cleanupCtx, conn, errors.Wrap(err, "failed executing EncodeAndSave"))
	}

	var success bool
	if err := conn.Eval(ctx, "TEST.success()", &success); err != nil || !success {
		return outputJSLogAndError(cleanupCtx, conn, errors.New("WebCodecs encoding is not successfully done"))
	}

	procs, err := chromeproc.GetUtilityProcesses()

	if err != nil {
		return errors.Wrap(err, "failed to FindAll()")
	}

	re := regexp.MustCompile(` --?utility-sub-type=([\w\.]+)(?: |$)`)
	var numUtilProcs = 0

	// Store utility process names for generating an error messages
	for _, proc := range procs {
		cmdline, err := proc.Cmdline()
		if err != nil {
			return errors.Wrap(err, "failed to get cmdline")
		}

		matches := re.FindStringSubmatch(cmdline)
		if len(matches) < 2 {
			continue
		}

		procName := matches[1]
		if procName == videoEncoderUtilProcName {
			numUtilProcs++
		}
	}

	// Removing the instances created at the beginning of RunEncodeTest
	numUtilProcs -= 2

	// numUtilProcs is divided by two here because the video encoder sandbox
	// opens a broker process with the same name.
	if (numUtilProcs / 2) != 1 {
		return errors.Errorf("expected 1 process but got %d", (numUtilProcs / 2))
	}

	return nil
}

// peelLayersFromYUVFile creates a file that contains YUV frames whose temporal layer id is not more than tid.
// yuvFilePath is the source of YUV frames and tids are the temporal layer ids of them.
// The filepath of the created file is returned. A caller has a responsibility to remove the file.
func peelLayersFromYUVFile(ctx context.Context, yuvFilePath string, w, h, tid int, tids []int) (createdFilePath string, err error) {
	yuvFile, err := os.Open(yuvFilePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to open YUV file")
	}
	defer yuvFile.Close()

	newYUVFilePrefix := fmt.Sprintf("%s.L1T%d", filepath.Base(yuvFilePath), tid+1)
	newYUVFile, err := encoding.CreatePublicTempFile(newYUVFilePrefix)
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

	// This code assumes yuvFile contains YUV 4:2:0.
	planeLen := int(w*h) + int(math.RoundToEven(float64(w))*math.RoundToEven(float64(h))/2.0)
	numYUVFrames := len(tids)
	if stat, err := yuvFile.Stat(); err != nil {
		return "", errors.Wrap(err, "failed to getting a YUV file size")
	} else if stat.Size() != int64(planeLen*numYUVFrames) {
		return "", errors.Errorf("unexpected file size: expected=%d, actual=%d", planeLen*numYUVFrames, stat.Size())
	}

	buf := make([]byte, planeLen)
	for i := 0; i < numYUVFrames; i++ {
		readSize, err := yuvFile.Read(buf)
		if err == io.EOF {
			return "", errors.Errorf("failed to less yuv frames: yuv frames=%d", i)
		}
		if err != nil {
			return "", err
		}
		if readSize != planeLen {
			return "", errors.Errorf("unexpected read size, expected=%d, actual=%d", planeLen, readSize)
		}

		if tids[i] > tid {
			continue
		}

		writeSize, err := newYUVFile.Write(buf)
		if err != nil {
			return "", err
		}
		if writeSize != planeLen {
			return "", errors.Errorf("invalid writing size, got=%d, want=%d", writeSize, planeLen)
		}
	}

	keep = true
	createdFilePath = newYUVFile.Name()
	return
}
