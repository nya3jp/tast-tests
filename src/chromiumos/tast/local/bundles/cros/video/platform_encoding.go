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
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// regExpFPS is the regexp to find the FPS output from the binary log.
var regExpFPS = regexp.MustCompile(`Processed \d+ frames in \d+ ms \((\d+\.\d+) FPS\)`)

// testParam is used to describe the config used to run each test.
type testParam struct {
	command  []string    // The command path to be run. This should be relative to /usr/local/bin.
	filename string      // Input file name.
	size     coords.Size // Width x Height in pixels of the input file.
}

func init() {
	testing.AddTest(&testing.Test{
		Func: PlatformEncoding,
		Desc: "Verifies platform encoding by using the libva-utils encoder binaries",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		SoftwareDeps: []string{"vaapi"},
		Params: []testing.Param{{
			Name: "vp8_180",
			Val: testParam{
				command:  []string{"vp8enc"},
				filename: "tulip2-320x180.vp9.webm",
				size:     coords.NewSize(320, 180),
			},
			ExtraData:         []string{"tulip2-320x180.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name: "vp8_360",
			Val: testParam{
				command:  []string{"vp8enc"},
				filename: "tulip2-640x360.vp9.webm",
				size:     coords.NewSize(640, 360),
			},
			ExtraData:         []string{"tulip2-640x360.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name: "vp8_720",
			Val: testParam{
				command:  []string{"vp8enc"},
				filename: "tulip2-1280x720.vp9.webm",
				size:     coords.NewSize(1280, 720),
			},
			ExtraData:         []string{"tulip2-1280x720.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
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

	testOpt.command = append(testOpt.command, strconv.Itoa(testOpt.size.Width), strconv.Itoa(testOpt.size.Height), yuvFile)

	// TODO(mcasas): decode the output file and PSNR/SSIM-compare with input.
	ivfFile := yuvFile + ".ivf"
	testOpt.command = append(testOpt.command, ivfFile)

	// WebRTC uses Constant BitRate (CBR) with a very large intra-frame
	// period, error resiliency and a certain quality parameter and target
	// bitrate.
	testOpt.command = append(testOpt.command, "--intra_period", "3000")
	testOpt.command = append(testOpt.command, "--qp", "28" /* Quality Parameter */)
	testOpt.command = append(testOpt.command, "--rcmode", "1" /* For Constant BitRate (CBR) */)
	testOpt.command = append(testOpt.command, "--error_resilient" /* Off by default, enable. */)

	bitrate := 256 * testOpt.size.Width * testOpt.size.Height / (320.0 * 240.0)
	testOpt.command = append(testOpt.command, "--fb", strconv.Itoa(bitrate) /* From Chromecast */)

	s.Log("Running ", shutil.EscapeSlice(testOpt.command))
	logFile, err := runTest(ctx, s.OutDir(), testOpt.command[0], testOpt.command[1:]...)
	if err != nil {
		s.Fatal("Failed to run binary: ", err)
	}
	defer os.Remove(ivfFile)

	fps, err := extractFPS(logFile)
	if err != nil {
		s.Fatal("extractFPS() failed: ", err)
	}
	p := perf.NewValues()
	p.Set(perf.Metric{
		Name:      "fps",
		Unit:      "fps",
		Direction: perf.BiggerIsBetter,
	}, fps)
	s.Log(p)
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

// extractFPS parses logFile and extracts the average frame rate.
func extractFPS(logFile string) (fps float64, err error) {
	b, err := ioutil.ReadFile(logFile)
	if err != nil {
		return 0.0, errors.Wrapf(err, "failed to read file %s", logFile)
	}

	matches := regExpFPS.FindAllStringSubmatch(string(b), -1)
	if len(matches) != 1 {
		return 0.0, errors.Errorf("found %d FPS matches in %q; want 1", len(matches), b)
	}

	fpsString := matches[0][1]
	fps, err = strconv.ParseFloat(fpsString, 64)
	if err != nil {
		return 0.0, errors.Wrapf(err, "failed to parse FPS value %q", fpsString)
	}
	return fps, nil
}
