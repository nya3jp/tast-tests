// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"testing"

	"chromiumos/tast/common/genparams"
)

var params = []struct {
	Name              string
	FileName          string
	DecoderType       string
	ExtraSoftwareDeps []string
	ExtraAttr         []string
	ExtraData         []string
	Fixture           string
}{{
	Name:              "h264_144p_30fps_hw",
	FileName:          "144p_30fps_300frames.h264.mp4",
	DecoderType:       "playback.Hardware",
	ExtraSoftwareDeps: []string{"caps.HWDecodeH264", `"proprietary_codecs"`},
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"144p_30fps_300frames.h264.mp4"},
	Fixture:           "chromeVideo",
}, {
	Name:              "h264_240p_30fps_hw",
	FileName:          "240p_30fps_300frames.h264.mp4",
	DecoderType:       "playback.Hardware",
	ExtraSoftwareDeps: []string{"caps.HWDecodeH264", `"proprietary_codecs"`},
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"240p_30fps_300frames.h264.mp4"},
	Fixture:           "chromeVideo",
}, {
	Name:              "h264_360p_30fps_hw",
	FileName:          "360p_30fps_300frames.h264.mp4",
	DecoderType:       "playback.Hardware",
	ExtraSoftwareDeps: []string{"caps.HWDecodeH264", `"proprietary_codecs"`},
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"360p_30fps_300frames.h264.mp4"},
	Fixture:           "chromeVideo",
}, {
	Name:              "h264_480p_30fps_hw",
	FileName:          "480p_30fps_300frames.h264.mp4",
	DecoderType:       "playback.Hardware",
	ExtraSoftwareDeps: []string{"caps.HWDecodeH264", `"proprietary_codecs"`},
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"480p_30fps_300frames.h264.mp4"},
	Fixture:           "chromeVideo",
}, {
	Name:              "h264_720p_30fps_hw",
	FileName:          "720p_30fps_300frames.h264.mp4",
	DecoderType:       "playback.Hardware",
	ExtraSoftwareDeps: []string{"caps.HWDecodeH264", `"proprietary_codecs"`},
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"720p_30fps_300frames.h264.mp4"},
	Fixture:           "chromeVideo",
}, {
	Name:              "h264_1080p_30fps_hw",
	FileName:          "1080p_30fps_300frames.h264.mp4",
	DecoderType:       "playback.Hardware",
	ExtraSoftwareDeps: []string{"caps.HWDecodeH264", `"proprietary_codecs"`},
	ExtraData:         []string{"1080p_30fps_300frames.h264.mp4"},
	Fixture:           "chromeVideo",
}, {
	Name:              "h264_1080p_60fps_hw",
	FileName:          "1080p_60fps_600frames.h264.mp4",
	DecoderType:       "playback.Hardware",
	ExtraSoftwareDeps: []string{"caps.HWDecodeH264_60", `"proprietary_codecs"`},
	ExtraData:         []string{"1080p_60fps_600frames.h264.mp4"},
	Fixture:           "chromeVideo",
}, {
	Name:              "h264_2160p_30fps_hw",
	FileName:          "2160p_30fps_300frames.h264.mp4",
	DecoderType:       "playback.Hardware",
	ExtraSoftwareDeps: []string{"caps.HWDecodeH264_4K", `"proprietary_codecs"`},
	ExtraData:         []string{"2160p_30fps_300frames.h264.mp4"},
	Fixture:           "chromeVideo",
}, {
	Name:              "h264_2160p_60fps_hw",
	FileName:          "2160p_60fps_600frames.h264.mp4",
	DecoderType:       "playback.Hardware",
	ExtraSoftwareDeps: []string{"caps.HWDecodeH264_4K60", `"proprietary_codecs"`},
	ExtraData:         []string{"2160p_60fps_600frames.h264.mp4"},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp8_144p_30fps_hw",
	FileName:          "144p_30fps_300frames.vp8.webm",
	DecoderType:       "playback.Hardware",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"144p_30fps_300frames.vp8.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP8"},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp8_240p_30fps_hw",
	FileName:          "240p_30fps_300frames.vp8.webm",
	DecoderType:       "playback.Hardware",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"240p_30fps_300frames.vp8.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP8"},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp8_360p_30fps_hw",
	FileName:          "360p_30fps_300frames.vp8.webm",
	DecoderType:       "playback.Hardware",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"360p_30fps_300frames.vp8.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP8"},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp8_480p_30fps_hw",
	FileName:          "480p_30fps_300frames.vp8.webm",
	DecoderType:       "playback.Hardware",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"480p_30fps_300frames.vp8.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP8"},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp8_720p_30fps_hw",
	FileName:          "720p_30fps_300frames.vp8.webm",
	DecoderType:       "playback.Hardware",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"720p_30fps_300frames.vp8.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP8"},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp8_1080p_30fps_hw",
	FileName:          "1080p_30fps_300frames.vp8.webm",
	DecoderType:       "playback.Hardware",
	ExtraData:         []string{"1080p_30fps_300frames.vp8.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP8"},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp8_1080p_60fps_hw",
	FileName:          "1080p_60fps_600frames.vp8.webm",
	DecoderType:       "playback.Hardware",
	ExtraData:         []string{"1080p_60fps_600frames.vp8.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP8_60"},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp8_2160p_30fps_hw",
	FileName:          "2160p_30fps_300frames.vp8.webm",
	DecoderType:       "playback.Hardware",
	ExtraData:         []string{"2160p_30fps_300frames.vp8.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP8_4K"},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp8_2160p_60fps_hw",
	FileName:          "2160p_60fps_600frames.vp8.webm",
	DecoderType:       "playback.Hardware",
	ExtraData:         []string{"2160p_60fps_600frames.vp8.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP8_4K60"},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp9_144p_30fps_hw",
	FileName:          "144p_30fps_300frames.vp9.webm",
	DecoderType:       "playback.Hardware",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"144p_30fps_300frames.vp9.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9"},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp9_240p_30fps_hw",
	FileName:          "240p_30fps_300frames.vp9.webm",
	DecoderType:       "playback.Hardware",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"240p_30fps_300frames.vp9.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9"},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp9_360p_30fps_hw",
	FileName:          "360p_30fps_300frames.vp9.webm",
	DecoderType:       "playback.Hardware",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"360p_30fps_300frames.vp9.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9"},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp9_480p_30fps_hw",
	FileName:          "480p_30fps_300frames.vp9.webm",
	DecoderType:       "playback.Hardware",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"480p_30fps_300frames.vp9.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9"},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp9_720p_30fps_hw",
	FileName:          "720p_30fps_300frames.vp9.webm",
	DecoderType:       "playback.Hardware",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"720p_30fps_300frames.vp9.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9"},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp9_1080p_30fps_hw",
	FileName:          "1080p_30fps_300frames.vp9.webm",
	DecoderType:       "playback.Hardware",
	ExtraData:         []string{"1080p_30fps_300frames.vp9.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9"},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp9_1080p_60fps_hw",
	FileName:          "1080p_60fps_600frames.vp9.webm",
	DecoderType:       "playback.Hardware",
	ExtraData:         []string{"1080p_60fps_600frames.vp9.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9_60"},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp9_2160p_30fps_hw",
	FileName:          "2160p_30fps_300frames.vp9.webm",
	DecoderType:       "playback.Hardware",
	ExtraData:         []string{"2160p_30fps_300frames.vp9.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9_4K"},
	Fixture:           "chromeVideo",
}, {
	Name:              "vp9_2160p_60fps_hw",
	FileName:          "2160p_60fps_600frames.vp9.webm",
	DecoderType:       "playback.Hardware",
	ExtraData:         []string{"2160p_60fps_600frames.vp9.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9_4K60"},
	Fixture:           "chromeVideo",
}, {
	Name:              "av1_480p_30fps_hw",
	FileName:          "480p_30fps_300frames.av1.mp4",
	DecoderType:       "playback.Hardware",
	ExtraData:         []string{"480p_30fps_300frames.av1.mp4"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeAV1"},
	Fixture:           "chromeVideoWithHWAV1Decoding",
}, {
	Name:              "av1_720p_30fps_hw",
	FileName:          "720p_30fps_300frames.av1.mp4",
	DecoderType:       "playback.Hardware",
	ExtraData:         []string{"720p_30fps_300frames.av1.mp4"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeAV1"},
	Fixture:           "chromeVideoWithHWAV1Decoding",
}, {
	Name:              "av1_720p_60fps_hw",
	FileName:          "720p_60fps_600frames.av1.mp4",
	DecoderType:       "playback.Hardware",
	ExtraData:         []string{"720p_60fps_600frames.av1.mp4"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeAV1_60"},
	Fixture:           "chromeVideoWithHWAV1Decoding",
}, {
	Name:              "av1_1080p_30fps_hw",
	FileName:          "1080p_30fps_300frames.av1.mp4",
	DecoderType:       "playback.Hardware",
	ExtraData:         []string{"1080p_30fps_300frames.av1.mp4"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeAV1"},
	Fixture:           "chromeVideoWithHWAV1Decoding",
}, {
	Name:              "av1_1080p_60fps_hw",
	FileName:          "1080p_60fps_600frames.av1.mp4",
	DecoderType:       "playback.Hardware",
	ExtraData:         []string{"1080p_60fps_600frames.av1.mp4"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeAV1_60"},
	Fixture:           "chromeVideoWithHWAV1Decoding",
}, {
	Name:              "av1_2160p_30fps_hw",
	FileName:          "2160p_30fps_300frames.av1.mp4",
	DecoderType:       "playback.Hardware",
	ExtraData:         []string{"2160p_30fps_300frames.av1.mp4"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeAV1_4K"},
	Fixture:           "chromeVideoWithHWAV1Decoding",
}, {
	Name:              "av1_2160p_60fps_hw",
	FileName:          "2160p_60fps_600frames.av1.mp4",
	DecoderType:       "playback.Hardware",
	ExtraData:         []string{"2160p_60fps_600frames.av1.mp4"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeAV1_4K60"},
	Fixture:           "chromeVideoWithHWAV1Decoding",
}, {
	Name:              "h264_480p_30fps_sw",
	FileName:          "480p_30fps_300frames.h264.mp4",
	DecoderType:       "playback.Software",
	ExtraSoftwareDeps: []string{`"proprietary_codecs"`},
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"480p_30fps_300frames.h264.mp4"},
	Fixture:           "chromeVideoWithSWDecoding",
}, {
	Name:              "h264_720p_30fps_sw",
	FileName:          "720p_30fps_300frames.h264.mp4",
	DecoderType:       "playback.Software",
	ExtraSoftwareDeps: []string{`"proprietary_codecs"`},
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"720p_30fps_300frames.h264.mp4"},
	Fixture:           "chromeVideoWithSWDecoding",
}, {
	Name:              "h264_1080p_30fps_sw",
	FileName:          "1080p_30fps_300frames.h264.mp4",
	DecoderType:       "playback.Software",
	ExtraSoftwareDeps: []string{`"proprietary_codecs"`},
	ExtraData:         []string{"1080p_30fps_300frames.h264.mp4"},
	Fixture:           "chromeVideoWithSWDecoding",
}, {
	Name:              "h264_1080p_60fps_sw",
	FileName:          "1080p_60fps_600frames.h264.mp4",
	DecoderType:       "playback.Software",
	ExtraSoftwareDeps: []string{`"proprietary_codecs"`},
	ExtraData:         []string{"1080p_60fps_600frames.h264.mp4"},
	Fixture:           "chromeVideoWithSWDecoding",
}, {
	Name:        "vp8_480p_30fps_sw",
	FileName:    "480p_30fps_300frames.vp8.webm",
	DecoderType: "playback.Software",
	ExtraAttr:   []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:   []string{"480p_30fps_300frames.vp8.webm"},
	Fixture:     "chromeVideoWithSWDecoding",
}, {
	Name:        "vp8_720p_30fps_sw",
	FileName:    "720p_30fps_300frames.vp8.webm",
	DecoderType: "playback.Software",
	ExtraAttr:   []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:   []string{"720p_30fps_300frames.vp8.webm"},
	Fixture:     "chromeVideoWithSWDecoding",
}, {
	Name:        "vp8_1080p_30fps_sw",
	FileName:    "1080p_30fps_300frames.vp8.webm",
	DecoderType: "playback.Software",
	ExtraData:   []string{"1080p_30fps_300frames.vp8.webm"},
	Fixture:     "chromeVideoWithSWDecoding",
}, {
	Name:        "vp8_1080p_60fps_sw",
	FileName:    "1080p_60fps_600frames.vp8.webm",
	DecoderType: "playback.Software",
	ExtraData:   []string{"1080p_60fps_600frames.vp8.webm"},
	Fixture:     "chromeVideoWithSWDecoding",
}, {
	Name:        "vp9_480p_30fps_sw",
	FileName:    "480p_30fps_300frames.vp9.webm",
	DecoderType: "playback.Software",
	ExtraAttr:   []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:   []string{"480p_30fps_300frames.vp9.webm"},
	Fixture:     "chromeVideoWithSWDecoding",
}, {
	Name:        "vp9_720p_30fps_sw",
	FileName:    "720p_30fps_300frames.vp9.webm",
	DecoderType: "playback.Software",
	ExtraAttr:   []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:   []string{"720p_30fps_300frames.vp9.webm"},
	Fixture:     "chromeVideoWithSWDecoding",
}, {
	Name:        "vp9_1080p_30fps_sw",
	FileName:    "1080p_30fps_300frames.vp9.webm",
	DecoderType: "playback.Software",
	ExtraData:   []string{"1080p_30fps_300frames.vp9.webm"},
	Fixture:     "chromeVideoWithSWDecoding",
}, {
	Name:        "vp9_1080p_60fps_sw",
	FileName:    "1080p_60fps_600frames.vp9.webm",
	DecoderType: "playback.Software",
	ExtraData:   []string{"1080p_60fps_600frames.vp9.webm"},
	Fixture:     "chromeVideoWithSWDecoding",
}, {
	Name:        "av1_480p_30fps_sw",
	FileName:    "480p_30fps_300frames.av1.mp4",
	DecoderType: "playback.Software",
	ExtraData:   []string{"480p_30fps_300frames.av1.mp4"},
	Fixture:     "chromeVideoWithSWDecoding",
}, {
	Name:        "av1_720p_30fps_sw",
	FileName:    "720p_30fps_300frames.av1.mp4",
	DecoderType: "playback.Software",
	ExtraData:   []string{"720p_30fps_300frames.av1.mp4"},
	Fixture:     "chromeVideoWithSWDecoding",
}, {
	Name:        "av1_720p_60fps_sw",
	FileName:    "720p_60fps_600frames.av1.mp4",
	DecoderType: "playback.Software",
	ExtraData:   []string{"720p_60fps_600frames.av1.mp4"},
	Fixture:     "chromeVideoWithSWDecoding",
}, {
	Name:        "av1_1080p_30fps_sw",
	FileName:    "1080p_30fps_300frames.av1.mp4",
	DecoderType: "playback.Software",
	ExtraData:   []string{"1080p_30fps_300frames.av1.mp4"},
	Fixture:     "chromeVideoWithSWDecoding",
}, {
	Name:        "av1_1080p_60fps_sw",
	FileName:    "1080p_60fps_600frames.av1.mp4",
	DecoderType: "playback.Software",
	ExtraData:   []string{"1080p_60fps_600frames.av1.mp4"},
	Fixture:     "chromeVideoWithSWDecoding",
}, {
	Name:        "av1_2160p_30fps_sw",
	FileName:    "2160p_30fps_300frames.av1.mp4",
	DecoderType: "playback.Software",
	ExtraData:   []string{"2160p_30fps_300frames.av1.mp4"},
	Fixture:     "chromeVideoWithSWDecoding",
}, {
	Name:        "av1_2160p_60fps_sw",
	FileName:    "2160p_60fps_600frames.av1.mp4",
	DecoderType: "playback.Software",
	ExtraData:   []string{"2160p_60fps_600frames.av1.mp4"},
	Fixture:     "chromeVideoWithSWDecoding",
}, {
	Name:              "av1_480p_30fps_sw_gav1",
	FileName:          "480p_30fps_300frames.av1.mp4",
	DecoderType:       "playback.LibGAV1",
	ExtraSoftwareDeps: []string{`"arm"`},
	ExtraData:         []string{"480p_30fps_300frames.av1.mp4"},
	Fixture:           "chromeVideoWithSWDecodingAndLibGAV1",
}, {
	Name:              "av1_720p_30fps_sw_gav1",
	FileName:          "720p_30fps_300frames.av1.mp4",
	DecoderType:       "playback.LibGAV1",
	ExtraSoftwareDeps: []string{`"arm"`},
	ExtraData:         []string{"720p_30fps_300frames.av1.mp4"},
	Fixture:           "chromeVideoWithSWDecodingAndLibGAV1",
}, {
	Name:              "av1_720p_60fps_sw_gav1",
	FileName:          "720p_60fps_600frames.av1.mp4",
	DecoderType:       "playback.LibGAV1",
	ExtraSoftwareDeps: []string{`"arm"`},
	ExtraData:         []string{"720p_60fps_600frames.av1.mp4"},
	Fixture:           "chromeVideoWithSWDecodingAndLibGAV1",
}, {
	Name:              "av1_1080p_30fps_sw_gav1",
	FileName:          "1080p_30fps_300frames.av1.mp4",
	DecoderType:       "playback.LibGAV1",
	ExtraSoftwareDeps: []string{`"arm"`},
	ExtraData:         []string{"1080p_30fps_300frames.av1.mp4"},
	Fixture:           "chromeVideoWithSWDecodingAndLibGAV1",
}, {
	Name:              "av1_1080p_60fps_sw_gav1",
	FileName:          "1080p_60fps_600frames.av1.mp4",
	DecoderType:       "playback.LibGAV1",
	ExtraSoftwareDeps: []string{`"arm"`},
	ExtraData:         []string{"1080p_60fps_600frames.av1.mp4"},
	Fixture:           "chromeVideoWithSWDecodingAndLibGAV1",
}, {
	Name:              "h264_1080p_60fps_hw_alt",
	FileName:          "1080p_60fps_600frames.h264.mp4",
	DecoderType:       "playback.Hardware",
	ExtraSoftwareDeps: []string{"caps.HWDecodeH264_60", `"video_decoder_legacy_supported"`, `"proprietary_codecs"`},
	ExtraData:         []string{"1080p_60fps_600frames.h264.mp4"},
	Fixture:           "chromeAlternateVideoDecoder",
}, {
	Name:              "vp8_1080p_60fps_hw_alt",
	FileName:          "1080p_60fps_600frames.vp8.webm",
	DecoderType:       "playback.Hardware",
	ExtraData:         []string{"1080p_60fps_600frames.vp8.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP8_60", `"video_decoder_legacy_supported"`},
	Fixture:           "chromeAlternateVideoDecoder",
}, {
	Name:              "vp9_1080p_60fps_hw_alt",
	FileName:          "1080p_60fps_600frames.vp9.webm",
	DecoderType:       "playback.Hardware",
	ExtraData:         []string{"1080p_60fps_600frames.vp9.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9_60", `"video_decoder_legacy_supported"`},
	Fixture:           "chromeAlternateVideoDecoder",
}, {
	Name:              "vp9_2160p_60fps_hw_alt",
	FileName:          "2160p_60fps_600frames.vp9.webm",
	DecoderType:       "playback.Hardware",
	ExtraData:         []string{"2160p_60fps_600frames.vp9.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9_4K60", `"video_decoder_legacy_supported"`},
	Fixture:           "chromeAlternateVideoDecoder",
}}

func TestPlaybackPerf(t *testing.T) {
	code := genparams.Template(t, `{{ range . }}{
	Name: {{ .Name | fmt }},
	Val: playbackPerfParams{
		fileName: {{ .FileName | fmt }},
		decoderType: {{ .DecoderType }},
		chromeType: lacros.ChromeTypeChromeOS,
	},
	{{ if .ExtraSoftwareDeps }}
	ExtraSoftwareDeps: []string{ {{ range .ExtraSoftwareDeps }} {{ . }}, {{ end }} },
	{{ end }}
	{{ if .ExtraAttr }}
	ExtraAttr: {{ .ExtraAttr | fmt }},
	{{ end }}
	ExtraData: {{ .ExtraData | fmt }},
	Fixture: {{ .Fixture | fmt }},
}, {
	Name: {{ .Name | printf "\"%s_lacros\"" }},
	Val: playbackPerfParams{
		fileName: {{ .FileName | fmt }},
		decoderType: {{ .DecoderType }},
		chromeType: lacros.ChromeTypeLacros,
	},
	ExtraSoftwareDeps: []string{ {{ if .ExtraSoftwareDeps }} {{ range .ExtraSoftwareDeps }} {{ . }}, {{ end }} {{ end }} "lacros" },
	{{ if .ExtraAttr }}
	ExtraAttr: {{ .ExtraAttr | fmt }},
	{{ end }}
	ExtraData: []string{ {{ if .ExtraData }} {{ range .ExtraData }} {{ . | fmt }}, {{ end }} {{ end }} launcher.DataArtifact },
	Fixture: {{ .Fixture | printf "\"%sLacros\"" }},
}, {{ end }}`, params)
	genparams.Ensure(t, "playback_perf.go", code)
}
