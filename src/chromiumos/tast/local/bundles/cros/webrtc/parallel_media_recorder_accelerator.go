// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"

	"chromiumos/tast/local/bundles/cros/webrtc/mediarecorder"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ParallelMediaRecorderAccelerator,
		Desc: "Verifies that MediaRecorder can use multiple encoders in parallel",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"loopback_media_recorder.html"},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Params: []testing.Param{{
			Name:              "vp8_vp8",
			Val:               []videotype.Codec{videotype.VP8, videotype.VP8},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name: "vp8_h264",
			Val:  []videotype.Codec{videotype.H264, videotype.VP8},
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{caps.HWEncodeH264, caps.HWEncodeVP8, "chrome_internal"},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name: "h264_h264",
			Val:  []videotype.Codec{videotype.H264, videotype.H264},
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{caps.HWEncodeH264, "chrome_internal"},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}},
	})
}

// ParallelMediaRecorderAccelerator verifies that multiple video encoders can be used
// in parallel.
func ParallelMediaRecorderAccelerator(ctx context.Context, s *testing.State) {
	const (
		// Record for several seconds to ensure the encodings are overlapping.
		// Also, any interaction between the two encoders may take severals
		// seconds to occur.
		recordDuration = 10 * time.Second
	)

	g, ctx := errgroup.WithContext(ctx)
	for _, codec := range s.Param().([]videotype.Codec) {
		c := codec
		g.Go(func() error {
			s.Log("Running codec ", c)
			return mediarecorder.VerifyMediaRecorderUsesEncodeAccelerator(
				ctx, s.PreValue().(*chrome.Chrome), s.DataFileSystem(), c, recordDuration)
		})
	}

	if err := g.Wait(); err != nil {
		s.Error("Failed to run VerifyMediaRecorderUsesEncodeAccelerator: ", err)
	}
}
