// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/encoding"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// regExpFPSVP8 is the regexp to find the FPS output from the VP8 binary log.
var regExpFPSVP8 = regexp.MustCompile(`Processed \d+ frames in \d+ ms \((\d+\.\d+) FPS\)`)

// regExpFPSVP9 is the regexp to find the FPS output from the VP9 binary log.
var regExpFPSVP9 = regexp.MustCompile(`encode \d+ frames in \d+.\d+ secondes, FPS is (\d+\.\d+)`)

// regExpFPSH264 is the regexp to find the FPS output from the H.264 binary log.
var regExpFPSH264 = regexp.MustCompile(`PERFORMANCE:\s+Frame Rate\s+: (\d+.\d+)`)

// regExpFPSV4L2 is the regexp to find the FPS output from v4l2_stateful_encoder.
var regExpFPSV4L2 = regexp.MustCompile(`\((\d+\.\d+)fps\)`)

// regExpFPSVpxenc is the regexp to find the FPS output from vpxenc's log.
var regExpFPSVpxenc = regexp.MustCompile(`\((\d+\.\d+) fps\)`)

// regExpSSIM is the regexp to find the SSIM output in the tiny_ssim log.
var regExpSSIM = regexp.MustCompile(`\nSSIM: (\d+\.\d+)`)

// regExpPSNR is the regexp to find the PSNR output in the tiny_ssim log.
var regExpPSNR = regexp.MustCompile(`\nGlbPSNR: (\d+\.\d+)`)

// commandBuilderFn is the function type to generate the command line with arguments.
type commandBuilderFn func(exe, yuvFile string, size coords.Size, fps int) (command []string, ivfFile string, bitrate int)

// testParam is used to describe the config used to run each test.
type testParam struct {
	command        string           // The command path to be run. This should be relative to /usr/local/bin.
	filename       string           // Input file name. This will be decoded to produce the uncompressed input to the encoder binary, so it can come in any format/container.
	size           coords.Size      // Width x Height in pixels of the input file.
	numFrames      int              // Number of frames of the input file.
	fps            float64          // FPS of the input file.
	commandBuilder commandBuilderFn // Function to create the command line arguments.
	regExpFPS      *regexp.Regexp   // Regexp to find the FPS from output.
	decoder        string           // Command line decoder binary.
}

func init() {
	testing.AddTest(&testing.Test{
		Func: PlatformEncoding,
		Desc: "Verifies platform encoding by using the libva-utils encoder binaries",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Attr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		// Guado, buddy and rikku have a companion video acceleration chip
		// (called Kepler), skip this test in these models.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("guado", "buddy", "rikku")),
		Params: []testing.Param{{
			Name: "vaapi_vp8_180",
			Val: testParam{
				command:        "vp8enc",
				filename:       "tulip2-320x180.vp9.webm",
				numFrames:      500,
				fps:            30,
				size:           coords.NewSize(320, 180),
				commandBuilder: vp8argsVAAPI,
				regExpFPS:      regExpFPSVP8,
				decoder:        "vpxdec",
			},
			ExtraData:         []string{"tulip2-320x180.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeVP8},
		}, {
			Name: "vaapi_vp8_360",
			Val: testParam{
				command:        "vp8enc",
				filename:       "tulip2-640x360.vp9.webm",
				numFrames:      500,
				fps:            30,
				size:           coords.NewSize(640, 360),
				commandBuilder: vp8argsVAAPI,
				regExpFPS:      regExpFPSVP8,
				decoder:        "vpxdec",
			},
			ExtraData:         []string{"tulip2-640x360.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeVP8},
		}, {
			Name: "vaapi_vp8_720",
			Val: testParam{
				command:        "vp8enc",
				filename:       "tulip2-1280x720.vp9.webm",
				numFrames:      500,
				fps:            30,
				size:           coords.NewSize(1280, 720),
				commandBuilder: vp8argsVAAPI,
				regExpFPS:      regExpFPSVP8,
				decoder:        "vpxdec",
			},
			ExtraData:         []string{"tulip2-1280x720.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeVP8},
		}, {
			Name: "vaapi_vp8_180_meet",
			Val: testParam{
				command:        "vp8enc",
				filename:       "gipsrestat-320x180.vp9.webm",
				numFrames:      846,
				fps:            50,
				size:           coords.NewSize(320, 180),
				commandBuilder: vp8argsVAAPI,
				regExpFPS:      regExpFPSVP8,
				decoder:        "vpxdec",
			},
			ExtraData:         []string{"gipsrestat-320x180.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeVP8},
		}, {
			Name: "vaapi_vp8_360_meet",
			Val: testParam{
				command:        "vp8enc",
				filename:       "gipsrestat-640x360.vp9.webm",
				numFrames:      846,
				fps:            50,
				size:           coords.NewSize(640, 360),
				commandBuilder: vp8argsVAAPI,
				regExpFPS:      regExpFPSVP8,
				decoder:        "vpxdec",
			},
			ExtraData:         []string{"gipsrestat-640x360.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeVP8},
		}, {
			Name: "vaapi_vp8_720_meet",
			Val: testParam{
				command:        "vp8enc",
				filename:       "gipsrestat-1280x720.vp9.webm",
				numFrames:      846,
				fps:            50,
				size:           coords.NewSize(1280, 720),
				commandBuilder: vp8argsVAAPI,
				regExpFPS:      regExpFPSVP8,
				decoder:        "vpxdec",
			},
			ExtraData:         []string{"gipsrestat-1280x720.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeVP8},
		}, {
			Name: "vaapi_vp9_180",
			Val: testParam{
				command:        "vp9enc",
				filename:       "tulip2-320x180.vp9.webm",
				numFrames:      500,
				fps:            30,
				size:           coords.NewSize(320, 180),
				commandBuilder: vp9argsVAAPI,
				regExpFPS:      regExpFPSVP9,
				decoder:        "vpxdec",
			},
			ExtraData:         []string{"tulip2-320x180.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeVP9},
		}, {
			Name: "vaapi_vp9_360",
			Val: testParam{
				command:        "vp9enc",
				filename:       "tulip2-640x360.vp9.webm",
				numFrames:      500,
				fps:            30,
				size:           coords.NewSize(640, 360),
				commandBuilder: vp9argsVAAPI,
				regExpFPS:      regExpFPSVP9,
				decoder:        "vpxdec",
			},
			ExtraData:         []string{"tulip2-640x360.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeVP9},
		}, {
			Name: "vaapi_vp9_720",
			Val: testParam{
				command:        "vp9enc",
				filename:       "tulip2-1280x720.vp9.webm",
				numFrames:      500,
				fps:            30,
				size:           coords.NewSize(1280, 720),
				commandBuilder: vp9argsVAAPI,
				regExpFPS:      regExpFPSVP9,
				decoder:        "vpxdec",
			},
			ExtraData:         []string{"tulip2-1280x720.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeVP9},
		}, {
			Name: "vaapi_vp9_180_meet",
			Val: testParam{
				command:        "vp9enc",
				filename:       "gipsrestat-320x180.vp9.webm",
				numFrames:      846,
				fps:            50,
				size:           coords.NewSize(320, 180),
				commandBuilder: vp9argsVAAPI,
				regExpFPS:      regExpFPSVP9,
				decoder:        "vpxdec",
			},
			ExtraData:         []string{"gipsrestat-320x180.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeVP9},
		}, {
			Name: "vaapi_vp9_360_meet",
			Val: testParam{
				command:        "vp9enc",
				filename:       "gipsrestat-640x360.vp9.webm",
				numFrames:      846,
				fps:            50,
				size:           coords.NewSize(640, 360),
				commandBuilder: vp9argsVAAPI,
				regExpFPS:      regExpFPSVP9,
				decoder:        "vpxdec",
			},
			ExtraData:         []string{"gipsrestat-640x360.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeVP9},
		}, {
			Name: "vaapi_vp9_720_meet",
			Val: testParam{
				command:        "vp9enc",
				filename:       "gipsrestat-1280x720.vp9.webm",
				numFrames:      846,
				fps:            50,
				size:           coords.NewSize(1280, 720),
				commandBuilder: vp9argsVAAPI,
				regExpFPS:      regExpFPSVP9,
				decoder:        "vpxdec",
			},
			ExtraData:         []string{"gipsrestat-1280x720.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeVP9},
		}, {
			Name: "vaapi_h264_180",
			Val: testParam{
				command:        "h264encode",
				filename:       "tulip2-320x180.vp9.webm",
				numFrames:      500,
				fps:            30,
				size:           coords.NewSize(320, 180),
				commandBuilder: h264argsVAAPI,
				regExpFPS:      regExpFPSH264,
				decoder:        "openh264dec",
			},
			ExtraData:         []string{"tulip2-320x180.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeH264},
		}, {
			Name: "vaapi_h264_360",
			Val: testParam{
				command:        "h264encode",
				filename:       "tulip2-640x360.vp9.webm",
				numFrames:      500,
				fps:            30,
				size:           coords.NewSize(640, 360),
				commandBuilder: h264argsVAAPI,
				regExpFPS:      regExpFPSH264,
				decoder:        "openh264dec",
			},
			ExtraData:         []string{"tulip2-640x360.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeH264},
		}, {
			Name: "vaapi_h264_720",
			Val: testParam{
				command:        "h264encode",
				filename:       "tulip2-1280x720.vp9.webm",
				numFrames:      500,
				fps:            30,
				size:           coords.NewSize(1280, 720),
				commandBuilder: h264argsVAAPI,
				regExpFPS:      regExpFPSH264,
				decoder:        "openh264dec",
			},
			ExtraData:         []string{"tulip2-1280x720.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeH264},
		}, {
			Name: "vaapi_h264_180_meet",
			Val: testParam{
				command:        "h264encode",
				filename:       "gipsrestat-320x180.vp9.webm",
				numFrames:      846,
				fps:            50,
				size:           coords.NewSize(320, 180),
				commandBuilder: h264argsVAAPI,
				regExpFPS:      regExpFPSH264,
				decoder:        "openh264dec",
			},
			ExtraData:         []string{"gipsrestat-320x180.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeH264},
		}, {
			Name: "vaapi_h264_360_meet",
			Val: testParam{
				command:        "h264encode",
				filename:       "gipsrestat-640x360.vp9.webm",
				numFrames:      846,
				fps:            50,
				size:           coords.NewSize(640, 360),
				commandBuilder: h264argsVAAPI,
				regExpFPS:      regExpFPSH264,
				decoder:        "openh264dec",
			},
			ExtraData:         []string{"gipsrestat-640x360.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeH264},
		}, {
			Name: "vaapi_h264_720_meet",
			Val: testParam{
				command:        "h264encode",
				filename:       "gipsrestat-1280x720.vp9.webm",
				numFrames:      846,
				fps:            50,
				size:           coords.NewSize(1280, 720),
				commandBuilder: h264argsVAAPI,
				regExpFPS:      regExpFPSH264,
				decoder:        "openh264dec",
			},
			ExtraData:         []string{"gipsrestat-1280x720.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeH264},
		}, {
			Name: "vpxenc_vp8_180",
			Val: testParam{
				command:        "vpxenc",
				filename:       "tulip2-320x180.vp9.webm",
				numFrames:      500,
				fps:            30,
				size:           coords.NewSize(320, 180),
				commandBuilder: vp8argsVpxenc,
				regExpFPS:      regExpFPSVpxenc,
				decoder:        "vpxdec",
			},
			ExtraData: []string{"tulip2-320x180.vp9.webm"},
		}, {
			Name: "vpxenc_vp8_360",
			Val: testParam{
				command:        "vpxenc",
				filename:       "tulip2-640x360.vp9.webm",
				numFrames:      500,
				fps:            30,
				size:           coords.NewSize(640, 360),
				commandBuilder: vp8argsVpxenc,
				regExpFPS:      regExpFPSVpxenc,
				decoder:        "vpxdec",
			},
			ExtraData: []string{"tulip2-640x360.vp9.webm"},
		}, {
			Name: "vpxenc_vp8_720",
			Val: testParam{
				command:        "vpxenc",
				filename:       "tulip2-1280x720.vp9.webm",
				numFrames:      500,
				fps:            30,
				size:           coords.NewSize(1280, 720),
				commandBuilder: vp8argsVpxenc,
				regExpFPS:      regExpFPSVpxenc,
				decoder:        "vpxdec",
			},
			ExtraData: []string{"tulip2-1280x720.vp9.webm"},
		}, {
			Name: "vpxenc_vp8_180_meet",
			Val: testParam{
				command:        "vpxenc",
				filename:       "gipsrestat-320x180.vp9.webm",
				numFrames:      846,
				fps:            50,
				size:           coords.NewSize(320, 180),
				commandBuilder: vp8argsVpxenc,
				regExpFPS:      regExpFPSVpxenc,
				decoder:        "vpxdec",
			},
			ExtraData: []string{"gipsrestat-320x180.vp9.webm"},
		}, {
			Name: "vpxenc_vp8_360_meet",
			Val: testParam{
				command:        "vpxenc",
				filename:       "gipsrestat-640x360.vp9.webm",
				numFrames:      846,
				fps:            50,
				size:           coords.NewSize(640, 360),
				commandBuilder: vp8argsVpxenc,
				regExpFPS:      regExpFPSVpxenc,
				decoder:        "vpxdec",
			},
			ExtraData: []string{"gipsrestat-640x360.vp9.webm"},
		}, {
			Name: "vpxenc_vp8_720_meet",
			Val: testParam{
				command:        "vpxenc",
				filename:       "gipsrestat-1280x720.vp9.webm",
				numFrames:      846,
				fps:            50,
				size:           coords.NewSize(1280, 720),
				commandBuilder: vp8argsVpxenc,
				regExpFPS:      regExpFPSVpxenc,
				decoder:        "vpxdec",
			},
			ExtraData: []string{"gipsrestat-1280x720.vp9.webm"},
		}, {
			Name: "v4l2_h264_180",
			Val: testParam{
				command:        "v4l2_stateful_encoder",
				filename:       "tulip2-320x180.vp9.webm",
				numFrames:      500,
				fps:            30,
				size:           coords.NewSize(320, 180),
				commandBuilder: h264argsV4L2,
				regExpFPS:      regExpFPSV4L2,
				decoder:        "openh264dec",
			},
			ExtraData:         []string{"tulip2-320x180.vp9.webm"},
			ExtraSoftwareDeps: []string{"v4l2_codec", caps.HWEncodeH264},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("trogdor")),
		}, {
			Name: "v4l2_h264_360",
			Val: testParam{
				command:        "v4l2_stateful_encoder",
				filename:       "tulip2-640x360.vp9.webm",
				numFrames:      500,
				fps:            30,
				size:           coords.NewSize(640, 360),
				commandBuilder: h264argsV4L2,
				regExpFPS:      regExpFPSV4L2,
				decoder:        "openh264dec",
			},
			ExtraData:         []string{"tulip2-640x360.vp9.webm"},
			ExtraSoftwareDeps: []string{"v4l2_codec", caps.HWEncodeH264},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("trogdor")),
		}, {
			Name: "v4l2_h264_720",
			Val: testParam{
				command:        "v4l2_stateful_encoder",
				filename:       "tulip2-1280x720.vp9.webm",
				numFrames:      500,
				fps:            30,
				size:           coords.NewSize(1280, 720),
				commandBuilder: h264argsV4L2,
				regExpFPS:      regExpFPSV4L2,
				decoder:        "openh264dec",
			},
			ExtraData:         []string{"tulip2-1280x720.vp9.webm"},
			ExtraSoftwareDeps: []string{"v4l2_codec", caps.HWEncodeH264},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("trogdor")),
		}, {
			Name: "v4l2_h264_180_meet",
			Val: testParam{
				command:        "v4l2_stateful_encoder",
				filename:       "gipsrestat-320x180.vp9.webm",
				numFrames:      846,
				fps:            50,
				size:           coords.NewSize(320, 180),
				commandBuilder: h264argsV4L2,
				regExpFPS:      regExpFPSV4L2,
				decoder:        "openh264dec",
			},
			ExtraData:         []string{"gipsrestat-320x180.vp9.webm"},
			ExtraSoftwareDeps: []string{"v4l2_codec", caps.HWEncodeH264},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("trogdor")),
		}, {
			Name: "v4l2_h264_360_meet",
			Val: testParam{
				command:        "v4l2_stateful_encoder",
				filename:       "gipsrestat-640x360.vp9.webm",
				numFrames:      846,
				fps:            50,
				size:           coords.NewSize(640, 360),
				commandBuilder: h264argsV4L2,
				regExpFPS:      regExpFPSV4L2,
				decoder:        "openh264dec",
			},
			ExtraData:         []string{"gipsrestat-640x360.vp9.webm"},
			ExtraSoftwareDeps: []string{"v4l2_codec", caps.HWEncodeH264},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("trogdor")),
		}, {
			Name: "v4l2_h264_720_meet",
			Val: testParam{
				command:        "v4l2_stateful_encoder",
				filename:       "gipsrestat-1280x720.vp9.webm",
				numFrames:      846,
				fps:            50,
				size:           coords.NewSize(1280, 720),
				commandBuilder: h264argsV4L2,
				regExpFPS:      regExpFPSV4L2,
				decoder:        "openh264dec",
			},
			ExtraData:         []string{"gipsrestat-1280x720.vp9.webm"},
			ExtraSoftwareDeps: []string{"v4l2_codec", caps.HWEncodeH264},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("trogdor")),
		}},
		Timeout: 20 * time.Minute,
	})
}

// PlatformEncoding verifies platform encoding by running a command line encoder
// binary and comparing its result with the original input. The encoder input is
// an uncompressed YUV file which would be too large to be stored, so it is
// produced on the fly from testParam.filename. The compressed bitstream output
// is decompressed using testParam.decoder so that it can be compared with the
// original YUV file.
func PlatformEncoding(ctx context.Context, s *testing.State) {
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui job: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	testOpt := s.Param().(testParam)

	yuvFile, err := encoding.PrepareYUV(ctx, s.DataPath(testOpt.filename), videotype.I420, coords.NewSize(0, 0) /* placeholder size */)
	if err != nil {
		s.Fatal("Failed to prepare YUV file: ", err)
	}
	defer os.Remove(yuvFile)

	command, encodedFile, targetBitrate := testOpt.commandBuilder(testOpt.command, yuvFile, testOpt.size, int(testOpt.fps))

	energy, raplErr := power.NewRAPLSnapshot()
	if raplErr != nil || energy == nil {
		s.Log("Energy consumption is not available for this board")
	}
	startTime := time.Now()

	s.Log("Running ", shutil.EscapeSlice(command))
	logFile, err := runTest(ctx, s.OutDir(), command[0], command[1:]...)
	if err != nil {
		s.Fatal("Failed to run binary: ", err)
	}
	defer os.Remove(encodedFile)

	timeDelta := time.Now().Sub(startTime)
	var energyDiff *power.RAPLValues
	var energyErr error
	if raplErr == nil && energy != nil {
		if energyDiff, energyErr = energy.DiffWithCurrentRAPL(); energyErr != nil {
			s.Log("Energy consumption measurement failed: ", energyErr)
		}
	}

	fps, err := extractValue(logFile, testOpt.regExpFPS)
	if err != nil {
		s.Fatal("Failed to extract FPS: ", err)
	}

	SSIMFile, err := compareFiles(ctx, testOpt.decoder, yuvFile, encodedFile, s.OutDir(), testOpt.size)
	if err != nil {
		s.Fatal("Failed to decode and compare results: ", err)
	}
	SSIM, err := extractValue(SSIMFile, regExpSSIM)
	if err != nil {
		s.Fatal("Failed to extract SSIM: ", err)
	}
	PSNR, err := extractValue(SSIMFile, regExpPSNR)
	if err != nil {
		s.Fatal("Failed to extract PSNR: ", err)
	}

	p := perf.NewValues()
	p.Set(perf.Metric{
		Name:      "fps",
		Unit:      "fps",
		Direction: perf.BiggerIsBetter,
	}, fps)
	p.Set(perf.Metric{
		Name:      "SSIM",
		Unit:      "percent",
		Direction: perf.BiggerIsBetter,
	}, SSIM*100)
	p.Set(perf.Metric{
		Name:      "PSNR",
		Unit:      "dB",
		Direction: perf.BiggerIsBetter,
	}, PSNR)

	if energyDiff != nil && energyErr == nil {
		energyDiff.ReportWattPerfMetrics(p, "", timeDelta)
	}

	actualBitrate, err := calculateBitrate(encodedFile, testOpt.fps, testOpt.numFrames)
	if err != nil {
		s.Fatal("Failed to calculate the resulting bitrate: ", err)
	}
	p.Set(perf.Metric{
		Name:      "bitrate_deviation",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, (100.0*actualBitrate/float64(targetBitrate))-100.0)

	s.Log(p)
	if err := p.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf results: ", err)
	}
}

// runTest runs the exe binary test with arguments args.
func runTest(ctx context.Context, outDir, exe string, args ...string) (logFile string, err error) {
	logFile = filepath.Join(outDir, filepath.Base(exe)+".txt")
	f, err := os.Create(logFile)
	if err != nil {
		return "", errors.Wrap(err, "failed to create log file")
	}
	defer f.Close()

	cmd := testexec.CommandContext(ctx, exe, args...)
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return "", errors.Wrapf(err, "failed to run: %s", exe)
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

// calculateBitrate calculates the bitrate of encodedFile.
func calculateBitrate(encodedFile string, fileFPS float64, numFrames int) (value float64, err error) {
	s, err := os.Stat(encodedFile)
	if err != nil {
		return 0.0, errors.Wrapf(err, "failed to get stats for file %s", encodedFile)
	}
	return float64(s.Size()) * 8 /* bits per byte */ * fileFPS / float64(numFrames), nil
}

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

// vp8argsVAAPI constructs the command line for the VP8 encoding binary exe.
func vp8argsVAAPI(exe, yuvFile string, size coords.Size, fps int) (command []string, ivfFile string, bitrate int) {
	command = append(command, exe, strconv.Itoa(size.Width), strconv.Itoa(size.Height), yuvFile)

	ivfFile = yuvFile + ".ivf"
	command = append(command, ivfFile)

	// WebRTC uses Constant BitRate (CBR) with a very large intra-frame
	// period, error resiliency and a certain quality parameter and target
	// bitrate.
	command = append(command, "--intra_period", "3000")
	command = append(command, "--qp", "28" /* Quality Parameter */)
	command = append(command, "--rcmode", "1" /* For Constant BitRate (CBR) */)
	command = append(command, "--error_resilient" /* Off by default, enable. */)

	command = append(command, "-f", strconv.Itoa(fps))

	bitrate = int(0.1 /* BPP */ * float64(fps) * float64(size.Width) * float64(size.Height))
	command = append(command, "--fb", strconv.Itoa(bitrate) /* Kbps */)
	return
}

// vp9argsVAAPI constructs the command line for the VP9 encoding binary exe.
func vp9argsVAAPI(exe, yuvFile string, size coords.Size, fps int) (command []string, ivfFile string, bitrate int) {
	command = append(command, exe, strconv.Itoa(size.Width), strconv.Itoa(size.Height), yuvFile)

	ivfFile = yuvFile + ".ivf"
	command = append(command, ivfFile)

	// WebRTC uses Constant BitRate (CBR) with a very large intra-frame
	// period and a certain quality parameter target, loop filter level
	// and bitrate.
	command = append(command, "--intra_period", "3000")
	command = append(command, "--qp", "24" /* Quality Parameter */)
	command = append(command, "--rcmode", "1" /* For Constant BitRate (CBR) */)
	command = append(command, "--lf_level", "10" /* Loop filter level. */)

	// Intel Gen 11 and later (JSL, TGL, etc) only support Low-Power
	// encoding. Let exe decide which one to use (auto mode).
	command = append(command, "--low_power", "-1")

	command = append(command, "-f", strconv.Itoa(fps))

	// VP9 uses a 30% better bitrate than VP8/H.264, which targets 0.1 bpp.
	bitrate = int(0.07 /* BPP */ * float64(fps) * float64(size.Width) * float64(size.Height))
	command = append(command, "--fb", strconv.Itoa(bitrate/1000.0) /* in Kbps */)
	return
}

// h264argsVAAPI constructs the command line for the H.264 encoding binary exe.
func h264argsVAAPI(exe, yuvFile string, size coords.Size, fps int) (command []string, h264File string, bitrate int) {
	command = append(command, exe, "-w", strconv.Itoa(size.Width), "-h", strconv.Itoa(size.Height))
	command = append(command, "--srcyuv", yuvFile, "--fourcc", "YV12")
	command = append(command, "-n", "0" /* Read number of frames from yuvFile*/)

	h264File = yuvFile + ".h264"
	command = append(command, "-o", h264File)

	// WebRTC uses Constant BitRate (CBR) with a very large intra-frame
	// period and a certain quality parameter target, bitrate and profile.
	command = append(command, "--intra_period", "2048", "--idr_period", "2048", "--ip_period", "1")
	command = append(command, "--minqp", "24", "--initialqp", "26" /* Quality Parameter */)
	command = append(command, "--profile", "BP" /* (Constrained) Base Profile. */)

	command = append(command, "-f", strconv.Itoa(fps))

	command = append(command, "--rcmode", "CBR" /* Constant BitRate */)
	bitrate = int(0.1 /* BPP */ * float64(fps) * float64(size.Width) * float64(size.Height))
	command = append(command, "--bitrate", strconv.Itoa(bitrate) /* bps */)
	return
}

// vp8argsVpxenc constructs the command line for vpxenc.
func vp8argsVpxenc(exe, yuvFile string, size coords.Size, fps int) (command []string, ivfFile string, bitrate int) {
	command = append(command, exe, "-w", strconv.Itoa(size.Width), "-h", strconv.Itoa(size.Height))

	command = append(command, "--passes=1" /* 1 encoding pass */)
	command = append(command, "--codec=vp8")

	// WebRTC uses Constant BitRate (CBR) with a very large intra-frame period and
	// a certain quality parameter and target bitrate. See WebRTC libvpx wrapper
	// (LibvpxVP8Encoder at the time of writing) and also
	// https://www.webmproject.org/docs/encoder-parameters/#3-rate-control.
	command = append(command, "--kf-min-dist=0", "--kf-max-dist=3000")
	command = append(command, "--min-q=2", "--max-q=63" /* Quality Parameter */)
	command = append(command, "--end-usage=cbr" /* Constant BitRate */)
	command = append(command, "--error-resilient=0" /* Off. */)
	command = append(command, "--undershoot-pct=100", "--overshoot-pct=15")
	command = append(command, "--buf-sz=1000", "--buf-initial-sz=500", "--buf-optimal-sz=600")
	command = append(command, "--cpu-used=-6")
	if size.Width*size.Height > 640*480 {
		// WebRTC uses 2 threads for resolutions above VGA if the CPU has 3 or more
		// cores. All ChromeOS devices should comply.
		command = append(command, "--threads=2")
	}

	bitrate = int(0.1 /* BPP */ * float64(fps) * float64(size.Width) * float64(size.Height))
	command = append(command, fmt.Sprintf("--target-bitrate=%d", bitrate/1000) /* Kbps */)

	command = append(command, fmt.Sprintf("--fps=%d/1", fps))

	ivfFile = yuvFile + ".ivf"
	command = append(command, "--ivf", "-o", ivfFile)

	// Source file goes at the end without any flag.
	command = append(command, yuvFile)
	return
}

// h264argsV4L2 constructs the command line for the v4l2_stateful_encoder and for H.264.
func h264argsV4L2(exe, yuvFile string, size coords.Size, fps int) (command []string, h264File string, bitrate int) {
	command = append(command, exe, "--width", strconv.Itoa(size.Width), "--height", strconv.Itoa(size.Height))
	command = append(command, "--file", yuvFile, "--file_format", "yv12")

	command = append(command, "--fps", strconv.Itoa(fps))

	command = append(command, "--codec", "H264")

	// The output file automatically gets a .h264 suffix added.
	command = append(command, "--output", yuvFile)
	h264File = yuvFile + ".h264"

	// WebRTC uses Constant BitRate (CBR) with a very large intra-frame
	// period and a certain quality parameter target, bitrate and profile.
	command = append(command, "--gop", "65535")
	command = append(command, "--end_usage", "CBR" /* Constant BitRate */)

	bitrate = int(0.1 /* BPP */ * float64(fps) * float64(size.Width) * float64(size.Height))
	command = append(command, "--bitrate", strconv.Itoa(bitrate) /* bps */)
	return
}
