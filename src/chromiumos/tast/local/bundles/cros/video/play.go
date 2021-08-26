// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type playParams struct {
	fileName     string
	videoType    play.VideoType
	verifyMode   play.VerifyHWAcceleratorMode
	chromeType   lacros.ChromeType
	unmutePlayer bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Play,
		Desc: "Checks simple video playback in Chrome is working",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "av1",
			Val: playParams{
				fileName:   "bear-320x240.av1.mp4",
				videoType:  play.NormalVideo,
				verifyMode: play.NoVerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.av1.mp4"},
			Fixture:   "chromeVideoWithHWAV1Decoding",
		}, {
			Name: "h264",
			Val: playParams{
				fileName:   "bear-320x240.h264.mp4",
				videoType:  play.NormalVideo,
				verifyMode: play.NoVerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp8",
			Val: playParams{
				fileName:   "bear-320x240.vp8.webm",
				videoType:  play.NormalVideo,
				verifyMode: play.NoVerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.vp8.webm"},
			Fixture:   "chromeVideo",
		}, {
			Name: "vp9",
			Val: playParams{
				fileName:   "bear-320x240.vp9.webm",
				videoType:  play.NormalVideo,
				verifyMode: play.NoVerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.vp9.webm"},
			Fixture:   "chromeVideo",
		}, {
			Name: "vp9_hdr",
			Val: playParams{
				fileName:   "peru.8k.cut.hdr.vp9.webm",
				videoType:  play.NormalVideo,
				verifyMode: play.NoVerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData: []string{"video.html", "peru.8k.cut.hdr.vp9.webm"},
			Fixture:   "chromeVideoWithHDRScreen",
		}, {
			Name: "av1_sw",
			Val: playParams{
				fileName:   "bear-320x240.av1.mp4",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyNoHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.av1.mp4"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name: "h264_sw",
			Val: playParams{
				fileName:   "bear-320x240.h264.mp4",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyNoHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			Fixture:           "chromeVideoWithSWDecoding",
		}, {
			Name: "vp8_sw",
			Val: playParams{
				fileName:   "bear-320x240.vp8.webm",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyNoHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.vp8.webm"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name: "vp9_sw",
			Val: playParams{
				fileName:   "bear-320x240.vp9.webm",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyNoHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.vp9.webm"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name: "vp9_2_sw",
			Val: playParams{
				fileName:   "bear-320x240.vp9.2.webm",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyNoHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.vp9.2.webm"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name: "vp9_sw_hdr",
			Val: playParams{
				fileName:   "peru.8k.cut.hdr.vp9.webm",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyNoHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData: []string{"video.html", "peru.8k.cut.hdr.vp9.webm"},
			Fixture:   "chromeVideoWithSWDecodingAndHDRScreen",
		}, {
			Name: "av1_hw",
			Val: playParams{
				fileName:   "bear-320x240.av1.mp4",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.av1.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name: "av1_hw_odd_dimension",
			Val: playParams{
				fileName:   "bear-321x241.av1.mp4",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-321x241.av1.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name: "h264_hw",
			Val: playParams{
				fileName:   "bear-320x240.h264.mp4",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideo",
		}, {
			Name: "h264_hw_lacros",
			Val: playParams{
				fileName:   "bear-320x240.h264.mp4",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeLacros,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs", "lacros"},
			Fixture:           "chromeVideoLacros",
		}, {
			Name: "vp8_hw",
			Val: playParams{
				fileName:   "bear-320x240.vp8.webm",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp8_hw_odd_dimension",
			Val: playParams{
				fileName:   "bear-321x241.vp8.webm",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-321x241.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp9_hw",
			Val: playParams{
				fileName:   "bear-320x240.vp9.webm",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp9_hw_odd_dimension",
			Val: playParams{
				fileName:   "bear-321x241.vp9.webm",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-321x241.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp9_hw_lacros",
			Val: playParams{
				fileName:   "bear-320x240.vp9.webm",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeLacros,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "lacros"},
			Fixture:           "chromeVideoLacros",
		}, {
			Name: "vp9_2_hw",
			Val: playParams{
				fileName:   "bear-320x240.vp9.2.webm",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.vp9.2.webm"},
			// VP9 Profile 2 is only supported by the direct Video Decoder.
			ExtraSoftwareDeps: []string{"video_decoder_direct", caps.HWDecodeVP9_2},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp9_hw_hdr",
			Val: playParams{
				fileName:   "peru.8k.cut.hdr.vp9.webm",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "peru.8k.cut.hdr.vp9.webm"},
			// TODO(crbug.com/1057870): filter this by Intel SoC generation: KBL+. For now, kohaku will do.
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kohaku")),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2},
			Fixture:           "chromeVideoWithHDRScreen",
		}, {
			Name: "hevc_hw",
			Val: playParams{
				fileName:   "bear-320x240.hevc.mp4",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.hevc.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs", "protected_content"},
			Fixture:           "chromeVideoWithClearHEVCHWDecoding",
		}, {
			Name: "hevc10_hw",
			Val: playParams{
				fileName:   "bear-320x240.hevc10.mp4",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.hevc10.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC10BPP, "proprietary_codecs", "protected_content"},
			Fixture:           "chromeVideoWithClearHEVCHWDecoding",
		}, {
			Name: "h264_hw_mse",
			Val: playParams{
				fileName:   "bear-320x240.h264.mpd",
				videoType:  play.MSEVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         append(play.MSEDataFiles(), "bear-320x240-video-only.h264.mp4", "bear-320x240-audio-only.aac.mp4", "bear-320x240.h264.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp8_hw_mse",
			Val: playParams{
				fileName:   "bear-320x240.vp8.mpd",
				videoType:  play.MSEVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         append(play.MSEDataFiles(), "bear-320x240-video-only.vp8.webm", "bear-320x240-audio-only.vorbis.webm", "bear-320x240.vp8.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp9_hw_mse",
			Val: playParams{
				fileName:   "bear-320x240.vp9.mpd",
				videoType:  play.MSEVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         append(play.MSEDataFiles(), "bear-320x240-video-only.vp9.webm", "bear-320x240-audio-only.opus.webm", "bear-320x240.vp9.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideo",
		}, {
			Name: "hevc_hw_mse",
			Val: playParams{
				fileName:   "bear-320x240.hevc.mpd",
				videoType:  play.MSEVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         append(play.MSEDataFiles(), "bear-320x240-video-only.hevc.mp4", "bear-320x240-audio-only.aac.mp4", "bear-320x240.hevc.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs", "protected_content"},
			Fixture:           "chromeVideoWithClearHEVCHWDecoding",
		}, {
			Name: "av1_guest",
			Val: playParams{
				fileName:   "bear-320x240.av1.mp4",
				videoType:  play.NormalVideo,
				verifyMode: play.NoVerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.av1.mp4"},
			Fixture:   "chromeVideoWithGuestLoginAndHWAV1Decoding",
		}, {
			Name: "h264_guest",
			Val: playParams{
				fileName:   "bear-320x240.h264.mp4",
				videoType:  play.NormalVideo,
				verifyMode: play.NoVerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			Fixture:           "chromeVideoWithGuestLogin",
		}, {
			Name: "vp8_guest",
			Val: playParams{
				fileName:   "bear-320x240.vp8.webm",
				videoType:  play.NormalVideo,
				verifyMode: play.NoVerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.vp8.webm"},
			Fixture:   "chromeVideoWithGuestLogin",
		}, {
			Name: "vp9_guest",
			Val: playParams{
				fileName:   "bear-320x240.vp9.webm",
				videoType:  play.NormalVideo,
				verifyMode: play.NoVerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.vp9.webm"},
			Fixture:   "chromeVideoWithGuestLogin",
		}, {
			Name: "av1_unmuted",
			Val: playParams{
				fileName:     "bear-320x240.av1.mp4",
				videoType:    play.NormalVideo,
				verifyMode:   play.NoVerifyHWAcceleratorUsed,
				chromeType:   lacros.ChromeTypeChromeOS,
				unmutePlayer: true,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.av1.mp4"},
			Fixture:   "chromeVideoWithHWAV1Decoding",
		}, {
			Name: "h264_unmuted",
			Val: playParams{
				fileName:     "bear-320x240.h264.mp4",
				videoType:    play.NormalVideo,
				verifyMode:   play.NoVerifyHWAcceleratorUsed,
				chromeType:   lacros.ChromeTypeChromeOS,
				unmutePlayer: true,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp8_unmuted",
			Val: playParams{
				fileName:     "bear-320x240.vp8.webm",
				videoType:    play.NormalVideo,
				verifyMode:   play.NoVerifyHWAcceleratorUsed,
				chromeType:   lacros.ChromeTypeChromeOS,
				unmutePlayer: true,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.vp8.webm"},
			Fixture:   "chromeVideo",
		}, {
			Name: "vp9_unmuted",
			Val: playParams{
				fileName:     "bear-320x240.vp9.webm",
				videoType:    play.NormalVideo,
				verifyMode:   play.NoVerifyHWAcceleratorUsed,
				chromeType:   lacros.ChromeTypeChromeOS,
				unmutePlayer: true,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.vp9.webm"},
			Fixture:   "chromeVideo",
		}, {
			Name: "h264_hw_alt",
			Val: playParams{
				fileName:   "bear-320x240.h264.mp4",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "video_decoder_legacy_supported", "proprietary_codecs"},
			Fixture:           "chromeAlternateVideoDecoder",
		}, {
			Name: "vp8_hw_alt",
			Val: playParams{
				fileName:   "bear-320x240.vp8.webm",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "video_decoder_legacy_supported"},
			Fixture:           "chromeAlternateVideoDecoder",
		}, {
			Name: "vp9_hw_alt",
			Val: playParams{
				fileName:   "bear-320x240.vp9.webm",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "video_decoder_legacy_supported"},
			Fixture:           "chromeAlternateVideoDecoder",
		}, {
			Name: "vp9_2_hw_alt",
			Val: playParams{
				fileName:   "bear-320x240.vp9.2.webm",
				videoType:  play.NormalVideo,
				verifyMode: play.VerifyHWAcceleratorUsed,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.vp9.2.webm"},
			// VP9 Profile 2 is only supported by the direct Video Decoder so we only
			// want to run this case if that is not enabled by default, i.e. if the
			// platform is configured to use the legacy video decoder by default.
			ExtraSoftwareDeps: []string{"video_decoder_legacy", "video_decoder_legacy_supported", caps.HWDecodeVP9_2},
			Fixture:           "chromeAlternateVideoDecoder",
		}},
	})
}

// Play plays a given file in Chrome and verifies that the playback happens
// correctly; if verifyMode says so, it verifies if playback uses hardware
// acceleration.
// If videoType is NormalVideo, a simple <video> player is instantiated with the
// input filename, whereas if it's MSEVideo,then we try to feed the media files
// via a SourceBuffer (using MSE, the Media Source Extensions protocol, and a
// DASH MPD file).
func Play(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(playParams)

	cr, l, cs, err := lacros.Setup(ctx, s.FixtValue(), testOpt.chromeType)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacrosChrome(ctx, l)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	defer tconn.Close()

	if err := play.TestPlay(ctx, s, cs, cr, testOpt.fileName, testOpt.videoType, testOpt.verifyMode, testOpt.unmutePlayer); err != nil {
		s.Fatal("TestPlay failed: ", err)
	}
}
