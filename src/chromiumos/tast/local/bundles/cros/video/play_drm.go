// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PlayDRM,
		Desc: "Checks HW protected DRM video playback in Chrome is working",
		Contacts: []string{
			"jkardatzke@google.com",
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome", "protected_content"},
		Params: []testing.Param{{
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
			Name:              "cencv3_hevc_cbc",
			Val:               "tulip_480p_hevc_cencv3_cbc.mpd",
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         append(play.DRMDataFiles(), "tulip_480p_hevc_cencv3_cbc.mp4", "tulip_audio_aac_cencv3_cbc.mp4", "tulip_480p_hevc_cencv3_cbc.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeCBCV3HEVC, "proprietary_codecs"},
			Fixture:           "chromeVideoWithClearHEVCHWDecodingAndDistinctiveIdentifier",
		}, {
			Name:              "cencv3_hevc_ctr",
			Val:               "tulip_480p_hevc_cencv3_ctr.mpd",
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         append(play.DRMDataFiles(), "tulip_480p_hevc_cencv3_ctr.mp4", "tulip_audio_aac_cencv3_ctr.mp4", "tulip_480p_hevc_cencv3_ctr.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeCTRV3HEVC, "proprietary_codecs"},
			Fixture:           "chromeVideoWithClearHEVCHWDecodingAndDistinctiveIdentifier",
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
		}},
	})
}

// PlayDRM plays a given file in Chrome and verifies that the playback happens
// correctly and that a screenshot of the video will be solid black (which is
// another indicator of HW DRM). This will use the Shaka player to playback a
// Widevine DRM protected MPD video via MSE/EME.
func PlayDRM(ctx context.Context, s *testing.State) {
	if err := play.TestPlay(ctx, s, s.FixtValue().(*chrome.Chrome), s.Param().(string), play.DRMVideo, play.VerifyHWDRMUsed); err != nil {
		s.Fatal("TestPlay failed: ", err)
	}
}
