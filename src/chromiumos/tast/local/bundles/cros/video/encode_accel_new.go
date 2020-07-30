// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/bundles/cros/video/encode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/encoding"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

func jsonFileName(webMFileName string) string {
	const webMSuffix = ".vp9.webm"
	if !strings.HasSuffix(webMFileName, webMSuffix) {
		return "error.json"
	}
	yuvName := strings.TrimSuffix(webMFileName, webMSuffix) + ".yuv"
	return yuvName + ".json"
}

func encodeTestData(webmFileName string) []string {
	return []string{webmFileName, jsonFileName(webmFileName)}
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         EncodeAccelNew,
		Desc:         "TODO",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		SoftwareDeps: []string{"chrome"},
		// TODO(crbug.com/979497): Reduce to appropriate timeout after checking the exact execution time of h264_2160p.
		Timeout: 10 * time.Minute,
		Params: []testing.Param{{
			Name: "h264_180p",
			Val: encoding.TestOptionsNew{
				WebMName: encode.Tulip180P.Name,
				JSONName: jsonFileName(encode.Tulip180P.Name),
				Profile:  videotype.H264Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         encodeTestData(encode.Tulip180P.Name),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		}, {
			Name: "h264_192p",
			Val: encoding.TestOptionsNew{
				WebMName: encode.Bear192P.Name,
				JSONName: jsonFileName(encode.Bear192P.Name),
				Profile:  videotype.H264Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         encodeTestData(encode.Bear192P.Name),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		}, {
			Name: "h264_360p",
			Val: encoding.TestOptionsNew{
				WebMName: encode.Tulip360P.Name,
				JSONName: jsonFileName(encode.Tulip360P.Name),
				Profile:  videotype.H264Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         encodeTestData(encode.Tulip360P.Name),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		}, {
			Name: "h264_720p",
			Val: encoding.TestOptionsNew{
				WebMName: encode.Tulip720P.Name,
				JSONName: jsonFileName(encode.Tulip720P.Name),
				Profile:  videotype.H264Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         encodeTestData(encode.Tulip720P.Name),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		}, {
			Name: "h264_1080p",
			Val: encoding.TestOptionsNew{
				WebMName: encode.Crowd1080P.Name,
				JSONName: jsonFileName(encode.Crowd1080P.Name),
				Profile:  videotype.H264Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         encodeTestData(encode.Crowd1080P.Name),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		}, {
			Name: "h264_2160p",
			Val: encoding.TestOptionsNew{
				WebMName: encode.Crowd2160P.Name,
				JSONName: jsonFileName(encode.Crowd2160P.Name),
				Profile:  videotype.H264Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264_4K},
			ExtraData:         encodeTestData(encode.Crowd2160P.Name),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		}, {
			Name: "vp8_180p",
			Val: encoding.TestOptionsNew{
				WebMName: encode.Tulip180P.Name,
				JSONName: jsonFileName(encode.Tulip180P.Name),
				Profile:  videotype.VP8Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         encodeTestData(encode.Tulip180P.Name),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		}, {
			Name: "vp8_192p",
			Val: encoding.TestOptionsNew{
				WebMName: encode.Bear192P.Name,
				JSONName: jsonFileName(encode.Bear192P.Name),
				Profile:  videotype.VP8Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         encodeTestData(encode.Bear192P.Name),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		}, {
			Name: "vp8_360p",
			Val: encoding.TestOptionsNew{
				WebMName: encode.Tulip360P.Name,
				JSONName: jsonFileName(encode.Tulip360P.Name),
				Profile:  videotype.VP8Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         encodeTestData(encode.Tulip360P.Name),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		}, {
			Name: "vp8_720p",
			Val: encoding.TestOptionsNew{
				WebMName: encode.Tulip720P.Name,
				JSONName: jsonFileName(encode.Tulip720P.Name),
				Profile:  videotype.VP8Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         encodeTestData(encode.Tulip720P.Name),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		}, {
			Name: "vp8_1080p",
			Val: encoding.TestOptionsNew{
				WebMName: encode.Crowd1080P.Name,
				JSONName: jsonFileName(encode.Crowd1080P.Name),
				Profile:  videotype.VP8Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         encodeTestData(encode.Crowd1080P.Name),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		}, {
			Name: "vp8_2160p",
			Val: encoding.TestOptionsNew{
				WebMName: encode.Crowd2160P.Name,
				JSONName: jsonFileName(encode.Crowd2160P.Name),
				Profile:  videotype.VP8Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8_4K},
			ExtraData:         encodeTestData(encode.Crowd2160P.Name),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		}, {
			Name: "vp9_180p",
			Val: encoding.TestOptionsNew{
				WebMName: encode.Tulip180P.Name,
				JSONName: jsonFileName(encode.Tulip180P.Name),
				Profile:  videotype.VP9Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			ExtraData:         encodeTestData(encode.Tulip180P.Name),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		}, {
			Name: "vp9_192p",
			Val: encoding.TestOptionsNew{
				WebMName: encode.Bear192P.Name,
				JSONName: jsonFileName(encode.Bear192P.Name),
				Profile:  videotype.VP9Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			ExtraData:         encodeTestData(encode.Bear192P.Name),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		}, {
			Name: "vp9_360p",
			Val: encoding.TestOptionsNew{
				WebMName: encode.Tulip360P.Name,
				JSONName: jsonFileName(encode.Tulip360P.Name),
				Profile:  videotype.VP9Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			ExtraData:         encodeTestData(encode.Tulip360P.Name),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		}, {
			Name: "vp9_720p",
			Val: encoding.TestOptionsNew{
				WebMName: encode.Tulip720P.Name,
				JSONName: jsonFileName(encode.Tulip720P.Name),
				Profile:  videotype.VP9Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			ExtraData:         encodeTestData(encode.Tulip720P.Name),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		}, {
			Name: "vp9_1080p",
			Val: encoding.TestOptionsNew{
				WebMName: encode.Crowd1080P.Name,
				JSONName: jsonFileName(encode.Crowd1080P.Name),
				Profile:  videotype.VP9Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			ExtraData:         encodeTestData(encode.Crowd1080P.Name),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		}, {
			Name: "vp9_2160p",
			Val: encoding.TestOptionsNew{
				WebMName: encode.Crowd2160P.Name,
				JSONName: jsonFileName(encode.Crowd2160P.Name),
				Profile:  videotype.VP9Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9_4K},
			ExtraData:         encodeTestData(encode.Crowd2160P.Name),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		}},
	})
}

func EncodeAccelNew(ctx context.Context, s *testing.State) {
	encode.RunNewAccelVideoTest(ctx, s, s.Param().(encoding.TestOptionsNew))
}
