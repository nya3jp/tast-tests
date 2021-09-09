// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webcodecs

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/media/devtools"
	"chromiumos/tast/local/media/encoding"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	crowd720p = "crowd-1280x720_30frames.vp9.webm"
)

type videoConfig struct {
	width, height, numFrames, framerate int
}

func getVideoConfig(video string) videoConfig {
	switch video {
	case crowd720p:
		return videoConfig{width: 1280, height: 720, numFrames: 30, framerate: 30}
	default:
		panic(fmt.Sprintf("unknown video: %s", video))
	}
}

type testArgs struct {
	codec        videotype.Codec
	acceleration HardwareAcceleration
	videoFile    string
}

const encodeHTML = "encode.html"
const commonJS = "webcodecs_common.js"

func init() {
	testing.AddTest(&testing.Test{
		Func: Encode,
		Desc: "Verifies that WebCodecs API works, maybe verifying use of a hardware accelerator",
		Contacts: []string{
			"hiroh@chromium.org", // Test author.
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{encodeHTML, commonJS},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Fixture:      "chromeWebCodecs",
		Params: []testing.Param{{
			Name:              "h264_720p_sw",
			Val:               testArgs{codec: videotype.H264, acceleration: preferSoftware, videoFile: crowd720p},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			ExtraData:         []string{crowd720p},
		}, {
			Name:              "h264_720p_hw",
			Val:               testArgs{codec: videotype.H264, acceleration: preferHardware, videoFile: crowd720p},
			ExtraSoftwareDeps: []string{"proprietary_codecs", caps.HWEncodeH264},
			ExtraData:         []string{crowd720p},
		}, {
			Name:      "vp8_720p_sw",
			Val:       testArgs{codec: videotype.VP8, acceleration: preferSoftware, videoFile: crowd720p},
			ExtraData: []string{crowd720p},
		}, {
			Name:              "vp8_720p_hw",
			Val:               testArgs{codec: videotype.VP8, acceleration: preferHardware, videoFile: crowd720p},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         []string{crowd720p},
		}, {
			Name:      "vp9_720p_sw",
			Val:       testArgs{codec: videotype.VP9, acceleration: preferSoftware, videoFile: crowd720p},
			ExtraData: []string{crowd720p},
		}, {
			Name:              "vp9_720p_hw",
			Val:               testArgs{codec: videotype.VP9, acceleration: preferHardware, videoFile: crowd720p},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			ExtraData:         []string{crowd720p},
		}},
	})
}

func Encode(ctx context.Context, s *testing.State) {
	args := s.Param().(testArgs)
	if err := RunEncodeTest(ctx, s.FixtValue().(*chrome.Chrome),
		s.DataFileSystem(), args, s.DataPath(args.videoFile), s.OutDir()); err != nil {
		s.Error("test failed: ", err)
	}
}

func readYUV(yuvFile string, width, height, numFrames int) ([][]byte, error) {
	frameSize := width * height * 3 / 2
	fileInfo, err := os.Stat(yuvFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get file info")
	}

	if fileInfo.Size() != int64(frameSize*numFrames) {
		return nil, errors.Errorf("unexpected file size, expected=%d, actual=%d", frameSize*numFrames, fileInfo.Size())
	}

	buf, err := ioutil.ReadFile(yuvFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file")
	}

	var frames [][]byte
	for i := 0; i < numFrames; i++ {
		frames = append(frames, buf[i*frameSize:(i+1)*frameSize])
	}

	return frames, nil
}

func RunEncodeTest(ctx context.Context, cr *chrome.Chrome, fileSystem http.FileSystem, args testArgs, videoFile, outDir string) error {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		return errors.Wrap(err, "failed to set values for verbose logging")
	}
	defer vl.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	// TODO: Change to NV12.
	yuvFile, err := encoding.PrepareYUV(ctx, videoFile, videotype.I420, coords.NewSize(0, 0) /* placeholder size */)
	if err != nil {
		return errors.Wrap(err, "failed to prepare YUV file")
	}
	defer os.Remove(yuvFile)

	config := getVideoConfig(args.videoFile)

	yuvFrames, err := readYUV(yuvFile, config.width, config.height, config.numFrames)
	if err != nil {
		return errors.Wrap(err, "failed to read YUV frames")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to test API")
	}
	defer tconn.Close()

	if _, err := display.GetInternalInfo(ctx, tconn); err == nil {
		// The device has an internal display.
		// For consistency across test runs, ensure that the device is in landscape-primary orientation.
		if err = graphics.RotateDisplayToLandscapePrimary(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to set display to landscape-primary orientation")
		}
	}

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

	codec := ToMIMECodec(args.codec)
	if codec == "" {
		return errors.Errorf("Unknown codec: %s", args.codec)
	}

	// TODO: Change bitrate per codec.
	bitrate := config.width * config.height * config.framerate / 10
	// Send raw YUVs to JS. Since a YUV is very large, it hits 1MB restriction
	// of the buffer size for websocket connection.
	// TODO: Send frames by cutting less than 1MB?
	if err := conn.Call(ctx, nil, "EncodeAndSave", codec, args.acceleration, config.width, config.height, bitrate, config.framerate, yuvFrames); err != nil {
		return errors.Wrapf(err, "Failed executing EncodeAndSave()")
	}

	if err := conn.WaitForExpr(ctx, "TEST.complete()"); err != nil {
		return errors.Wrap(err, "error completing EncodeAndSave()")
	}

	var success bool
	if err := conn.Eval(ctx, "TEST.success()", &success); err != nil {
		return errors.Wrap(err, "error getting TEST.success")
	}

	if !success {
		var logs string
		if err := conn.Eval(cleanupCtx, "TEST.getLogs()", &logs); err != nil {
			return errors.Wrap(err, "error getting TEST.logs")
		}
		testing.ContextLog(cleanupCtx, "log=", logs)
		return errors.New("TEST failure")
	}

	// Check if a preferred encoder is used.
	isPlatform, name, err := devtools.GetVideoEncoder(ctx, observer, server.URL+"/"+encodeHTML)
	if err != nil {
		return errors.Wrap(err, "failed getting encoder type")
	}
	if args.acceleration == preferHardware && !isPlatform {
		return errors.Errorf("video is encoded by a software encoder, %s", name)
	} else if args.acceleration == preferSoftware && isPlatform {
		return errors.Errorf("video is encoded by a hardware encoder, %s", name)
	}

	var bitstreams [][]byte
	if err := conn.Eval(ctx, "bitstream_saver.getBitstream()", &bitstreams); err != nil {
		return errors.Wrap(err, "error getting bitstream")
	}

	bitstreamFile, err := saveBitstream(bitstreams, args.codec,
		config.width, config.height, config.framerate, outDir)
	if err != nil {
		return errors.Wrap(err, "failed saving bitstream")
	}
	defer os.Remove(bitstreamFile)

	var decoder string
	switch args.codec {
	case videotype.H264:
		decoder = "openh264dec"
	case videotype.VP8:
		decoder = "vpxdec"
	case videotype.VP9:
		decoder = "vpxdec"
	}

	SSIMFile, err := compareFiles(ctx, decoder, yuvFile, bitstreamFile, outDir, coords.NewSize(config.width, config.height))
	if err != nil {
		return errors.Wrap(err, "Failed to decode and compare results")
	}

	// regExpSSIM is the regexp to find the SSIM output in the tiny_ssim log.
	var regExpSSIM = regexp.MustCompile(`\nSSIM: (\d+\.\d+)`)
	// regExpPSNR is the regexp to find the PSNR output in the tiny_ssim log.
	var regExpPSNR = regexp.MustCompile(`\nGlbPSNR: (\d+\.\d+)`)
	SSIM, err := extractValue(SSIMFile, regExpSSIM)
	if err != nil {
		return errors.Wrap(err, "failed to extract SSIM")
	}
	PSNR, err := extractValue(SSIMFile, regExpPSNR)
	if err != nil {
		return errors.Wrap(err, "failed to extract PSNR")
	}

	// TODO: Have thresholds and fails the test if SSIM or PSNR is lower than them?
	testing.ContextLog(ctx, "SSIM: ", SSIM)
	testing.ContextLog(ctx, "PSNR: ", PSNR)

	// TODO: Save bitstream always, if SSIM or PSNR is bad or never?
	return nil
}

func saveBitstream(bitstreams [][]byte, codec videotype.Codec, width, height, framerate int, dir string) (string, error) {
	var filePrefix string
	switch codec {
	case videotype.H264:
		filePrefix = "webcodecs.h264"
	case videotype.VP8:
		filePrefix = "webcodecs.ivf"
	case videotype.VP9:
		filePrefix = "webcodecs.ivf"
	}

	bitstreamFile, err := publicTempFile(filePrefix)
	if err != nil {
		return "", errors.Wrap(err, "Failed creating temporary file")
	}
	defer bitstreamFile.Close()

	// Add Create IVF header
	if codec != videotype.H264 {
		WriteIVFFileHeader(bitstreamFile, codec, width, height, framerate, len(bitstreams))
	}

	for i, b := range bitstreams {
		if codec != videotype.H264 {
			timestamp := uint64(i * 1000 / framerate)
			WriteIVFFrameHeader(bitstreamFile, uint32(len(b)), timestamp)
		}
		if writeSize, err := bitstreamFile.Write(b); err != nil {
			return "", err
		} else if writeSize != len(b) {
			return "", errors.Errorf("invalid writing size, got=%d, want=%d", writeSize, len(b))
		}
	}

	return bitstreamFile.Name(), nil
}

func WriteIVFFrameHeader(bitstreamFile io.Writer, size uint32, timestamp uint64) {
	binary.Write(bitstreamFile, binary.LittleEndian, size)
	binary.Write(bitstreamFile, binary.LittleEndian, timestamp)
}
func WriteIVFFileHeader(bitstreamFile io.Writer, codec videotype.Codec, w, h, framerate, numFrames int) {
	bitstreamFile.Write([]byte{'D', 'K', 'I', 'F'})
	binary.Write(bitstreamFile, binary.LittleEndian, uint16(0))
	binary.Write(bitstreamFile, binary.LittleEndian, uint16(32))
	switch codec {
	case videotype.VP8:
		bitstreamFile.Write([]byte{'V', 'P', '8', '0'})
	case videotype.VP9:
		bitstreamFile.Write([]byte{'V', 'P', '9', '0'})
	default:
		panic("Unknown codec")
	}

	binary.Write(bitstreamFile, binary.LittleEndian, uint16(w))
	binary.Write(bitstreamFile, binary.LittleEndian, uint16(h))
	binary.Write(bitstreamFile, binary.LittleEndian, uint32(framerate))
	binary.Write(bitstreamFile, binary.LittleEndian, uint32(1))
	binary.Write(bitstreamFile, binary.LittleEndian, uint32(numFrames))
	binary.Write(bitstreamFile, binary.LittleEndian, uint32(0))
}

// TODO: This is the duplication code in yuv.go. Shall we make this a common library?
// publicTempFile creates a world-readable temporary file.
func publicTempFile(prefix string) (*os.File, error) {
	f, err := ioutil.TempFile("", prefix)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a public temporary file")
	}
	if err := f.Chmod(0644); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, errors.Wrap(err, "failed to create a public temporary file")
	}
	return f, nil
}

// Copy-paste from video/platform_encoding.go --------------------------------
// TODO: Put these to src/chromiumos/tast/local/media/encoding?
// compareFiles decodes encodedFile using decoder and compares it with yuvFile using tiny_ssim.
func compareFiles(ctx context.Context, decoder, yuvFile, encodedFile, outDir string, size coords.Size) (logFile string, err error) {
	yuvFile2 := yuvFile + ".2"
	tf, err := os.Create(yuvFile2)
	if err != nil {
		return "", errors.Wrap(err, "failed to create a temporary YUV file")
	}
	defer os.Remove(yuvFile2)
	defer tf.Close()

	decodeCommand := []string{encodedFile}
	if decoder == "vpxdec" {
		decodeCommand = append(decodeCommand, "-o")
	}
	decodeCommand = append(decodeCommand, yuvFile2)
	testing.ContextLogf(ctx, "Executing %s %s", decoder, shutil.EscapeSlice(decodeCommand))
	vpxCmd := testexec.CommandContext(ctx, decoder, decodeCommand...)
	if err := vpxCmd.Run(); err != nil {
		vpxCmd.DumpLog(ctx)
		return "", errors.Wrap(err, "decode failed")
	}

	logFile = filepath.Join(outDir, "tiny_ssim.txt")
	f, err := os.Create(logFile)
	if err != nil {
		return "", errors.Wrap(err, "failed to create log file")
	}
	defer f.Close()

	SSIMCmd := testexec.CommandContext(ctx, "tiny_ssim", yuvFile, yuvFile2, strconv.Itoa(size.Width)+"x"+strconv.Itoa(size.Height))
	SSIMCmd.Stdout = f
	SSIMCmd.Stderr = f
	if err := SSIMCmd.Run(testexec.DumpLogOnError); err != nil {
		return "", errors.Wrap(err, "failed to run tiny_ssim")
	}
	return logFile, nil
}

// extractValue parses logFile using r and returns a single float64 match.
func extractValue(logFile string, r *regexp.Regexp) (value float64, err error) {
	b, err := ioutil.ReadFile(logFile)
	if err != nil {
		return 0.0, errors.Wrapf(err, "failed to read file %s", logFile)
	}

	matches := r.FindAllStringSubmatch(string(b), -1)
	if len(matches) != 1 {
		return 0.0, errors.Errorf("found %d matches in %q; want 1", len(matches), b)
	}

	matchString := matches[0][1]
	if value, err = strconv.ParseFloat(matchString, 64); err != nil {
		return 0.0, errors.Wrapf(err, "failed to parse value %q", matchString)
	}
	return
}
