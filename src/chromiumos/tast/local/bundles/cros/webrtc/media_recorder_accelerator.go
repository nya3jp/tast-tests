// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"

	"chromiumos/tast/local/bundles/cros/webrtc/mediarecorder"
	"chromiumos/tast/local/media/caps"
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
		}, {
			Name:              "vp8",
			Val:               videotype.VP8,
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name:              "vp9",
			Val:               videotype.VP9,
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
		}},
	})
}

// MediaRecorderAccelerator verifies that a video encode accelerator was used.
func MediaRecorderAccelerator(ctx context.Context, s *testing.State) {
	mediarecorder.VerifyMediaRecorderUsesEncodeAccelerator(ctx, s,
		s.Param().(videotype.Codec))
}
