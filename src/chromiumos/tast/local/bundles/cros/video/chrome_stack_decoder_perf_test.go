// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"fmt"
	"testing"

	"chromiumos/tast/common/genparams"
)

// NB: If modifying any of the files or test specifications, be sure to
// regenerate the test parameters by running the following in a chroot:
// TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 chromiumos/tast/local/bundles/cros/video

func genDataPath(codec, resolution, frameRate string) string {
	numFrames := "300"
	if frameRate == "60" {
		numFrames = "600"
	}

	extension := fmt.Sprintf("%s.ivf", codec)
	if codec == "h264" {
		extension = "h264"
	} else if codec == "hevc" {
		extension = "hevc"
	}

	return fmt.Sprintf("perf/%s/%sp_%sfps_%sframes.%s", codec, resolution, frameRate, numFrames, extension)
}

func TestChromeStackDecoderPerfParams(t *testing.T) {
	type paramData struct {
		Name                    string
		File                    string
		ConcurrentDecoders      bool
		GlobalVAAPILockDisabled bool
		SoftwareDeps            []string
		Metadata                []string
		Attr                    []string
	}

	var params []paramData

	var codecs = []string{"av1", "h264", "hevc", "vp8", "vp9"}
	var resolutions = []string{"1080", "2160"}
	var frameRates = []string{"30", "60"}
	for _, codec := range codecs {
		for _, resolution := range resolutions {
			for _, frameRate := range frameRates {
				dataPath := genDataPath(codec, resolution, frameRate)
				param := paramData{
					Name:         fmt.Sprintf("%s_%sp_%sfps", codec, resolution, frameRate),
					File:         dataPath,
					SoftwareDeps: []string{fmt.Sprintf("autotest-capability:hw_dec_%s_%s_%s", codec, resolution, frameRate)},
					Metadata:     []string{dataPath, dataPath + ".json"},
					Attr:         []string{"graphics_video_decodeaccel"},
				}

				params = append(params, param)
			}
		}
	}

	// Another round for the concurrent decoder tests.
	for _, codec := range codecs {
		resolution := "1080"
		frameRate := "60"
		dataPath := genDataPath(codec, resolution, frameRate)
		param := paramData{
			Name:               fmt.Sprintf("%s_%sp_%sfps_concurrent", codec, resolution, frameRate),
			File:               dataPath,
			ConcurrentDecoders: true,
			SoftwareDeps:       []string{fmt.Sprintf("autotest-capability:hw_dec_%s_%s_%s", codec, resolution, frameRate), "thread_safe_libva_backend"},
			Metadata:           []string{dataPath, dataPath + ".json"},
			Attr:               []string{"graphics_video_decodeaccel"},
		}

		params = append(params, param)
	}
	// Another round for the concurrent decoder tests with VA-API global lock
	// disabled.
	for _, codec := range codecs {
		resolution := "1080"
		frameRate := "60"
		dataPath := genDataPath(codec, resolution, frameRate)
		param := paramData{
			Name:                    fmt.Sprintf("%s_%sp_%sfps_concurrent_global_vaapi_lock_disabled", codec, resolution, frameRate),
			File:                    dataPath,
			ConcurrentDecoders:      true,
			GlobalVAAPILockDisabled: true,
			SoftwareDeps:            []string{fmt.Sprintf("autotest-capability:hw_dec_%s_%s_%s", codec, resolution, frameRate), "thread_safe_libva_backend"},
			Metadata:                []string{dataPath, dataPath + ".json"},
			Attr:                    []string{"graphics_video_decodeaccel"},
		}

		params = append(params, param)
	}

	code := genparams.Template(t, `{{ range . }}{
		Name: {{ .Name | fmt }},
		Val:  chromeStackDecoderPerfParams{
			dataPath: {{ .File | fmt }},
			runConcurrentDecodersOnly: {{ .ConcurrentDecoders | fmt }},
			disableGlobalVaapiLock: {{ .GlobalVAAPILockDisabled | fmt }},
		},
		{{ if .SoftwareDeps }}
		ExtraSoftwareDeps: {{ .SoftwareDeps | fmt }},
		{{ end }}
		ExtraData: {{ .Metadata | fmt }},
		{{ if .Attr }}
		ExtraAttr: {{ .Attr | fmt }},
		{{ end }}
	},
	{{ end }}`, params)
	genparams.Ensure(t, "chrome_stack_decoder_perf.go", code)

	// Clear the params, and compose another set of params with a reduced set of
	// codecs, for the legacy variant of the test.
	params = nil
	var reducedCodecs = []string{"h264", "vp8", "vp9"}
	for _, codec := range reducedCodecs {
		for _, resolution := range resolutions {
			for _, frameRate := range frameRates {
				dataPath := genDataPath(codec, resolution, frameRate)
				param := paramData{
					Name:         fmt.Sprintf("%s_%sp_%sfps", codec, resolution, frameRate),
					File:         dataPath,
					SoftwareDeps: []string{fmt.Sprintf("autotest-capability:hw_dec_%s_%s_%s", codec, resolution, frameRate)},
					Metadata:     []string{dataPath, dataPath + ".json"},
					Attr:         []string{"graphics_video_decodeaccel"},
				}

				params = append(params, param)
			}
		}
	}

	legacyCode := genparams.Template(t, `{{ range . }}{
		Name: {{ .Name | fmt }},
		Val: {{ .File | fmt }},
		{{ if .SoftwareDeps }}
		ExtraSoftwareDeps: {{ .SoftwareDeps | fmt }},
		{{ end }}
		ExtraData: {{ .Metadata | fmt }},
		{{ if .Attr }}
		ExtraAttr: {{ .Attr | fmt }},
		{{ end }}
	},
	{{ end }}`, params)
	genparams.Ensure(t, "chrome_stack_decoder_legacy_perf.go", legacyCode)

}
