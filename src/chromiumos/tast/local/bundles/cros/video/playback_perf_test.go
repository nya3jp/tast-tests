// Copyright 2022 The ChromiumOS Authors
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

// To regenerate the test parameters by running the following in a chroot:
// TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 chromiumos/tast/local/bundles/cros/video

// codec
var playbackPerfLongFile = map[string]string{
	"h264": "crosvideo/1080.mp4",
	"vp8":  "crosvideo/1080_vp8.webm",
	"vp9":  "crosvideo/1080.webm",
	"av1":  "crosvideo/av1_1080p_30fps.mp4",
}

// TODO(hiroh): genPlaybackPerfDataPath() and genPlaybackPerfSwDeps() can be
// reused by other parameter generator code. Put the functions in common places.

func genPlaybackPerfDataPath(codec string, resolution, fps int) string {
	if fps != 30 && fps != 60 {
		panic("Unexpected fps")
	}
	numFrames := fps * 10

	extension := ""
	switch codec {
	case "h264", "hevc", "hevc10", "av1":
		extension = codec + ".mp4"
	case "vp8", "vp9":
		extension = codec + ".webm"
	default:
		panic("Unexpected codec")
	}

	return fmt.Sprintf("perf/%s/%dp_%dfps_%dframes.%s",
		codec, resolution, fps, numFrames, extension)
}

func genPlaybackPerfSwDeps(codec string, resolution, fps int, dec string) []string {
	var swDeps []string

	if codec == "h264" || codec == "hevc" || codec == "hevc10" {
		swDeps = append(swDeps, "proprietary_codecs")
	}

	if dec != "hw" {
		return swDeps
	}

	if resolution < 1080 {
		resolution = 1080
	}
	if codec == "hevc10" {
		swDeps = append(swDeps, fmt.Sprintf("autotest-capability:hw_dec_hevc_%d_%d_10bpp", resolution, fps))
	} else {
		swDeps = append(swDeps, fmt.Sprintf("autotest-capability:hw_dec_%s_%d_%d", codec, resolution, fps))
	}
	return swDeps
}

type playbackParamData struct {
	Name string

	// playbackPerfParams
	File             string
	DecoderType      playback.DecoderType
	BrowserType      string
	GridWidth        int
	GridHeight       int
	PerfTracing      bool
	MeasureRoughness bool

	SoftwareDeps []string
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
	if strings.Contains(nameSuffix, "lacros") {
		brwType = "browser.TypeLacros"
	}
	deps := genPlaybackPerfSwDeps(codec, resolution, fps, dec)
	if len(extendDeps) > 0 {
		deps = append(deps, extendDeps...)
	}

	return playbackParamData{
		Name:         testName,
		File:         file,
		DecoderType:  decType,
		BrowserType:  brwType,
		SoftwareDeps: deps,
		Data:         []string{file},
		Fixture:      fixture,
	}
}

func TestPlaybackPerfParams(t *testing.T) {
	var params []playbackParamData

	codecs := []string{"h264", "vp8", "vp9", "av1", "hevc"}
	for _, codec := range codecs {
		for _, resolution := range []int{720, 1080, 2160, 4320} {
			if resolution >= 4320 && codec == "vp8" {
				// VP8 does not support 8K.
				continue
			}
			fpss := []int{30}
			if resolution >= 1080 {
				fpss = append(fpss, 60)
			}
			decs := []string{"hw"}
			if codec != "hevc" && resolution <= 2160 {
				decs = append(decs, "sw")
			}
			for _, fps := range fpss {
				for _, dec := range decs {
					params = append(params,
						genPlaybackParam(codec, genPlaybackPerfDataPath(codec, resolution, fps),
							resolution, fps, dec, "", "", []string{}))
				}
			}
		}
	}
	// HEVC10
	for _, fps := range []int{30, 60} {
		for _, resolution := range []int{2160, 4320} {
			codec, dec := "hevc10", "hw"
			params = append(params,
				genPlaybackParam(codec, genPlaybackPerfDataPath(codec, resolution, fps),
					resolution, fps, dec,
					"", "", []string{}))
		}
	}
	// Alt
	for _, codec := range []string{"h264", "vp8", "vp9"} {
		resolutions := []int{1080}
		if codec == "vp9" {
			resolutions = append(resolutions, 2160)
		}
		for _, resolution := range resolutions {
			fps, dec := 60, "hw"
			params = append(params,
				genPlaybackParam(codec, genPlaybackPerfDataPath(codec, resolution, fps),
					resolution, fps, dec, "alt", "chromeAlternateVideoDecoder",
					[]string{"video_decoder_legacy_supported"}))
		}
	}
	// long
	for _, codec := range []string{"h264", "vp8", "vp9", "av1"} {
		for _, dec := range []string{"hw", "sw"} {
			resolution, fps := 1080, 30
			file := playbackPerfLongFile[codec]
			param := genPlaybackParam(codec, file, resolution, fps, dec,
				"long", "", []string{"drm_atomic"})

			param.HardwareDeps = "hwdep.SkipOnModel(\"hana\", \"elm\"), hwdep.InternalDisplay()"
			param.MeasureRoughness = true
			params = append(params, param)
		}
	}

	// Out-of-process video decoding (ash-chrome).
	for _, resolution := range []int{720, 1080, 2160} {
		fpss := []int{30}
		if resolution >= 1080 {
			fpss = append(fpss, 60)
		}
		for _, fps := range fpss {
			params = append(params,
				genPlaybackParam("h264", genPlaybackPerfDataPath("h264", resolution, fps),
					resolution, fps, "hw", "oopvd", "chromeVideoOOPVD", []string{}))
		}
	}

	// Out-of-process video decoding (lacros-chrome).
	for _, resolution := range []int{720, 1080, 2160} {
		fpss := []int{30}
		if resolution >= 1080 {
			fpss = append(fpss, 60)
		}
		for _, fps := range fpss {
			params = append(params,
				genPlaybackParam("h264", genPlaybackPerfDataPath("h264", resolution, fps),
					resolution, fps, "hw", "lacros_oopvd", "chromeVideoLacrosOOPVD", []string{"lacros"}))
		}
	}

	// grid
	// TODO(b/234643665): Reduce these to 2x2 1080p (as many pixels as 4K).
	for _, codec := range []string{"h264", "vp8", "vp9", "av1"} {
		resolution, fps, dec := 720, 30, "hw"
		param := genPlaybackParam(codec, genPlaybackPerfDataPath(codec, resolution, fps),
			resolution, fps, dec, "3x3", "", []string{})
		param.GridWidth = 3
		param.GridHeight = 3
		params = append(params, param)
	}

	// lacros
	for _, codec := range []string{"h264", "vp9"} {
		resolution, fps, dec := 720, 30, "hw"
		params = append(params,
			genPlaybackParam(codec, genPlaybackPerfDataPath(codec, resolution, fps),
				resolution, fps, dec, "lacros", "chromeVideoLacros",
				[]string{"lacros"}))
	}
	// libgav1
	for _, resolution := range []int{720, 1080} {
		codec := "av1"
		fpss := []int{30, 60}
		for _, fps := range fpss {
			dec := "sw_gav1"
			params = append(params,
				genPlaybackParam(codec, genPlaybackPerfDataPath(codec, resolution, fps),
					resolution, fps, dec, "",
					"chromeVideoWithSWDecodingAndLibGAV1", []string{"arm"}))
		}
	}

	// multi-playback
	// Get threads and fixture for them. Sort the threads as the order of Golang map iteration is not deterministic.
	threadsCands := []int{1, 2, 4, 9, 16}
	fixtureMap := map[int]string{}
	for _, numThreads := range threadsCands {
		fixtureMap[numThreads] = fmt.Sprintf("chromeVideoWith%dDecoderThreadsAndGlobalVaapiLockDisabled", numThreads)
	}

	for _, codec := range []string{"h264", "vp9"} {
		// 1080p x 2 ~= 2K, 480p x 9  ~= 2K, 360p x 16 ~= 2K, 180p x 49 ~= 1260p
		// TODO(b/237600904): Add {180, 7, 7} once the issue is resolved.
		for _, resGrid := range [][3]int{{1080, 2, 1}, {480, 3, 3}, {360, 4, 4}} {
			resolution, gridW, gridH := resGrid[0], resGrid[1], resGrid[2]
			for _, numThreads := range threadsCands {
				numVideos := gridW * gridH
				// The used decoder threads is smaller of |numThreads| and |numVideos|.
				// So skip the redundant case that |numVideos| is less than |numThreads|.
				if numVideos < numThreads {
					continue
				}
				fps, dec := 30, "hw"
				testNameSuffix := fmt.Sprintf("x%d_%dthreads", numVideos, numThreads)
				fixtureName := fixtureMap[numThreads]
				param := genPlaybackParam(codec,
					genPlaybackPerfDataPath(codec, resolution, fps),
					resolution, fps, dec, testNameSuffix, fixtureName, []string{"thread_safe_libva_backend"})
				param.GridWidth = gridW
				param.GridHeight = gridH
				param.PerfTracing = true
				param.Attr = []string{"group:graphics", "graphics_video", "graphics_nightly"}
				params = append(params, param)
			}
		}
	}

	code := genparams.Template(t, `{{ range . }}{
		Name: {{ .Name | fmt }},
		Val:  playbackPerfParams{
			fileName: {{ .File | fmt }},
			decoderType: {{ .DecoderType }},
			browserType: {{ .BrowserType }},
			{{ if .GridWidth }}
			gridWidth: {{ .GridWidth | fmt }},
			{{ end }}
			{{ if .GridHeight }}
			gridHeight: {{ .GridHeight | fmt }},
			{{ end }}
			{{ if .PerfTracing }}
			perfTracing: {{ .PerfTracing | fmt }},
			{{ end }}
			{{ if .MeasureRoughness }}
			measureRoughness: {{ .MeasureRoughness | fmt }},
			{{ end }}
		},
		{{ if .HardwareDeps }}
		ExtraHardwareDeps: hwdep.D({{ .HardwareDeps }}),
		{{ end }}
		{{ if .SoftwareDeps }}
		ExtraSoftwareDeps: {{ .SoftwareDeps | fmt }},
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
