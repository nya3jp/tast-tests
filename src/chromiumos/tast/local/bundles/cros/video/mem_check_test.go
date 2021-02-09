// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"testing"

	"chromiumos/tast/common/genparams"
)

var memCheckTestParams = []struct {
	Name              string
	FileName          string
	Sizes             string
	VideoType         string
	MSEDataFiles      bool
	ExtraSoftwareDeps []string
	ExtraAttr         []string
	ExtraData         []string
	Fixture           string
}{{
	Name:              "av1_hw",
	FileName:          "720_av1.mp4",
	Sizes:             "[]graphics.Size{{Width: 1280, Height: 720}}",
	VideoType:         "play.NormalVideo",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"video.html", "720_av1.mp4"},
	ExtraSoftwareDeps: []string{`"amd64"`, `"video_overlays"`, "caps.HWDecodeAV1"},
	Fixture:           "chromeVideoWithGuestLoginAndHWAV1Decoding",
}, {
	Name:              "h264_hw",
	FileName:          "720_h264.mp4",
	Sizes:             "[]graphics.Size{{Width: 1280, Height: 720}}",
	VideoType:         "play.NormalVideo",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"video.html", "720_h264.mp4"},
	ExtraSoftwareDeps: []string{`"amd64"`, `"video_overlays"`, "caps.HWDecodeH264", `"proprietary_codecs"`},
	Fixture:           "chromeVideoWithGuestLogin",
}, {
	Name:              "vp8_hw",
	FileName:          "720_vp8.webm",
	Sizes:             "[]graphics.Size{{Width: 1280, Height: 720}}",
	VideoType:         "play.NormalVideo",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"video.html", "720_vp8.webm"},
	ExtraSoftwareDeps: []string{`"amd64"`, `"video_overlays"`, "caps.HWDecodeVP8"},
	Fixture:           "chromeVideoWithGuestLogin",
}, {
	Name:              "vp9_hw",
	FileName:          "720_vp9.webm",
	Sizes:             "[]graphics.Size{{Width: 1280, Height: 720}}",
	VideoType:         "play.NormalVideo",
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"video.html", "720_vp9.webm"},
	ExtraSoftwareDeps: []string{`"amd64"`, `"video_overlays"`, "caps.HWDecodeVP9"},
	Fixture:           "chromeVideoWithGuestLogin",
}, {
	Name:              "av1_hw_switch",
	FileName:          "dash_smpte_av1.mp4.mpd",
	Sizes:             "[]graphics.Size{{Width: 256, Height: 144}, {Width: 426, Height: 240}}",
	VideoType:         "play.MSEVideo",
	MSEDataFiles:      true,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"dash_smpte_av1.mp4.mpd", "dash_smpte_144.av1.mp4", "dash_smpte_240.av1.mp4"},
	ExtraSoftwareDeps: []string{`"amd64"`, `"video_overlays"`, "caps.HWDecodeAV1"},
	Fixture:           "chromeVideoWithGuestLoginAndHWAV1Decoding",
}, {
	Name:              "h264_hw_switch",
	FileName:          "cars_dash_mp4.mpd",
	Sizes:             "[]graphics.Size{{Width: 256, Height: 144}, {Width: 426, Height: 240}}",
	VideoType:         "play.MSEVideo",
	MSEDataFiles:      true,
	ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
	ExtraData:         []string{"cars_dash_mp4.mpd", "cars_144_h264.mp4", "cars_240_h264.mp4"},
	ExtraSoftwareDeps: []string{`"amd64"`, `"video_overlays"`, "caps.HWDecodeH264", `"proprietary_codecs"`},
	Fixture:           "chromeVideoWithGuestLogin",
}}

func TestMemCheck(t *testing.T) {
	code := genparams.Template(t, `{{ range . }}{
	Name: {{ .Name | fmt }},
	Val: memCheckParams{
		fileName: {{ .FileName | fmt }},
		sizes: {{ .Sizes }},
		videoType: {{ .VideoType }},
		chromeType: lacros.ChromeTypeChromeOS,
	},
	{{ if .ExtraAttr }}
	ExtraAttr: {{ .ExtraAttr | fmt }},
	{{ end }}
	{{ if .MSEDataFiles }}
	ExtraData: append(play.MSEDataFiles(), {{ range .ExtraData }} {{ . | fmt }}, {{ end }}),
	{{ else }}
	ExtraData: {{ .ExtraData | fmt }},
  {{ end }}
	ExtraSoftwareDeps: []string{ {{ range .ExtraSoftwareDeps }} {{ . }}, {{ end }} },
	Fixture: {{ .Fixture | fmt }},
}, {
	Name: {{ .Name | printf "\"%s_lacros\"" }},
	Val: memCheckParams{
		fileName: {{ .FileName | fmt }},
		sizes: {{ .Sizes }},
		videoType: {{ .VideoType }},
		chromeType: lacros.ChromeTypeLacros,
	},
	{{ if .ExtraAttr }}
	ExtraAttr: {{ .ExtraAttr | fmt }},
	{{ end }}
	{{ if .MSEDataFiles }}
	ExtraData: append(play.MSEDataFiles(), {{ range .ExtraData }} {{ . | fmt }}, {{ end }} launcher.DataArtifact),
	{{ else }}
	ExtraData: []string{ {{ if .ExtraData }} {{ range .ExtraData }} {{ . | fmt }}, {{ end }} {{ end }} launcher.DataArtifact },
  {{ end }}
	ExtraSoftwareDeps: []string{ {{ range .ExtraSoftwareDeps }} {{ . }}, {{ end }} "lacros" },
	Fixture: {{ .Fixture | printf "\"%sLacros\"" }},
}, {{ end }}`, memCheckTestParams)
	genparams.Ensure(t, "mem_check.go", code)
}
