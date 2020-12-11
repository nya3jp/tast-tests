// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
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

// regExpSSIM is the regexp to find the SSIM output in the tiny_ssim log.
var regExpSSIM = regexp.MustCompile(`\nSSIM: (\d+\.\d+)`)

// regExpPSNR is the regexp to find the PSNR output in the tiny_ssim log.
var regExpPSNR = regexp.MustCompile(`\nGlbPSNR: (\d+\.\d+)`)

// commandBuilderFn is the function type to generate the command line with arguments.
type commandBuilderFn func(exe, yuvFile string, size coords.Size) (command []string, ivfFile string)

// testParam is used to describe the config used to run each test.
type testParam struct {
	command        string           // The command path to be run. This should be relative to /usr/local/bin.
	filename       string           // Input file name.
	size           coords.Size      // Width x Height in pixels of the input file.
	commandBuilder commandBuilderFn // Function to create the command line arguments.
	regExpFPS      *regexp.Regexp   // Regexp to find the FPS from output.
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
		SoftwareDeps: []string{"vaapi"},
		Params: []testing.Param{{
			Name: "vp8_180",
			Val: testParam{
				command:        "vp8enc",
				filename:       "tulip2-320x180.vp9.webm",
				size:           coords.NewSize(320, 180),
				commandBuilder: vp8args,
				regExpFPS:      regExpFPSVP8,
			},
			ExtraData:         []string{"tulip2-320x180.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name: "vp8_360",
			Val: testParam{
				command:        "vp8enc",
				filename:       "tulip2-640x360.vp9.webm",
				size:           coords.NewSize(640, 360),
				commandBuilder: vp8args,
				regExpFPS:      regExpFPSVP8,
			},
			ExtraData:         []string{"tulip2-640x360.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name: "vp8_720",
			Val: testParam{
				command:        "vp8enc",
				filename:       "tulip2-1280x720.vp9.webm",
				size:           coords.NewSize(1280, 720),
				commandBuilder: vp8args,
				regExpFPS:      regExpFPSVP8,
			},
			ExtraData:         []string{"tulip2-1280x720.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name: "vp9_180",
			Val: testParam{
				command:        "vp9enc",
				filename:       "tulip2-320x180.vp9.webm",
				size:           coords.NewSize(320, 180),
				commandBuilder: vp9args,
				regExpFPS:      regExpFPSVP9,
			},
			ExtraData:         []string{"tulip2-320x180.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
		}, {
			Name: "vp9_360",
			Val: testParam{
				command:        "vp9enc",
				filename:       "tulip2-640x360.vp9.webm",
				size:           coords.NewSize(640, 360),
				commandBuilder: vp9args,
				regExpFPS:      regExpFPSVP9,
			},
			ExtraData:         []string{"tulip2-640x360.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
		}, {
			Name: "vp9_720",
			Val: testParam{
				command:        "vp9enc",
				filename:       "tulip2-1280x720.vp9.webm",
				size:           coords.NewSize(1280, 720),
				commandBuilder: vp9args,
				regExpFPS:      regExpFPSVP9,
			},
			ExtraData:         []string{"tulip2-1280x720.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
		}},
		Timeout: 20 * time.Minute,
	})
}

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

	command, ivfFile := testOpt.commandBuilder(testOpt.command, yuvFile, testOpt.size)

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
	defer os.Remove(ivfFile)

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

	SSIMFile, err := compareFiles(ctx, yuvFile, ivfFile, s.OutDir(), testOpt.size)
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

// compareFiles decodes ivfFile using vpxdec and compares it with yuvFile using tiny_ssim.
func compareFiles(ctx context.Context, yuvFile, ivfFile, outDir string, size coords.Size) (logFile string, err error) {
	yuvFile2 := yuvFile + ".2"
	tf, err := os.Create(yuvFile2)
	if err != nil {
		return "", errors.Wrap(err, "failed to create a temporary YUV file")
	}
	defer os.Remove(yuvFile2)
	defer tf.Close()

	testing.ContextLogf(ctx, "Executing vpxdec %s", filepath.Base(ivfFile))
	vpxCmd := testexec.CommandContext(ctx, "vpxdec", ivfFile, "-o", yuvFile2)
	if err := vpxCmd.Run(); err != nil {
		vpxCmd.DumpLog(ctx)
		return "", errors.Wrap(err, "vpxdec failed")
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

// vp8args constructs the command line for the VP8 encoding binary exe.
func vp8args(exe, yuvFile string, size coords.Size) (command []string, ivfFile string) {
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

	bitrate := 256 * size.Width * size.Height / (320.0 * 240.0)
	command = append(command, "--fb", strconv.Itoa(bitrate) /* From Chromecast */)
	return
}

// vp9args constructs the command line for the VP9 encoding binary exe.
func vp9args(exe, yuvFile string, size coords.Size) (command []string, ivfFile string) {
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
	command = append(command, "--low_power", "0" /* Prefer non Low-Power mode */)

	bitrate := int(1.3 * float64(size.Width) * float64(size.Height) / 1000.0)
	command = append(command, "--fb", strconv.Itoa(bitrate))
	return
}
