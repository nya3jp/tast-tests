// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package encode provides common code to run Chrome binary tests for video encoding.
package encode

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/bintest"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// StreamParams is the parameter for video_encode_accelerator_unittest.
type StreamParams struct {
	// Name is the name of input raw data file.
	Name string
	// Size is the width and height of YUV image in the input raw data.
	Size videotype.Size
	// Bitrate is the requested bitrate in bits per second. VideoEncodeAccelerator is forced to output
	// encoded video in expected range around the bitrate.
	Bitrate int
	// FrameRate is the initial frame rate in the test. This value is optional, and will be set to
	// 30 if unspecified.
	FrameRate int
	// SubseqBitrate is the bitrate to switch to in the middle of the stream in some test cases in
	// video_encode_accelerator_unittest. This value is optional, and will be set to two times of Bitrate if unspecified.
	SubseqBitrate int
	// SubseqFrameRate is the frame rate to switch to in the middle of the stream in some test cases in
	// video_encode_accelerator_unittest. This value is optional, and will be set to 30 if unspecified.
	SubseqFrameRate int
}

// TestOptions is the arguments for RunAccelVideoTest.
type TestOptions struct {
	// Profile is the codec profile to encode.
	Profile videotype.CodecProfile
	// Params is the test parameters for video_encode_accelerator_unittest.
	Params StreamParams
	// PixelFormat is the pixel format of input raw video data.
	PixelFormat videotype.PixelFormat
	// ExtraArgs is the additional arguments to pass video_encode_accelerator_unittest, for example, "--native_input".
	ExtraArgs []string
}

// RunAccelVideoTest runs video_encode_accelerator_unittest.
// It fails if video_encode_accelerator_unittest fails.
func RunAccelVideoTest(ctx context.Context, s *testing.State, opts TestOptions) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging: ", err)
	}
	defer vl.Close()

	// Reserve time to restart the ui job at the end of the test.
	// Only a single process can have access to the GPU, so we are required
	// to call "stop ui" at the start of the test. This will shut down the
	// chrome process and allow us to claim ownership of the GPU.
	shortCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	upstart.StopJob(shortCtx, "ui")
	defer upstart.EnsureJobRunning(ctx, "ui")

	params := opts.Params
	if !strings.HasSuffix(params.Name, ".vp9.webm") {
		s.Fatalf("Source video %v must be VP9 WebM", params.Name)
	}

	streamPath, err := prepareYUV(shortCtx, s.DataPath(params.Name), opts.PixelFormat, params.Size)
	if err != nil {
		s.Fatal("Failed to prepare YUV file: ", err)
	}
	defer os.Remove(streamPath)
	encodeOutFile := strings.TrimSuffix(params.Name, ".vp9.webm")
	if opts.Profile == videotype.H264Prof {
		encodeOutFile += ".h264"
	} else {
		encodeOutFile += ".vp8.ivf"
	}

	outPath := filepath.Join(s.OutDir(), encodeOutFile)
	args := append([]string{logging.ChromeVmoduleFlag(),
		createStreamDataArg(params, opts.Profile, opts.PixelFormat, streamPath, outPath),
		"--ozone-platform=gbm",
	}, opts.ExtraArgs...)
	const exec = "video_encode_accelerator_unittest"
	if ts, err := bintest.Run(shortCtx, exec, args, s.OutDir()); err != nil {
		s.Errorf("Failed to run %v: %v", exec, err)
		for _, t := range ts {
			s.Error(t, " failed")
		}
	}
}

// RunARCVideoTest runs arcvideoencoder_test in ARC.
// It fails if arcvideoencoder_test fails.
func RunARCVideoTest(ctx context.Context, s *testing.State, opts TestOptions) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	// Prepare video stream.
	params := opts.Params
	if !strings.HasSuffix(params.Name, ".vp9.webm") {
		s.Fatalf("Source video %v must be VP9 WebM", params.Name)
	}

	streamPath, err := prepareYUV(ctx, s.DataPath(params.Name), opts.PixelFormat, params.Size)
	if err != nil {
		s.Fatal("Failed to prepare YUV file: ", err)
	}
	defer os.Remove(streamPath)

	// Push video stream file to ARC container.
	const arcTmpPath = "/data/local/tmp"
	arcStreamPath := filepath.Join(arcTmpPath, filepath.Base(streamPath))
	defer a.Command(ctx, "rm", arcStreamPath).Run()
	if err := a.PushFile(ctx, streamPath, arcStreamPath); err != nil {
		s.Fatal("Failed to push video stream to ARC: ", err)
	}

	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	encodeOutFile := strings.TrimSuffix(params.Name, ".vp9.webm") + ".h264"
	outPath := filepath.Join(arcTmpPath, encodeOutFile)
	args := append([]string{
		createStreamDataArg(params, opts.Profile, opts.PixelFormat, arcStreamPath, outPath),
	}, opts.ExtraArgs...)

	// Push test binary files to ARC container. For x86_64 device we might install both amd64 and x86 binaries.
	const binaryDirPath = "/usr/local/libexec/arc-binary-tests"
	var execs []string
	for _, abi := range []string{"amd64", "x86", "arm"} {
		exec := filepath.Join(binaryDirPath, "arcvideoencoder_test_"+abi)
		if _, err := os.Stat(exec); err == nil {
			arcExec := filepath.Join(arcTmpPath, "arcvideoencoder_test_"+abi)
			defer a.Command(ctx, "rm", arcExec).Run()
			if err := a.PushFile(ctx, exec, arcExec); err != nil {
				s.Fatalf("Failed to push test binary %v to ARC", exec)
			}
			execs = append(execs, arcExec)
		}
	}
	if len(execs) == 0 {
		s.Fatal("Test binary is not found in ", binaryDirPath)
	}

	// Execute binary in ARC.
	for _, exec := range execs {
		s.Logf("Running %v %v", exec, strings.Join(args, " "))
		cmd := a.Command(ctx, exec, args...)
		out, err := cmd.Output()
		if err != nil {
			s.Errorf("Failed to run %v: %v", exec, err)
			cmd.DumpLog(ctx)
			continue
		}
		// Because the return value of the adb command is always 0, we cannot use the value to determine whether the test passes.
		// Therefore we parse the output result as alternative.
		if err := ioutil.WriteFile(filepath.Join(s.OutDir(), filepath.Base(exec)+".log"), out, 0644); err != nil {
			s.Error("Failed to write output to file: ", err)
		}
		if strings.Contains(string(out), "FAILED TEST") {
			s.Errorf("Test failed: %s %s", exec, strings.Join(args, " "))
		}
	}
}

// createStreamDataArg creates an argument of video_encode_accelerator_unittest from profile, dataPath and outFile.
func createStreamDataArg(params StreamParams, profile videotype.CodecProfile, pixelFormat videotype.PixelFormat, dataPath, outFile string) string {
	const (
		defaultFrameRate          = 30
		defaultSubseqBitrateRatio = 2
	)

	// Fill default values if they are unsettled.
	if params.FrameRate == 0 {
		params.FrameRate = defaultFrameRate
	}
	if params.SubseqBitrate == 0 {
		params.SubseqBitrate = params.Bitrate * defaultSubseqBitrateRatio
	}
	if params.SubseqFrameRate == 0 {
		params.SubseqFrameRate = defaultFrameRate
	}
	streamDataArgs := fmt.Sprintf("--test_stream_data=%s:%d:%d:%d:%s:%d:%d:%d:%d:%d",
		dataPath, params.Size.W, params.Size.H, int(profile), outFile,
		params.Bitrate, params.FrameRate, params.SubseqBitrate,
		params.SubseqFrameRate, int(pixelFormat))
	return streamDataArgs
}
