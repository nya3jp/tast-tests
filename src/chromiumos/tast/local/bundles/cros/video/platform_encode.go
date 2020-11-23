// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/encoding"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// testParam is used to describe the config used to run each test.
type testParam struct {
	command  []string      // The command path to be run. This should be relative to /usr/local/bin.
	filename string        // Input file name.
	size     coords.Size   // Width x Height in pixels of the input file.
	timeout  time.Duration // Timeout to run the test.
}

func init() {
	testing.AddTest(&testing.Test{
		Func: PlatformEncode,
		Desc: "Verifies platform encoding",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Attr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Params: []testing.Param{{
			Name: "vp8_qvga",
			Val: testParam{
				command:  []string{"vp8enc"},
				filename: "tulip2-320x180.vp9.webm",
				size:     coords.NewSize(320, 180),
				timeout:  20 * time.Second,
			},
			ExtraData:         []string{"tulip2-320x180.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8, "vaapi"},
		}, {
			Name: "vp8_vga",
			Val: testParam{
				command:  []string{"vp8enc"},
				filename: "tulip2-640x360.vp9.webm",
				size:     coords.NewSize(640, 360),
				timeout:  20 * time.Second,
			},
			ExtraData:         []string{"tulip2-640x360.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8, "vaapi"},
		}, {
			Name: "vp8_hd",
			Val: testParam{
				command:  []string{"vp8enc"},
				filename: "tulip2-1280x720.vp9.webm",
				size:     coords.NewSize(1280, 720),
				timeout:  20 * time.Second,
			},
			ExtraData:         []string{"tulip2-1280x720.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8, "vaapi"},
		}},
		Timeout: 20 * time.Minute,
	})
}

func PlatformEncode(ctx context.Context, s *testing.State) {
	if err := setUp(ctx); err != nil {
		s.Fatal("Failed to set up the DRM test: ", err)
	}
	defer tearDown(ctx)

	testOpt := s.Param().(testParam)

	yuvPath, err := encoding.PrepareYUV(ctx, s.DataPath(testOpt.filename), videotype.I420, coords.NewSize(0, 0) /* placeholder size */)
	if err != nil {
		s.Fatal("Failed to prepare YUV file: ", err)
	}
	defer os.Remove(yuvPath)

	testOpt.command = append(testOpt.command, strconv.Itoa(testOpt.size.Width), strconv.Itoa(testOpt.size.Height))
	testOpt.command = append(testOpt.command, yuvPath)

	// TODO(mcasas): decode the output file and PSNR/SSIM-compare with input.
	testOpt.command = append(testOpt.command, "/tmp/kk.ivf")

	runTest(ctx, s, testOpt.timeout, testOpt.command[0], testOpt.command[1:]...)
}

// setUp prepares the testing environment to run runTest().
func setUp(ctx context.Context) error {
	testing.ContextLog(ctx, "Setting up encoding test")
	return upstart.StopJob(ctx, "ui")
}

// tearDown restores the working environment after runTest().
func tearDown(ctx context.Context) error {
	testing.ContextLog(ctx, "Tearing down encoding test")
	return upstart.EnsureJobRunning(ctx, "ui")
}

// runTest runs the exe binary test. This method may be called several times as long as setUp() has been invoked beforehand.
func runTest(ctx context.Context, s *testing.State, t time.Duration, exe string, args ...string) {
	s.Log("Running ", shutil.EscapeSlice(append([]string{exe}, args...)))

	f, err := os.Create(filepath.Join(s.OutDir(), filepath.Base(exe)+".txt"))
	if err != nil {
		s.Fatal("Failed to create a log file: ", err)
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()
	cmd := testexec.CommandContext(ctx, exe, args...)
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Errorf("Failed to run %s: %v", exe, err)
	}
}
