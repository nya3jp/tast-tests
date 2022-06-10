// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"fmt"
	"strings"
	"testing"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/bundles/cros/video/playback"
)

// codec, fps, resolution
var playbackPerfFile = map[string]map[int]map[int]string{
	"h264": {
		30: {
			144:  "perf/h264/144p_30fps_300frames.h264.mp4",
			240:  "perf/h264/240p_30fps_300frames.h264.mp4",
			360:  "perf/h264/360p_30fps_300frames.h264.mp4",
			480:  "perf/h264/480p_30fps_300frames.h264.mp4",
			720:  "perf/h264/720p_30fps_300frames.h264.mp4",
			1080: "perf/h264/1080p_30fps_300frames.h264.mp4",
			2160: "perf/h264/2160p_30fps_300frames.h264.mp4",
		},
		60: {
			1080: "perf/h264/1080p_60fps_600frames.h264.mp4",
			2160: "perf/h264/2160p_60fps_600frames.h264.mp4",
		},
	},
	"vp8": {
		30: {
			144:  "perf/vp8/144p_30fps_300frames.vp8.webm",
			240:  "perf/vp8/240p_30fps_300frames.vp8.webm",
			360:  "perf/vp8/360p_30fps_300frames.vp8.webm",
			480:  "perf/vp8/480p_30fps_300frames.vp8.webm",
			720:  "perf/vp8/720p_30fps_300frames.vp8.webm",
			1080: "perf/vp8/1080p_30fps_300frames.vp8.webm",
			2160: "perf/vp8/2160p_30fps_300frames.vp8.webm",
		},
		60: {
			1080: "perf/vp8/1080p_60fps_600frames.vp8.webm",
			2160: "perf/vp8/2160p_60fps_600frames.vp8.webm",
		},
	},
	"vp9": {
		30: {
			144:  "perf/vp9/144p_30fps_300frames.vp9.webm",
			240:  "perf/vp9/240p_30fps_300frames.vp9.webm",
			360:  "perf/vp9/360p_30fps_300frames.vp9.webm",
			480:  "perf/vp9/480p_30fps_300frames.vp9.webm",
			720:  "perf/vp9/720p_30fps_300frames.vp9.webm",
			1080: "perf/vp9/1080p_30fps_300frames.vp9.webm",
			2160: "perf/vp9/2160p_30fps_300frames.vp9.webm",
		},
		60: {
			1080: "perf/vp9/1080p_60fps_600frames.vp9.webm",
			2160: "perf/vp9/2160p_60fps_600frames.vp9.webm",
		},
	},
	"hevc": {
		30: {
			144:  "perf/hevc/144p_30fps_300frames.hevc.mp4",
			240:  "perf/hevc/240p_30fps_300frames.hevc.mp4",
			360:  "perf/hevc/360p_30fps_300frames.hevc.mp4",
			480:  "perf/hevc/480p_30fps_300frames.hevc.mp4",
			720:  "perf/hevc/720p_30fps_300frames.hevc.mp4",
			1080: "perf/hevc/1080p_30fps_300frames.hevc.mp4",
			2160: "perf/hevc/2160p_30fps_300frames.hevc.mp4",
		},
		60: {
			1080: "perf/hevc/1080p_60fps_600frames.hevc.mp4",
			2160: "perf/hevc/2160p_60fps_600frames.hevc.mp4",
		},
	},
	"av1": {
		30: {
			144:  "perf/av1/144p_30fps_300frames.av1.mp4",
			240:  "perf/av1/240p_30fps_300frames.av1.mp4",
			360:  "perf/av1/360p_30fps_300frames.av1.mp4",
			480:  "perf/av1/480p_30fps_300frames.av1.mp4",
			720:  "perf/av1/720p_30fps_300frames.av1.mp4",
			1080: "perf/av1/1080p_30fps_300frames.av1.mp4",
			2160: "perf/av1/2160p_30fps_300frames.av1.mp4",
		},
		60: {
			720:  "perf/av1/720p_60fps_600frames.av1.mp4",
			1080: "perf/av1/1080p_60fps_600frames.av1.mp4",
			2160: "perf/av1/2160p_60fps_600frames.av1.mp4",
		},
	},
}

// codec
var playbackPerfLongFile = map[string]string{
	"h264": "crosvideo/1080.mp4",
	"vp8":  "crosvideo/1080_vp8.webm",
	"vp9":  "crosvideo/1080.webm",
	"av1":  "crosvideo/av1_1080p_30fps.mp4",
}

// fps
var playbackPerfHEVC104KFile = map[int]string{
	30: "perf/hevc10/2160p_30fps_300frames.hevc10.mp4",
	60: "perf/hevc10/2160p_60fps_600frames.hevc10.mp4",
}

func getPlaybackPerfSoftDeps(codec string, resolution, fps int, dec string) []string {
	var deps []string
	if codec == "h264" || codec == "hevc" || codec == "hevc10" {
		deps = append(deps, "\"proprietary_codecs\"")
	}

	if dec != "hw" {
		return deps
	}

	switch codec {
	case "h264":
		if fps == 60 {
			if resolution >= 2160 {
				deps = append(deps, "caps.HWDecodeH264_4K60")
			} else {
				deps = append(deps, "caps.HWDecodeH264_60")
			}
		} else {
			if resolution >= 2160 {
				deps = append(deps, "caps.HWDecodeH264_4K")
			} else {
				deps = append(deps, "caps.HWDecodeH264")
			}
		}
	case "vp8":
		if fps == 60 {
			if resolution >= 2160 {
				deps = append(deps, "caps.HWDecodeVP8_4K60")
			} else {
				deps = append(deps, "caps.HWDecodeVP8_60")
			}
		} else {
			if resolution >= 2160 {
				deps = append(deps, "caps.HWDecodeVP8_4K")
			} else {
				deps = append(deps, "caps.HWDecodeVP8")
			}
		}
	case "vp9":
		if fps == 60 {
			if resolution >= 2160 {
				deps = append(deps, "caps.HWDecodeVP9_4K60")
			} else {
				deps = append(deps, "caps.HWDecodeVP9_60")
			}
		} else {
			if resolution >= 2160 {
				deps = append(deps, "caps.HWDecodeVP9_4K")
			} else {
				deps = append(deps, "caps.HWDecodeVP9")
			}
		}
	case "hevc":
		if fps == 60 {
			if resolution >= 2160 {
				deps = append(deps, "caps.HWDecodeHEVC4K60")
			} else {
				deps = append(deps, "caps.HWDecodeHEVC60")
			}
		} else {
			if resolution >= 2160 {
				deps = append(deps, "caps.HWDecodeHEVC4K")
			} else {
				deps = append(deps, "caps.HWDecodeHEVC")
			}
		}
	case "hevc10":
		if resolution >= 2160 {
			if fps == 60 {
				deps = append(deps, "caps.HWDecodeHEVC4K60_10BPP")
			} else {
				deps = append(deps, "caps.HWDecodeHEVC4K10BPP")
			}
		} else {
			panic("Unsupported test")
		}
	case "av1":
		if fps == 60 {
			if resolution >= 2160 {
				deps = append(deps, "caps.HWDecodeAV1_4K60")
			} else {
				deps = append(deps, "caps.HWDecodeAV1_60")
			}
		} else {
			if resolution >= 2160 {
				deps = append(deps, "caps.HWDecodeAV1_4K")
			} else {
				deps = append(deps, "caps.HWDecodeAV1")
			}
		}
	}

	return deps
}

type playbackParamData struct {
	Name string

	// playbackPerfParams
	File             string
	DecoderType      playback.DecoderType
	BrowserType      string
	GridSize         int
	MeasureRoughness bool

	SoftwareDeps string
	HardwareDeps string
	Data         []string
	Attr         []string
	Fixture      string
}

func genPlaybackParam(codec, file string, resolution, fps int, dec, nameSuffix, fixture string, extendDeps []string) playbackParamData {
	if file == "" {
		fmt.Println(codec, resolution, fps, dec, nameSuffix)
		panic("file is empty")
	}
	testName := fmt.Sprintf("%s_%dp_%dfps_%s", codec, resolution, fps, dec)
	if nameSuffix != "" {
		testName += "_" + nameSuffix
	}
	decType := playback.Hardware
	if dec == "sw" {
		decType = playback.Software
	} else if dec == "sw_gav1" {
		decType = playback.LibGAV1
	}
	if fixture == "" {
		if dec == "hw" {
			if codec == "av1" {
				// TODO(hiroh): Remove this as av1 hw decoder has been enabled
				// for a long time.
				fixture = "chromeVideoWithHWAV1Decoding"
			} else {
				fixture = "chromeVideo"
			}
		} else {
			fixture = "chromeVideoWithSWDecoding"
		}
	}

	brwType := "browser.TypeAsh"
	if nameSuffix == "lacros" {
		brwType = "browser.TypeLacros"
	}
	deps := getPlaybackPerfSoftDeps(codec, resolution, fps, dec)
	if len(extendDeps) > 0 {
		deps = append(deps, extendDeps...)
	}

	var attr []string
	// Run nightly for av1 playback perf tests for consistency.
	if codec != "av1" && resolution < 1080 {
		attr = []string{"group:graphics", "graphics_video", "graphics_nightly"}
	}
	return playbackParamData{
		Name:         testName,
		File:         file,
		DecoderType:  decType,
		BrowserType:  brwType,
		SoftwareDeps: "[]string{" + strings.Join(deps, ",") + "}",
		Data:         []string{file},
		Attr:         attr,
		Fixture:      fixture,
	}
}

func TestPlaybackPerfParams(t *testing.T) {
	var params []playbackParamData

	codecs := []string{"h264", "vp8", "vp9", "hevc"}
	for _, codec := range codecs {
		for _, resolution := range []int{144, 240, 360, 480, 720, 1080, 2160} {
			fpss := []int{30}
			if resolution >= 1080 {
				fpss = append(fpss, 60)
			}
			decs := []string{"hw"}
			if codec != "hevc" && resolution >= 480 {
				decs = append(decs, "sw")
			}
			for _, fps := range fpss {
				for _, dec := range decs {
					file := playbackPerfFile[codec][fps][resolution]
					params = append(params,
						genPlaybackParam(codec, file, resolution, fps, dec,
							"", "", []string{}))
				}
			}
		}
	}
	// AV1
	for _, resolution := range []int{480, 720, 1080, 2160} {
		codec := "av1"
		fpss := []int{30}
		if resolution >= 720 {
			fpss = append(fpss, 60)
		}
		for _, fps := range fpss {
			for _, dec := range []string{"hw", "sw"} {
				file := playbackPerfFile[codec][fps][resolution]
				params = append(params,
					genPlaybackParam(codec, file, resolution, fps, dec,
						"", "", []string{}))
			}
		}
	}
	// HEVC10
	for _, fps := range []int{30, 60} {
		codec, resolution, dec := "hevc10", 2160, "hw"
		file := playbackPerfHEVC104KFile[fps]
		params = append(params,
			genPlaybackParam(codec, file, resolution, fps, dec,
				"", "", []string{}))
	}
	// Alt
	for _, codec := range []string{"h264", "vp8", "vp9"} {
		resolutions := []int{1080}
		if codec == "vp9" {
			resolutions = append(resolutions, 2160)
		}
		for _, resolution := range resolutions {
			fps, dec := 60, "hw"
			file := playbackPerfFile[codec][fps][resolution]
			params = append(params,
				genPlaybackParam(codec, file, resolution, fps, dec,
					"alt", "chromeAlternateVideoDecoder",
					[]string{"\"video_decoder_legacy_supported\""}))
		}
	}
	// long
	for _, codec := range []string{"h264", "vp8", "vp9", "av1"} {
		for _, dec := range []string{"hw", "sw"} {
			resolution, fps := 1080, 30
			file := playbackPerfLongFile[codec]
			param := genPlaybackParam(codec, file, resolution, fps, dec,
				"long", "", []string{"\"drm_atomic\""})

			param.HardwareDeps = "hwdep.SkipOnModel(\"hana\", \"elm\"), hwdep.InternalDisplay()"
			param.MeasureRoughness = true
			params = append(params, param)
		}
	}

	// grid
	for _, codec := range []string{"h264", "vp8", "vp9", "av1"} {
		resolution, fps, dec := 720, 30, "hw"
		file := playbackPerfFile[codec][fps][resolution]
		param := genPlaybackParam(codec, file, resolution, fps, dec,
			"3x3", "", []string{})
		param.GridSize = 3
		params = append(params, param)
	}

	// lacros
	for _, codec := range []string{"h264", "vp9"} {
		resolution, fps, dec := 720, 30, "hw"
		file := playbackPerfFile[codec][fps][resolution]
		params = append(params,
			genPlaybackParam(codec, file, resolution, fps, dec,
				"lacros", "chromeVideoLacros", []string{"\"lacros\""}))
	}
	// libgav1
	for _, resolution := range []int{480, 720, 1080} {
		codec := "av1"
		fpss := []int{30}
		if resolution >= 720 {
			fpss = append(fpss, 60)
		}
		for _, fps := range fpss {
			dec := "sw_gav1"
			file := playbackPerfFile[codec][fps][resolution]
			params = append(params,
				genPlaybackParam(codec, file, resolution, fps, dec,
					"", "chromeVideoWithSWDecodingAndLibGAV1", []string{"\"arm\""}))
		}
	}

	code := genparams.Template(t, `{{ range . }}{
		Name: {{ .Name | fmt }},
		Val:  playbackPerfParams{
			fileName: {{ .File | fmt }},
			decoderType: {{ .DecoderType }},
			browserType: {{ .BrowserType }},
			{{ if .GridSize }}
			gridSize: {{ .GridSize | fmt }},
			{{ end }}
			{{ if .MeasureRoughness }}
			measureRoughness: {{ .MeasureRoughness | fmt }},
			{{ end }}
		},
		{{ if .HardwareDeps }}
		ExtraHardwareDeps: hwdep.D({{ .HardwareDeps }}),
		{{ end }}
		{{ if .SoftwareDeps }}
		ExtraSoftwareDeps: {{ .SoftwareDeps }},
		{{ end }}
		ExtraData: {{ .Data | fmt }},
		{{ if .Attr }}
		ExtraAttr: {{ .Attr | fmt }},
		{{ end }}
		Fixture: {{ .Fixture | fmt }},
	},
	{{ end }}`, params)

	genparams.Ensure(t, "playback_perf.go", code)
}
