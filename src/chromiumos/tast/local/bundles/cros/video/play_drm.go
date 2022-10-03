// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlayDRM,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks HW protected DRM video playback in Chrome is working",
		Contacts: []string{
			"jkardatzke@google.com",
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome", "protected_content"},
		Params: []testing.Param{{
			Name:              "cencv1_h264_ctr",
			Val:               "tulip_480p_h264_cencv1_ctr.mpd",
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         append(play.DRMDataFiles(), "tulip_480p_h264_cencv1_ctr.mp4", "tulip_audio_aac_cencv1_ctr.mp4", "tulip_480p_h264_cencv1_ctr.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeCTRV1H264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithDistinctiveIdentifier",
		}, {
			Name:              "cencv1_h264_multislice_ctr",
			Val:               "tulip_480p_h264_multislice_cencv1_ctr.mpd",
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         append(play.DRMDataFiles(), "tulip_480p_h264_multislice_cencv1_ctr.mp4", "tulip_audio_aac_cencv1_ctr.mp4", "tulip_480p_h264_multislice_cencv1_ctr.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeCTRV1H264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithDistinctiveIdentifier",
		}, {
			Name:              "cencv3_h264_cbc",
			Val:               "tulip_480p_h264_cencv3_cbc.mpd",
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         append(play.DRMDataFiles(), "tulip_480p_h264_cencv3_cbc.mp4", "tulip_audio_aac_cencv3_cbc.mp4", "tulip_480p_h264_cencv3_cbc.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeCBCV3H264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithDistinctiveIdentifier",
		}, {
			Name:              "cencv3_h264_ctr",
			Val:               "tulip_480p_h264_cencv3_ctr.mpd",
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         append(play.DRMDataFiles(), "tulip_480p_h264_cencv3_ctr.mp4", "tulip_audio_aac_cencv3_ctr.mp4", "tulip_480p_h264_cencv3_ctr.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeCTRV3H264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithDistinctiveIdentifier",
		}, {
			Name:              "cencv3_h264_cbc_then_ctr",
			Val:               "tulip_480p_h264_cencv3_cbc_then_ctr.mpd",
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         append(play.DRMDataFiles(), "tulip_480p_h264_cencv3_cbc.mp4", "tulip_audio_aac_cencv3_cbc.mp4", "tulip_480p_h264_cencv3_ctr.mp4", "tulip_audio_aac_cencv3_ctr.mp4", "tulip_480p_h264_cencv3_cbc_then_ctr.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeCTRV3H264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithDistinctiveIdentifier",
		}, {
			Name:              "cencv3_hevc_cbc",
			Val:               "tulip_480p_hevc_cencv3_cbc.mpd",
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         append(play.DRMDataFiles(), "tulip_480p_hevc_cencv3_cbc.mp4", "tulip_audio_aac_cencv3_cbc.mp4", "tulip_480p_hevc_cencv3_cbc.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeCBCV3HEVC, "proprietary_codecs"},
			Fixture:           "chromeVideoWithDistinctiveIdentifier",
		}, {
			Name:              "cencv3_hevc_ctr",
			Val:               "tulip_480p_hevc_cencv3_ctr.mpd",
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         append(play.DRMDataFiles(), "tulip_480p_hevc_cencv3_ctr.mp4", "tulip_audio_aac_cencv3_ctr.mp4", "tulip_480p_hevc_cencv3_ctr.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeCTRV3HEVC, "proprietary_codecs"},
			Fixture:           "chromeVideoWithDistinctiveIdentifier",
		}, {
			Name:              "cencv3_vp9_cbc",
			Val:               "tulip_480p_vp9_cencv3_cbc.mpd",
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         append(play.DRMDataFiles(), "tulip_480p_vp9_cencv3_cbc.mp4", "tulip_audio_aac_cencv3_cbc.mp4", "tulip_480p_vp9_cencv3_cbc.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeCBCV3VP9, "proprietary_codecs"},
			Fixture:           "chromeVideoWithDistinctiveIdentifier",
		}, {
			Name:              "cencv3_vp9_ctr",
			Val:               "tulip_480p_vp9_cencv3_ctr.mpd",
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         append(play.DRMDataFiles(), "tulip_480p_vp9_cencv3_ctr.mp4", "tulip_audio_aac_cencv3_ctr.mp4", "tulip_480p_vp9_cencv3_ctr.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeCTRV3VP9, "proprietary_codecs"},
			Fixture:           "chromeVideoWithDistinctiveIdentifier",
		}, {
			Name:              "cencv3_av1_cbc",
			Val:               "tulip_480p_av1_cencv3_cbc.mpd",
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         append(play.DRMDataFiles(), "tulip_480p_av1_cencv3_cbc.webm", "tulip_audio_aac_cencv3_cbc.mp4", "tulip_480p_av1_cencv3_cbc.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeCBCV3AV1, "proprietary_codecs"},
			Fixture:           "chromeVideoWithDistinctiveIdentifier",
		}, {
			Name:              "cencv3_av1_ctr",
			Val:               "tulip_480p_av1_cencv3_ctr.mpd",
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         append(play.DRMDataFiles(), "tulip_480p_av1_cencv3_ctr.webm", "tulip_audio_aac_cencv3_ctr.mp4", "tulip_480p_av1_cencv3_ctr.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeCTRV3AV1, "proprietary_codecs"},
			Fixture:           "chromeVideoWithDistinctiveIdentifier",
		}},
	})
}

// PlayDRM plays a given file in Chrome and verifies that the playback happens
// correctly and that a screenshot of the video will be solid black (which is
// another indicator of HW DRM). This will use the Shaka player to playback a
// Widevine DRM protected MPD video via MSE/EME.
func PlayDRM(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	const unmutePlayer = false

	if err := play.TestPlay(ctx, s, cr, cr, s.Param().(string), play.DRMVideo, play.VerifyHWDRMUsed, unmutePlayer); err != nil {
		s.Fatal("TestPlay failed: ", err)
	}
}
