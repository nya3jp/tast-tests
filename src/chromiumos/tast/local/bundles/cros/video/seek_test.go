// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"testing"

	"chromiumos/tast/common/genparams"
)

var seekTestParams = []struct {
	Name              string
	FileName          string
	NumSeeks          int
	ExtraSoftwareDeps []string
	ExtraAttr         []string
	ExtraData         []string
	Timeout           string
	Fixture           string
}{{
	Name:     "av1",
	FileName: "720_av1.mp4", NumSeeks: 25,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"720_av1.mp4"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeAV1"},
	Fixture:           "chromeVideoWithHWAV1Decoding",
}, {
	Name:     "h264",
	FileName: "720_h264.mp4", NumSeeks: 25,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"720_h264.mp4"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeH264", `"proprietary_codecs"`},
	Fixture:           "chromeVideo",
}, {
	Name:     "vp8",
	FileName: "720_vp8.webm", NumSeeks: 25,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"720_vp8.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP8"},
	Fixture:           "chromeVideo",
}, {
	Name:     "vp9",
	FileName: "720_vp9.webm", NumSeeks: 25,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"720_vp9.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9"},
	Fixture:           "chromeVideo",
}, {
	Name:     "switch_av1",
	FileName: "smpte_bars_resolution_ladder.av1.webm", NumSeeks: 25,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"smpte_bars_resolution_ladder.av1.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeAV1"},
	Fixture:           "chromeVideoWithHWAV1Decoding",
}, {
	Name:     "switch_h264",
	FileName: "smpte_bars_resolution_ladder.h264.mp4", NumSeeks: 25,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"smpte_bars_resolution_ladder.h264.mp4"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeH264", `"proprietary_codecs"`},
	Fixture:           "chromeVideo",
}, {
	Name:     "switch_vp8",
	FileName: "smpte_bars_resolution_ladder.vp8.webm", NumSeeks: 25,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"smpte_bars_resolution_ladder.vp8.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP8"},
	Fixture:           "chromeVideo",
}, {
	Name:     "switch_vp9",
	FileName: "smpte_bars_resolution_ladder.vp9.webm", NumSeeks: 25,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"smpte_bars_resolution_ladder.vp9.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9"},
	Fixture:           "chromeVideo",
}, {
	Name:     "stress_av1",
	FileName: "720_av1.mp4", NumSeeks: 1000,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
	ExtraData:         []string{"720_av1.mp4"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeAV1"},
	Timeout:           "20 * time.Minute",
	Fixture:           "chromeVideoWithHWAV1Decoding",
}, {
	Name:     "stress_vp8",
	FileName: "720_vp8.webm", NumSeeks: 1000,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
	ExtraData:         []string{"720_vp8.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP8"},
	Timeout:           "20 * time.Minute",
	Fixture:           "chromeVideo",
}, {
	Name:     "stress_vp9",
	FileName: "720_vp9.webm", NumSeeks: 1000,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
	ExtraData:         []string{"720_vp9.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9"},
	Timeout:           "20 * time.Minute",
	Fixture:           "chromeVideo",
}, {
	Name:     "stress_h264",
	FileName: "720_h264.mp4", NumSeeks: 1000,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
	ExtraData:         []string{"720_h264.mp4"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeH264", `"proprietary_codecs"`},
	Timeout:           "20 * time.Minute",
	Fixture:           "chromeVideo",
}, {
	Name:     "h264_alt",
	FileName: "720_h264.mp4", NumSeeks: 25,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"720_h264.mp4"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeH264", `"video_decoder_legacy_supported"`, `"proprietary_codecs"`},
	Fixture:           "chromeAlternateVideoDecoder",
}, {
	Name:     "vp8_alt",
	FileName: "720_vp8.webm", NumSeeks: 25,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"720_vp8.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP8", `"video_decoder_legacy_supported"`},
	Fixture:           "chromeAlternateVideoDecoder",
}, {
	Name:     "vp9_alt",
	FileName: "720_vp9.webm", NumSeeks: 25,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"720_vp9.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9", `"video_decoder_legacy_supported"`},
	Fixture:           "chromeAlternateVideoDecoder",
}, {
	Name:     "switch_h264_alt",
	FileName: "smpte_bars_resolution_ladder.h264.mp4", NumSeeks: 25,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"smpte_bars_resolution_ladder.h264.mp4"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeH264", `"video_decoder_legacy_supported"`, `"proprietary_codecs"`},
	Fixture:           "chromeAlternateVideoDecoder",
}, {
	Name:     "switch_vp8_alt",
	FileName: "smpte_bars_resolution_ladder.vp8.webm", NumSeeks: 25,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"smpte_bars_resolution_ladder.vp8.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP8", `"video_decoder_legacy_supported"`},
	Fixture:           "chromeAlternateVideoDecoder",
}, {
	Name:     "switch_vp9_alt",
	FileName: "smpte_bars_resolution_ladder.vp9.webm", NumSeeks: 25,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
	ExtraData:         []string{"smpte_bars_resolution_ladder.vp9.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9", `"video_decoder_legacy_supported"`},
	Fixture:           "chromeAlternateVideoDecoder",
}, {
	Name:     "stress_vp8_alt",
	FileName: "720_vp8.webm", NumSeeks: 1000,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
	ExtraData:         []string{"720_vp8.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP8", `"video_decoder_legacy_supported"`},
	Timeout:           "20 * time.Minute",
	Fixture:           "chromeAlternateVideoDecoder",
}, {
	Name:     "stress_vp9_alt",
	FileName: "720_vp9.webm", NumSeeks: 1000,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
	ExtraData:         []string{"720_vp9.webm"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeVP9", `"video_decoder_legacy_supported"`},
	Timeout:           "20 * time.Minute",
	Fixture:           "chromeAlternateVideoDecoder",
}, {
	Name:     "stress_h264_alt",
	FileName: "720_h264.mp4", NumSeeks: 1000,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
	ExtraData:         []string{"720_h264.mp4"},
	ExtraSoftwareDeps: []string{"caps.HWDecodeH264", `"video_decoder_legacy_supported"`, `"proprietary_codecs"`},
	Timeout:           "20 * time.Minute",
	Fixture:           "chromeAlternateVideoDecoder",
}}

func TestSeek(t *testing.T) {
	code := genparams.Template(t, `{{ range . }}{
	Name: {{ .Name | fmt }},
	Val: seekTest{
		filename: {{ .FileName | fmt }},
		numSeeks: {{ .NumSeeks }},
		chromeType: lacros.ChromeTypeChromeOS,
	},
	{{ if .ExtraAttr }}
	ExtraAttr: {{ .ExtraAttr | fmt }},
	{{ end }}
	ExtraData: {{ .ExtraData | fmt }},
	{{ if .ExtraSoftwareDeps }}
	ExtraSoftwareDeps: []string{ {{ range .ExtraSoftwareDeps }} {{ . }}, {{ end }} },
	{{ end }}
	{{ if .Timeout }}
	Timeout: {{ .Timeout }},
	{{ end }}
	Fixture: {{ .Fixture | fmt }},
}, {
	Name: {{ .Name | printf "\"%s_lacros\"" }},
	Val: seekTest{
		filename: {{ .FileName | fmt }},
		numSeeks: {{ .NumSeeks }},
		chromeType: lacros.ChromeTypeLacros,
	},
	{{ if .ExtraAttr }}
	ExtraAttr: {{ .ExtraAttr | fmt }},
	{{ end }}
	ExtraData: []string{ {{ if .ExtraData }} {{ range .ExtraData }} {{ . | fmt }}, {{ end }} {{ end }} launcher.DataArtifact },
	ExtraSoftwareDeps: []string{ {{ range .ExtraSoftwareDeps }} {{ . }}, {{ end }} "lacros" },
	{{ if .Timeout }}
	Timeout: {{ .Timeout }},
	{{ end }}
	Fixture: {{ .Fixture | printf "\"%sLacros\"" }},
}, {{ end }}`, seekTestParams)
	genparams.Ensure(t, "seek.go", code)
}
