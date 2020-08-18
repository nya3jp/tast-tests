// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/bundles/cros/video/encode"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/encoding"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ParallelEncodeAccel,
		Desc:         "Verifies parallel HW encoding with different codecs",
		Contacts:     []string{"chromeos-video-eng@google.com"},
		Attr:         []string{"group:graphics", "graphics_video"},
		SoftwareDeps: []string{"chrome", caps.HWEncodeVP8, caps.HWEncodeH264},
		Data:         []string{encode.Crowd1080P.Name},
		// Default timeout (i.e. 2 minutes) is not enough.
		Timeout: 5 * time.Minute,
	})
}

func ParallelEncodeAccel(ctx context.Context, s *testing.State) {
	const (
		// Time to run the encoders fo.
		encodeDuration = 30 * time.Second
		// Pixelformat of the video that will be encoded.
		encodePixelFormat = videotype.I420
	)
	// Properties of the video that will be encoded.
	encodeParams := encode.Crowd1080P
	encodeParams.FrameRate = 30

	// Create a raw YUV video to encode for the video encoder tests.
	streamPath, err := encoding.PrepareYUV(ctx, s.DataPath(encodeParams.Name), encodePixelFormat, encodeParams.Size)
	if err != nil {
		s.Fatal("Failed to prepare YUV file: ", err)
	}
	defer os.Remove(streamPath)

	s.Log("Running video_encode_accelerator_unittest")

	errCh := make(chan error)
	for _, codec := range []videotype.CodecProfile{videotype.VP8Prof, videotype.H264Prof} {
		// Create gtest that runs the video encoder test.
		t := gtest.New(
			filepath.Join(chrome.BinTestDir, "video_encode_accelerator_unittest"),
			gtest.Filter("SimpleEncode/*/0"),
			gtest.UID(int(sysutil.ChronosUID)),
			gtest.ExtraArgs(
				encoding.CreateStreamDataArg(encodeParams, codec, encodePixelFormat, streamPath, "/dev/null"),
				"--run_at_fps",
				// Since only a single process can access the GPU at a time, run the
				// test headless so it only accesses the encoder and not the DRM
				// device.
				"--ozone-platform=headless",
				"--single-process-tests",
			))

		// Run the tests in parallel.
		go func() {
			if _, err := t.Run(ctx); err != nil {
				errCh <- err
			}
		}()
	}

	select {
	case err := <-errCh:
		s.Fatal("Error running video_encode_accelerator_unittest: ", err)
	case <-time.After(encodeDuration):
	}
}
