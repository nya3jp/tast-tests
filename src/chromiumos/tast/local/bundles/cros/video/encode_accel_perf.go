// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/encode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/encoding"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EncodeAccelPerf,
		Desc:         "Measures hardware video encode performance by running the video_encode_accelerator_unittest binary",
		Contacts:     []string{"johnylin@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		// Default timeout (i.e. 2 minutes) is not enough.
		Timeout: 10 * time.Minute,
		Params: []testing.Param{{
			Name: "h264_192p_i420",
			Val: encoding.TestOptions{
				Profile:     videotype.H264Prof,
				Params:      encoding.Bear192P,
				PixelFormat: videotype.I420,
				InputMode:   encoding.SharedMemory,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         []string{encoding.Bear192P.Name},
		}, {
			Name: "h264_360p_i420",
			Val: encoding.TestOptions{
				Profile:     videotype.H264Prof,
				Params:      encoding.Tulip360P,
				PixelFormat: videotype.I420,
				InputMode:   encoding.SharedMemory,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         []string{encoding.Tulip360P.Name},
		}, {
			Name: "h264_720p_i420_tulip",
			Val: encoding.TestOptions{
				Profile:     videotype.H264Prof,
				Params:      encoding.Tulip720P,
				PixelFormat: videotype.I420,
				InputMode:   encoding.SharedMemory,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         []string{encoding.Tulip720P.Name},
		}, {
			Name: "h264_720p_i420_vidyo",
			Val: encoding.TestOptions{
				Profile:     videotype.H264Prof,
				Params:      encoding.Vidyo720P,
				PixelFormat: videotype.I420,
				InputMode:   encoding.SharedMemory,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         []string{encoding.Vidyo720P.Name},
		}, {
			Name: "h264_1080p_i420",
			Val: encoding.TestOptions{
				Profile:     videotype.H264Prof,
				Params:      encoding.Crowd1080P,
				PixelFormat: videotype.I420,
				InputMode:   encoding.SharedMemory,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         []string{encoding.Crowd1080P.Name},
		}, {
			Name: "h264_2160p_i420",
			Val: encoding.TestOptions{
				Profile:     videotype.H264Prof,
				Params:      encoding.Crowd2160P,
				PixelFormat: videotype.I420,
				InputMode:   encoding.SharedMemory,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264_4K},
			ExtraData:         []string{encoding.Crowd2160P.Name},
		}, {
			Name: "vp8_192p_i420",
			Val: encoding.TestOptions{
				Profile:     videotype.VP8Prof,
				Params:      encoding.Bear192P,
				PixelFormat: videotype.I420,
				InputMode:   encoding.SharedMemory,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         []string{encoding.Bear192P.Name},
		}, {
			Name: "vp8_360p_i420",
			Val: encoding.TestOptions{
				Profile:     videotype.VP8Prof,
				Params:      encoding.Tulip360P,
				PixelFormat: videotype.I420,
				InputMode:   encoding.SharedMemory,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         []string{encoding.Tulip360P.Name},
		}, {
			Name: "vp8_720p_i420_tulip",
			Val: encoding.TestOptions{
				Profile:     videotype.VP8Prof,
				Params:      encoding.Tulip720P,
				PixelFormat: videotype.I420,
				InputMode:   encoding.SharedMemory,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         []string{encoding.Tulip720P.Name},
		}, {
			Name: "vp8_720p_i420_vidyo",
			Val: encoding.TestOptions{
				Profile:     videotype.VP8Prof,
				Params:      encoding.Vidyo720P,
				PixelFormat: videotype.I420,
				InputMode:   encoding.SharedMemory,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         []string{encoding.Vidyo720P.Name},
		}, {
			Name: "vp8_1080p_i420",
			Val: encoding.TestOptions{
				Profile:     videotype.VP8Prof,
				Params:      encoding.Crowd1080P,
				PixelFormat: videotype.I420,
				InputMode:   encoding.SharedMemory,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         []string{encoding.Crowd1080P.Name},
		}, {
			Name: "vp8_2160p_i420",
			Val: encoding.TestOptions{
				Profile:     videotype.VP8Prof,
				Params:      encoding.Crowd2160P,
				PixelFormat: videotype.I420,
				InputMode:   encoding.SharedMemory,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8_4K},
			ExtraData:         []string{encoding.Crowd2160P.Name},
		}},
	})
}

func EncodeAccelPerf(ctx context.Context, s *testing.State) {
	encode.RunAccelVideoPerfTest(ctx, s, s.Param().(encoding.TestOptions))
}
