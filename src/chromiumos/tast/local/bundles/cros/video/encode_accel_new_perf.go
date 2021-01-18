// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/encode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

const (
	crowd180p  = "crowd-320x180_30frames.vp9.webm"
	crowd360p  = "crowd-640x360_30frames.vp9.webm"
	crowd720p  = "crowd-1280x720_30frames.vp9.webm"
	crowd1080p = "crowd-1920x1080_30frames.vp9.webm"
	crowd2160p = "crowd-3840x2160_30frames.vp9.webm"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EncodeAccelNewPerf,
		Desc:         "Measures hardware video encode performance by running the video_encode_accelerator_perf_tests binary",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-gfx-video@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		// Default timeout (i.e. 2 minutes) is not enough.
		Timeout: 10 * time.Minute,
		Params: []testing.Param{{
			Name: "h264_192p",
			Val: encode.TestOptionsNew{
				WebMName: crowd180p,
				Profile:  videotype.H264Prof,
			},
			ExtraData:         encode.TestData(crowd180p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
		}, {
			Name: "h264_360p",
			Val: encode.TestOptionsNew{
				WebMName: crowd360p,
				Profile:  videotype.H264Prof,
			},
			ExtraData:         encode.TestData(crowd360p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
		}, {
			Name: "h264_720p",
			Val: encode.TestOptionsNew{
				WebMName: crowd720p,
				Profile:  videotype.H264Prof,
			},
			ExtraData:         encode.TestData(crowd720p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
		}, {
			Name: "h264_1080p",
			Val: encode.TestOptionsNew{
				WebMName: crowd1080p,
				Profile:  videotype.H264Prof,
			},
			ExtraData:         encode.TestData(crowd1080p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
		}, {
			Name: "h264_2160p",
			Val: encode.TestOptionsNew{
				WebMName: crowd2160p,
				Profile:  videotype.H264Prof,
			},
			ExtraData:         encode.TestData(crowd2160p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264_4K},
		}, {
			Name: "vp8_192p",
			Val: encode.TestOptionsNew{
				WebMName: crowd180p,
				Profile:  videotype.VP8Prof,
			},
			ExtraData:         encode.TestData(crowd180p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name: "vp8_360p",
			Val: encode.TestOptionsNew{
				WebMName: crowd360p,
				Profile:  videotype.VP8Prof,
			},
			ExtraData:         encode.TestData(crowd360p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name: "vp8_720p",
			Val: encode.TestOptionsNew{
				WebMName: crowd720p,
				Profile:  videotype.VP8Prof,
			},
			ExtraData:         encode.TestData(crowd720p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name: "vp8_1080p",
			Val: encode.TestOptionsNew{
				WebMName: crowd1080p,
				Profile:  videotype.VP8Prof,
			},
			ExtraData:         encode.TestData(crowd1080p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name: "vp8_2160p",
			Val: encode.TestOptionsNew{
				WebMName: crowd2160p,
				Profile:  videotype.VP8Prof,
			},
			ExtraData:         encode.TestData(crowd2160p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8_4K},
		}, {
			Name: "vp9_192p",
			Val: encode.TestOptionsNew{
				WebMName: crowd180p,
				Profile:  videotype.VP9Prof,
			},
			ExtraData:         encode.TestData(crowd180p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
		}, {
			Name: "vp9_360p",
			Val: encode.TestOptionsNew{
				WebMName: crowd360p,
				Profile:  videotype.VP9Prof,
			},
			ExtraData:         encode.TestData(crowd360p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
		}, {
			Name: "vp9_720p",
			Val: encode.TestOptionsNew{
				WebMName: crowd720p,
				Profile:  videotype.VP9Prof,
			},
			ExtraData:         encode.TestData(crowd720p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
		}, {
			Name: "vp9_1080p",
			Val: encode.TestOptionsNew{
				WebMName: crowd1080p,
				Profile:  videotype.VP9Prof,
			},
			ExtraData:         encode.TestData(crowd1080p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
		}, {
			Name: "vp9_2160p",
			Val: encode.TestOptionsNew{
				WebMName: crowd2160p,
				Profile:  videotype.VP9Prof,
			},
			ExtraData:         encode.TestData(crowd2160p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9_4K},
		}},
	})
}

func EncodeAccelNewPerf(ctx context.Context, s *testing.State) {
	if err := encode.RunNewAccelVideoPerfTest(ctx, s, s.Param().(encode.TestOptionsNew)); err != nil {
		s.Fatal("test failed: ", err)
	}
}
