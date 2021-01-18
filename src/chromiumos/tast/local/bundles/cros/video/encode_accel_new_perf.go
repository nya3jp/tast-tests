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
				WebMName: encode.Bear192P.Name,
				Profile:  videotype.H264Prof,
			},
			ExtraData:         encode.TestData(encode.Bear192P.Name),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
		}, {
			Name: "h264_360p",
			Val: encode.TestOptionsNew{
				WebMName: encode.Tulip360P.Name,
				Profile:  videotype.H264Prof,
			},
			ExtraData:         encode.TestData(encode.Tulip360P.Name),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
		}, {
			Name: "h264_720p",
			Val: encode.TestOptionsNew{
				WebMName: encode.Tulip720P.Name,
				Profile:  videotype.H264Prof,
			},
			ExtraData:         encode.TestData(encode.Tulip720P.Name),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
		}, {
			Name: "h264_1080p",
			Val: encode.TestOptionsNew{
				WebMName: encode.Crowd1080P.Name,
				Profile:  videotype.H264Prof,
			},
			ExtraData:         encode.TestData(encode.Crowd1080P.Name),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
		}, {
			Name: "h264_2160p",
			Val: encode.TestOptionsNew{
				WebMName: encode.Crowd2160P.Name,
				Profile:  videotype.H264Prof,
			},
			ExtraData:         encode.TestData(encode.Crowd2160P.Name),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264_4K},
		}, {
			Name: "vp8_192p",
			Val: encode.TestOptionsNew{
				WebMName: encode.Bear192P.Name,
				Profile:  videotype.VP8Prof,
			},
			ExtraData:         encode.TestData(encode.Bear192P.Name),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name: "vp8_360p",
			Val: encode.TestOptionsNew{
				WebMName: encode.Tulip360P.Name,
				Profile:  videotype.VP8Prof,
			},
			ExtraData:         encode.TestData(encode.Tulip360P.Name),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name: "vp8_720p",
			Val: encode.TestOptionsNew{
				WebMName: encode.Tulip720P.Name,
				Profile:  videotype.VP8Prof,
			},
			ExtraData:         encode.TestData(encode.Tulip720P.Name),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name: "vp8_1080p",
			Val: encode.TestOptionsNew{
				WebMName: encode.Crowd1080P.Name,
				Profile:  videotype.VP8Prof,
			},
			ExtraData:         encode.TestData(encode.Crowd1080P.Name),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name: "vp8_2160p",
			Val: encode.TestOptionsNew{
				WebMName: encode.Crowd2160P.Name,
				Profile:  videotype.VP8Prof,
			},
			ExtraData:         encode.TestData(encode.Crowd2160P.Name),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8_4K},
		}, {
			Name: "vp9_192p",
			Val: encode.TestOptionsNew{
				WebMName: encode.Bear192P.Name,
				Profile:  videotype.VP9Prof,
			},
			ExtraData:         encode.TestData(encode.Bear192P.Name),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
		}, {
			Name: "vp9_360p",
			Val: encode.TestOptionsNew{
				WebMName: encode.Tulip360P.Name,
				Profile:  videotype.VP9Prof,
			},
			ExtraData:         encode.TestData(encode.Tulip360P.Name),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
		}, {
			Name: "vp9_720p",
			Val: encode.TestOptionsNew{
				WebMName: encode.Tulip720P.Name,
				Profile:  videotype.VP9Prof,
			},
			ExtraData:         encode.TestData(encode.Tulip720P.Name),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
		}, {
			Name: "vp9_1080p",
			Val: encode.TestOptionsNew{
				WebMName: encode.Crowd1080P.Name,
				Profile:  videotype.VP9Prof,
			},
			ExtraData:         encode.TestData(encode.Crowd1080P.Name),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
		}, {
			Name: "vp9_2160p",
			Val: encode.TestOptionsNew{
				WebMName: encode.Crowd2160P.Name,
				Profile:  videotype.VP9Prof,
			},
			ExtraData:         encode.TestData(encode.Crowd2160P.Name),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9_4K},
		}},
	})
}

func EncodeAccelNewPerf(ctx context.Context, s *testing.State) {
	if err := encode.RunNewAccelVideoPerfTest(ctx, s, s.Param().(encode.TestOptionsNew)); err != nil {
		s.Fatal("test failed: ", err)
	}
}
