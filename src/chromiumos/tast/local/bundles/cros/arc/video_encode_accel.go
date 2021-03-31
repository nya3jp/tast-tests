// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/c2e2etest"
	"chromiumos/tast/local/bundles/cros/arc/video"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

const (
	// Enable to cache the extracted raw video to speed up the test.
	veaCacheExtractedVideo = false
	// Enable to download the video file encoded by the test.
	veaPullEncodedVideo = false
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoEncodeAccel,
		Desc:         "Verifies ARC++ and ARCVM hardware encode acceleration by running the arcvideoencoder_test binary",
		Contacts:     []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{c2e2etest.X86ApkName, c2e2etest.ArmApkName},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBootedWithVideoLogging",
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			Name: "h264_192p_i420",
			Val: video.EncodeTestOptions{
				Profile:     videotype.H264MainProf,
				Params:      video.Bear192P,
				PixelFormat: videotype.I420,
			},
			ExtraData:         []string{video.Bear192P.Name},
			ExtraSoftwareDeps: []string{"android_p", caps.HWEncodeH264},
		}, {
			Name: "h264_192p_i420_vm",
			Val: video.EncodeTestOptions{
				Profile:     videotype.H264MainProf,
				Params:      video.Bear192P,
				PixelFormat: videotype.I420,
			},
			ExtraData:         []string{video.Bear192P.Name},
			ExtraSoftwareDeps: []string{"android_vm", caps.HWEncodeH264},
		}, {
			Name: "h264_360p_i420",
			Val: video.EncodeTestOptions{
				Profile:     videotype.H264MainProf,
				Params:      video.Tulip360P,
				PixelFormat: videotype.I420},
			ExtraData:         []string{video.Tulip360P.Name},
			ExtraSoftwareDeps: []string{"android_p", caps.HWEncodeH264},
		}, {
			Name: "h264_360p_i420_vm",
			Val: video.EncodeTestOptions{
				Profile:     videotype.H264MainProf,
				Params:      video.Tulip360P,
				PixelFormat: videotype.I420},
			ExtraData:         []string{video.Tulip360P.Name},
			ExtraSoftwareDeps: []string{"android_vm", caps.HWEncodeH264},
		}, {
			Name: "h264_720p_i420",
			Val: video.EncodeTestOptions{
				Profile:     videotype.H264MainProf,
				Params:      video.Tulip720P,
				PixelFormat: videotype.I420},
			ExtraData:         []string{video.Tulip720P.Name},
			ExtraSoftwareDeps: []string{"android_p", caps.HWEncodeH264},
		}, {
			Name: "h264_720p_i420_vm",
			Val: video.EncodeTestOptions{
				Profile:     videotype.H264MainProf,
				Params:      video.Tulip720P,
				PixelFormat: videotype.I420},
			ExtraData:         []string{video.Tulip720P.Name},
			ExtraSoftwareDeps: []string{"android_vm", caps.HWEncodeH264},
		}, {
			Name: "h264_1080p_i420",
			Val: video.EncodeTestOptions{
				Profile:     videotype.H264MainProf,
				Params:      video.Crowd1080P,
				PixelFormat: videotype.I420},
			ExtraData:         []string{video.Crowd1080P.Name},
			ExtraSoftwareDeps: []string{"android_p", caps.HWEncodeH264},
		}, {
			Name: "h264_1080p_i420_vm",
			Val: video.EncodeTestOptions{
				Profile:     videotype.H264MainProf,
				Params:      video.Crowd1080P,
				PixelFormat: videotype.I420},
			ExtraData:         []string{video.Crowd1080P.Name},
			ExtraSoftwareDeps: []string{"android_vm", caps.HWEncodeH264},
		}, {
			Name: "vp8_192p_i420_vm",
			Val: video.EncodeTestOptions{
				Profile:     videotype.VP8Prof,
				Params:      video.Bear192P,
				PixelFormat: videotype.I420,
			},
			ExtraData:         []string{video.Bear192P.Name},
			ExtraSoftwareDeps: []string{"android_vm", caps.HWEncodeVP8},
		}, {
			Name: "vp8_360p_i420_vm",
			Val: video.EncodeTestOptions{
				Profile:     videotype.VP8Prof,
				Params:      video.Tulip360P,
				PixelFormat: videotype.I420},
			ExtraData:         []string{video.Tulip360P.Name},
			ExtraSoftwareDeps: []string{"android_vm", caps.HWEncodeVP8},
		}, {
			Name: "vp8_720p_i420_vm",
			Val: video.EncodeTestOptions{
				Profile:     videotype.VP8Prof,
				Params:      video.Tulip720P,
				PixelFormat: videotype.I420},
			ExtraData:         []string{video.Tulip720P.Name},
			ExtraSoftwareDeps: []string{"android_vm", caps.HWEncodeVP8},
		}, {
			Name: "vp8_1080p_i420_vm",
			Val: video.EncodeTestOptions{
				Profile:     videotype.VP8Prof,
				Params:      video.Crowd1080P,
				PixelFormat: videotype.I420},
			ExtraData:         []string{video.Crowd1080P.Name},
			ExtraSoftwareDeps: []string{"android_vm", caps.HWEncodeVP8},
		}, {
			Name: "vp9_192p_i420_vm",
			Val: video.EncodeTestOptions{
				Profile:     videotype.VP9Prof,
				Params:      video.Bear192P,
				PixelFormat: videotype.I420,
			},
			ExtraData:         []string{video.Bear192P.Name},
			ExtraSoftwareDeps: []string{"android_vm", caps.HWEncodeVP9},
		}, {
			Name: "vp9_360p_i420_vm",
			Val: video.EncodeTestOptions{
				Profile:     videotype.VP9Prof,
				Params:      video.Tulip360P,
				PixelFormat: videotype.I420},
			ExtraData:         []string{video.Tulip360P.Name},
			ExtraSoftwareDeps: []string{"android_vm", caps.HWEncodeVP9},
		}, {
			Name: "vp9_720p_i420_vm",
			Val: video.EncodeTestOptions{
				Profile:     videotype.VP9Prof,
				Params:      video.Tulip720P,
				PixelFormat: videotype.I420},
			ExtraData:         []string{video.Tulip720P.Name},
			ExtraSoftwareDeps: []string{"android_vm", caps.HWEncodeVP9},
		}, {
			Name: "vp9_1080p_i420_vm",
			Val: video.EncodeTestOptions{
				Profile:     videotype.VP9Prof,
				Params:      video.Crowd1080P,
				PixelFormat: videotype.I420},
			ExtraData:         []string{video.Crowd1080P.Name},
			ExtraSoftwareDeps: []string{"android_vm", caps.HWEncodeVP9},
		}},
	})
}

func VideoEncodeAccel(ctx context.Context, s *testing.State) {
	video.RunARCVideoTest(ctx, s, s.FixtValue().(*arc.PreData).ARC,
		s.Param().(video.EncodeTestOptions), veaPullEncodedVideo, veaCacheExtractedVideo)
}
