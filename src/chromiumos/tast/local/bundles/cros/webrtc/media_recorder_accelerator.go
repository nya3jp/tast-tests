// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/webrtc/mediarecorder"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MediaRecorderAccelerator,
		Desc: "Verifies that MediaRecorder uses video encode acceleration",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"loopback_media_recorder.html"},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Params: []testing.Param{{
			Name: "h264",
			Val:  videotype.H264,
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{caps.HWEncodeH264, "chrome_internal"},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name:              "vp8",
			Val:               videotype.VP8,
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name: "vp9",
			Val:  videotype.VP9,
			// TODO(crbug.com/811912): Remove "vaapi" and pre.ChromeVideoWithFakeWebcamAndVP9VaapiEncoder()
			// once the feature is enabled by default on VA-API devices.
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9, "vaapi"},
			Pre:               pre.ChromeVideoWithFakeWebcamAndVP9VaapiEncoder(),
		}, {
			Name:              "vp8_cam",
			Val:               videotype.VP8,
			ExtraSoftwareDeps: []string{caps.BuiltinCamera, caps.HWEncodeVP8},
			Pre:               pre.ChromeCameraPerf(),
		}},
	})
}

// MediaRecorderAccelerator verifies that a video encode accelerator was used.
func MediaRecorderAccelerator(ctx context.Context, s *testing.State) {
	const (
		// Let the MediaRecorder accumulate a few milliseconds, otherwise we might
		// receive just bits and pieces of the container header.
		recordDuration = 100 * time.Millisecond
	)

	if err := mediarecorder.VerifyMediaRecorderUsesEncodeAccelerator(ctx, s.PreValue().(*chrome.Chrome), s.DataFileSystem(), s.Param().(videotype.Codec), recordDuration); err != nil {
		s.Error("Failed to run VerifyMediaRecorderUsesEncodeAccelerator: ", err)
	}
}
