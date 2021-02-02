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
	tulip180P  = "tulip2-320x180.vp9.webm"
	bear192P   = "bear-320x192.vp9.webm"
	tulip360P  = "tulip2-640x360.vp9.webm"
	tulip361P  = "crowd-641x361.vp9.webm"
	tulip720P  = "tulip2-1280x720.vp9.webm"
	crowd1080P = "crowd-1920x1080.vp9.webm"
	crowd2160P = "crowd-3840x2160.vp9.webm"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EncodeAccel,
		Desc:         "Verifies hardware encode acceleration by running the video_encode_accelerator_tests binary",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-gfx-video@google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Params: []testing.Param{{
			Name: "h264_180p",
			Val: encode.TestOptions{
				WebMName: tulip180P,
				Profile:  videotype.H264Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         encode.TestData(tulip180P),
		}, {
			Name: "h264_192p",
			Val: encode.TestOptions{
				WebMName: bear192P,
				Profile:  videotype.H264Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         encode.TestData(bear192P),
		}, {
			Name: "h264_360p",
			Val: encode.TestOptions{
				WebMName: tulip360P,
				Profile:  videotype.H264Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         encode.TestData(tulip360P),
		}, {
			Name: "h264_720p",
			Val: encode.TestOptions{
				WebMName: tulip720P,
				Profile:  videotype.H264Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         encode.TestData(tulip720P),
		}, {
			Name: "h264_1080p",
			Val: encode.TestOptions{
				WebMName: crowd1080P,
				Profile:  videotype.H264Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         encode.TestData(crowd1080P),
		}, {
			Name: "h264_2160p",
			Val: encode.TestOptions{
				WebMName: crowd2160P,
				Profile:  videotype.H264Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264_4K},
			ExtraData:         encode.TestData(crowd2160P),
		}, {
			Name: "vp8_180p",
			Val: encode.TestOptions{
				WebMName: tulip180P,
				Profile:  videotype.VP8Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         encode.TestData(tulip180P),
		}, {
			Name: "vp8_192p",
			Val: encode.TestOptions{
				WebMName: bear192P,
				Profile:  videotype.VP8Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         encode.TestData(bear192P),
		}, {
			Name: "vp8_360p",
			Val: encode.TestOptions{
				WebMName: tulip360P,
				Profile:  videotype.VP8Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         encode.TestData(tulip360P),
		}, {
			Name: "vp8_720p",
			Val: encode.TestOptions{
				WebMName: tulip720P,
				Profile:  videotype.VP8Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         encode.TestData(tulip720P),
		}, {
			Name: "vp8_1080p",
			Val: encode.TestOptions{
				WebMName: crowd1080P,
				Profile:  videotype.VP8Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         encode.TestData(crowd1080P),
		}, {
			Name: "vp8_2160p",
			Val: encode.TestOptions{
				WebMName: crowd2160P,
				Profile:  videotype.VP8Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8_4K},
			ExtraData:         encode.TestData(crowd2160P),
		}, {
			Name: "vp9_180p",
			Val: encode.TestOptions{
				WebMName: tulip180P,
				Profile:  videotype.VP9Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			ExtraData:         encode.TestData(tulip180P),
		}, {
			Name: "vp9_192p",
			Val: encode.TestOptions{
				WebMName: bear192P,
				Profile:  videotype.VP9Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			ExtraData:         encode.TestData(bear192P),
		}, {
			Name: "vp9_360p",
			Val: encode.TestOptions{
				WebMName: tulip360P,
				Profile:  videotype.VP9Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			ExtraData:         encode.TestData(tulip360P),
		}, {
			Name: "vp9_720p",
			Val: encode.TestOptions{
				WebMName: tulip720P,
				Profile:  videotype.VP9Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			ExtraData:         encode.TestData(tulip720P),
		}, {
			Name: "vp9_1080p",
			Val: encode.TestOptions{
				WebMName: crowd1080P,
				Profile:  videotype.VP9Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			ExtraData:         encode.TestData(crowd1080P),
		}, {
			Name: "vp9_2160p",
			Val: encode.TestOptions{
				WebMName: crowd2160P,
				Profile:  videotype.VP9Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9_4K},
			ExtraData:         encode.TestData(crowd2160P),
		}},
	})
}

func EncodeAccel(ctx context.Context, s *testing.State) {
	encode.RunAccelVideoTest(ctx, s, s.Param().(encode.TestOptions))
}
