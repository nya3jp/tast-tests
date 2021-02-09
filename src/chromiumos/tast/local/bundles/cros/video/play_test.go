// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"testing"

	"chromiumos/tast/common/genparams"
)

var playTestParams = []struct {
	Name              string
	FileName          string
	VideoType         string
	VerifyMode        string
	MSEDataFiles      bool
	ExtraSoftwareDeps []string
	ExtraHardwareDeps string
	ExtraAttr         []string
	ExtraData         []string
	Fixture           string
}{{
	Name:       "av1",
	FileName:   "bear-320x240.av1.mp4",
	VideoType:  "play.NormalVideo",
	VerifyMode: "play.NoVerifyHWAcceleratorUsed",
	ExtraAttr:  []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:  []string{"video.html", "bear-320x240.av1.mp4"},
	Fixture:    "chromeVideoWithHWAV1Decoding",
}, {
	Name:              "h264",
	FileName:          "bear-320x240.h264.mp4",
	VideoType:         "play.NormalVideo",
	VerifyMode:        "play.NoVerifyHWAcceleratorUsed",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"video.html", "bear-320x240.h264.mp4"},
	ExtraSoftwareDeps: []string{`"proprietary_codecs"`},
	Fixture:           "chromeVideo",
}, {
	Name:       "vp8",
	FileName:   "bear-320x240.vp8.webm",
	VideoType:  "play.NormalVideo",
	VerifyMode: "play.NoVerifyHWAcceleratorUsed",
	ExtraAttr:  []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:  []string{"video.html", "bear-320x240.vp8.webm"},
	Fixture:    "chromeVideo",
}, {
	Name:       "vp9",
	FileName:   "bear-320x240.vp9.webm",
	VideoType:  "play.NormalVideo",
	VerifyMode: "play.NoVerifyHWAcceleratorUsed",
	ExtraAttr:  []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:  []string{"video.html", "bear-320x240.vp9.webm"},
	Fixture:    "chromeVideo",
}, {
	Name:       "vp9_hdr",
	FileName:   "peru.8k.cut.hdr.vp9.webm",
	VideoType:  "play.NormalVideo",
	VerifyMode: "play.NoVerifyHWAcceleratorUsed",
	ExtraAttr:  []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:  []string{"video.html", "peru.8k.cut.hdr.vp9.webm"},
	Fixture:    "chromeVideoWithHDRScreen",
}, {
	Name:       "av1_sw",
	FileName:   "bear-320x240.av1.mp4",
	VideoType:  "play.NormalVideo",
	VerifyMode: "play.VerifyNoHWAcceleratorUsed",
	ExtraAttr:  []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:  []string{"video.html", "bear-320x240.av1.mp4"},
	Fixture:    "chromeVideoWithSWDecoding",
}, {
	Name:              "h264_sw",
	FileName:          "bear-320x240.h264.mp4",
	VideoType:         "play.NormalVideo",
	VerifyMode:        "play.VerifyNoHWAcceleratorUsed",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"video.html", "bear-320x240.h264.mp4"},
	ExtraSoftwareDeps: []string{`"proprietary_codecs"`},
	Fixture:           "chromeVideoWithSWDecoding",
}, {
	Name:       "vp8_sw",
	FileName:   "bear-320x240.vp8.webm",
	VideoType:  "play.NormalVideo",
	VerifyMode: "play.VerifyNoHWAcceleratorUsed",
	ExtraAttr:  []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:  []string{"video.html", "bear-320x240.vp8.webm"},
	Fixture:    "chromeVideoWithSWDecoding",
}, {
	Name:       "vp9_sw",
	FileName:   "bear-320x240.vp9.webm",
	VideoType:  "play.NormalVideo",
	VerifyMode: "play.VerifyNoHWAcceleratorUsed",
	ExtraAttr:  []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:  []string{"video.html", "bear-320x240.vp9.webm"},
	Fixture:    "chromeVideoWithSWDecoding",
}, {
	Name:       "vp9_2_sw",
	FileName:   "bear-320x240.vp9.2.webm",
	VideoType:  "play.NormalVideo",
	VerifyMode: "play.VerifyNoHWAcceleratorUsed",
	ExtraAttr:  []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:  []string{"video.html", `"bear-320x240.vp9.2.webm"`},
	Fixture:    "chromeVideoWithSWDecoding",
}, {
	Name:       "vp9_sw_hdr",
	FileName:   "peru.8k.cut.hdr.vp9.webm",
	VideoType:  "play.NormalVideo",
	VerifyMode: "play.VerifyNoHWAcceleratorUsed",
	ExtraAttr:  []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:  []string{"video.html", "peru.8k.cut.hdr.vp9.webm"},
	Fixture:    "chromeVideoWithSWDecodingAndHDRScreen",
}, {
	Name:              "av1_hw",
	FileName:          "bear-320x240.av1.mp4",
	VideoType:         "play.NormalVideo",
	VerifyMode:        "play.VerifyHWAcceleratorUsed",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"video.html", "bear-320x240.av1.mp4"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeAV1"},
	Fixture:           "chromeVideoWithHWAV1Decoding",
}, {
	Name:              "h264_hw",
	FileName:          "bear-320x240.h264.mp4",
	VideoType:         "play.NormalVideo",
	VerifyMode:        "play.VerifyHWAcceleratorUsed",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"video.html", "bear-320x240.h264.mp4"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeH264", `"proprietary_codecs"`},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp8_hw",
	FileName:          "bear-320x240.vp8.webm",
	VideoType:         "play.NormalVideo",
	VerifyMode:        "play.VerifyHWAcceleratorUsed",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"video.html", "bear-320x240.vp8.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP8"},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp9_hw",
	FileName:          "bear-320x240.vp9.webm",
	VideoType:         "play.NormalVideo",
	VerifyMode:        "play.VerifyHWAcceleratorUsed",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"video.html", "bear-320x240.vp9.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9"},
	Fixture:           "chromeVideo",
}, {
	Name:       "vp9_2_hw",
	FileName:   "bear-320x240.vp9.2.webm",
	VideoType:  "play.NormalVideo",
	VerifyMode: "play.VerifyHWAcceleratorUsed",
	ExtraAttr:  []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:  []string{"video.html", `"bear-320x240.vp9.2.webm"`},
	// VP9 Profile 2 is only supported by the direct Video Decoder.
	ExtraSoftwareDeps: []string{`"video_decoder_direct"`, "caps.HWDecodeVP9_2"},
	Fixture:           "chromeVideo",
}, {
	Name:       "vp9_hw_hdr",
	FileName:   "peru.8k.cut.hdr.vp9.webm",
	VideoType:  "play.NormalVideo",
	VerifyMode: "play.VerifyHWAcceleratorUsed",
	ExtraAttr:  []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:  []string{"video.html", "peru.8k.cut.hdr.vp9.webm"},
	// TODO(crbug.com/1057870): filter this by Intel SoC generation: KBL+. For now, kohaku will do.
	ExtraHardwareDeps: "hwdep.D(hwdep.Model(\"kohaku\"))",
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9_2"},
	Fixture:           "chromeVideoWithHDRScreen",
}, {
	Name:              "h264_hw_mse",
	FileName:          "bear-320x240.h264.mpd",
	VideoType:         "play.MSEVideo",
	VerifyMode:        "play.VerifyHWAcceleratorUsed",
	MSEDataFiles:      true,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"bear-320x240-video-only.h264.mp4", "bear-320x240-audio-only.aac.mp4", "bear-320x240.h264.mpd"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeH264", `"proprietary_codecs"`},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp8_hw_mse",
	FileName:          "bear-320x240.vp8.mpd",
	VideoType:         "play.MSEVideo",
	VerifyMode:        "play.VerifyHWAcceleratorUsed",
	MSEDataFiles:      true,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"bear-320x240-video-only.vp8.webm", "bear-320x240-audio-only.vorbis.webm", "bear-320x240.vp8.mpd"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP8"},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp9_hw_mse",
	FileName:          "bear-320x240.vp9.mpd",
	VideoType:         "play.MSEVideo",
	VerifyMode:        "play.VerifyHWAcceleratorUsed",
	MSEDataFiles:      true,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"bear-320x240-video-only.vp9.webm", "bear-320x240-audio-only.opus.webm", "bear-320x240.vp9.mpd"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9"},
	Fixture:           "chromeVideo",
}, {
	Name:       "av1_guest",
	FileName:   "bear-320x240.av1.mp4",
	VideoType:  "play.NormalVideo",
	VerifyMode: "play.NoVerifyHWAcceleratorUsed",
	ExtraAttr:  []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:  []string{"video.html", "bear-320x240.av1.mp4"},
	Fixture:    "chromeVideoWithGuestLoginAndHWAV1Decoding",
}, {
	Name:              "h264_guest",
	FileName:          "bear-320x240.h264.mp4",
	VideoType:         "play.NormalVideo",
	VerifyMode:        "play.NoVerifyHWAcceleratorUsed",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"video.html", "bear-320x240.h264.mp4"},
	ExtraSoftwareDeps: []string{`"proprietary_codecs"`},
	Fixture:           "chromeVideoWithGuestLogin",
}, {
	Name:       "vp8_guest",
	FileName:   "bear-320x240.vp8.webm",
	VideoType:  "play.NormalVideo",
	VerifyMode: "play.NoVerifyHWAcceleratorUsed",
	ExtraAttr:  []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:  []string{"video.html", "bear-320x240.vp8.webm"},
	Fixture:    "chromeVideoWithGuestLogin",
}, {
	Name:       "vp9_guest",
	FileName:   "bear-320x240.vp9.webm",
	VideoType:  "play.NormalVideo",
	VerifyMode: "play.NoVerifyHWAcceleratorUsed",
	ExtraAttr:  []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:  []string{"video.html", "bear-320x240.vp9.webm"},
	Fixture:    "chromeVideoWithGuestLogin",
}, {
	Name:              "h264_hw_alt",
	FileName:          "bear-320x240.h264.mp4",
	VideoType:         "play.NormalVideo",
	VerifyMode:        "play.VerifyHWAcceleratorUsed",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"video.html", "bear-320x240.h264.mp4"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeH264", `"video_decoder_legacy_supported"`, `"proprietary_codecs"`},
	Fixture:           "chromeAlternateVideoDecoder",
}, {
	Name:              "vp8_hw_alt",
	FileName:          "bear-320x240.vp8.webm",
	VideoType:         "play.NormalVideo",
	VerifyMode:        "play.VerifyHWAcceleratorUsed",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"video.html", "bear-320x240.vp8.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP8", `"video_decoder_legacy_supported"`},
	Fixture:           "chromeAlternateVideoDecoder",
}, {
	Name:              "vp9_hw_alt",
	FileName:          "bear-320x240.vp9.webm",
	VideoType:         "play.NormalVideo",
	VerifyMode:        "play.VerifyHWAcceleratorUsed",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"video.html", "bear-320x240.vp9.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9", `"video_decoder_legacy_supported"`},
	Fixture:           "chromeAlternateVideoDecoder",
}, {
	Name:       "vp9_2_hw_alt",
	FileName:   "bear-320x240.vp9.2.webm",
	VideoType:  "play.NormalVideo",
	VerifyMode: "play.VerifyHWAcceleratorUsed",
	ExtraAttr:  []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:  []string{"video.html", "bear-320x240.vp9.2.webm"},
	// VP9 Profile 2 is only supported by the direct Video Decoder so we only
	// want to run this case if that is not enabled by default, i.e. if the
	// platform is configured to use the legacy video decoder by default.
	ExtraSoftwareDeps: []string{`"video_decoder_legacy"`, `"video_decoder_legacy_supported"`, "caps.HWDecodeVP9_2"},
	Fixture:           "chromeAlternateVideoDecoder",
},
}

func TestPlayPerf(t *testing.T) {
	code := genparams.Template(t, `{{ range . }}{
	Name: {{ .Name | fmt }},
	Val: playParams{
		fileName: {{ .FileName | fmt }},
		videoType: {{ .VideoType }},
		verifyMode: {{ .VerifyMode }},
	},
	{{ if .ExtraAttr }}
	ExtraAttr: {{ .ExtraAttr | fmt }},
	{{ end }}
	{{ if .MSEDataFiles }}
	ExtraData: append(play.MSEDataFiles(), {{ range .ExtraData }} {{ . | fmt }}, {{ end }}),
	{{ else }}
	ExtraData: {{ .ExtraData | fmt }},
  {{ end }}
	{{ if .ExtraHardwareDeps }}
	ExtraHardwareDeps: {{ .ExtraHardwareDeps }},
	{{ end }}
	{{ if .ExtraSoftwareDeps }}
	ExtraSoftwareDeps: []string{ {{ range .ExtraSoftwareDeps }} {{ . }}, {{ end }} },
	{{ end }}
	Fixture: {{ .Fixture | fmt }},
}, {{ end }}`, playTestParams)
	genparams.Ensure(t, "play.go", code)
}
