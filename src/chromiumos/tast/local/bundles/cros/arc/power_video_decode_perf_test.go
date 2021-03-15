// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"testing"

	"chromiumos/tast/common/genparams"
)

func TestPowerVideoDecodePerfParamsAreGenerated(t *testing.T) {
	type valMember struct {
		Key   string
		Value string
	}
	type paramData struct {
		Name         string
		Val          []valMember
		SoftwareDeps []string
		HardwareDeps []string
		Data         []string
	}
	var params []paramData
	for _, decoderParam := range []struct {
		name         string
		swdepLiteral string
		dataName     string
	}{
		{
			"h264_1080p_30fps",
			"caps.HWDecodeH264",
			"1080p_30fps_300frames.h264",
		},
		{
			"vp8_1080p_30fps",
			"caps.HWDecodeVP8",
			"1080p_30fps_300frames.vp8.ivf",
		},
		{
			"vp9_1080p_30fps",
			"caps.HWDecodeVP9",
			"1080p_30fps_300frames.vp9.ivf",
		},
		{
			"vp9_1080p_60fps",
			"caps.HWDecodeVP9_60",
			"1080p_60fps_600frames.vp9.ivf",
		},
		{
			"vp9_2160p_30fps",
			"caps.HWDecodeVP9_4K",
			"2160p_30fps_300frames.vp9.ivf",
		},
		{
			"vp9_2160p_60fps",
			"caps.HWDecodeVP9_4K60",
			"2160p_60fps_600frames.vp9.ivf",
		},
	} {
		for _, arcType := range []struct {
			name  string
			swdep string
			attr  []string
		}{
			{
				swdep: "android_p",
			},
			{
				name:  "vm",
				swdep: "android_vm",
			},
		} {
			for _, batteryMode := range []struct {
				name  string
				hwdep string
				val   []valMember
			}{
				{
					hwdep: "hwdep.ForceDischarge()",
					val:   []valMember{{"BatteryDischargeMode", "setup.ForceBatteryDischarge"}},
				},
				{
					name:  "nobatterymetrics",
					hwdep: "hwdep.NoForceDischarge()",
					val:   []valMember{{"BatteryDischargeMode", "setup.NoBatteryDischarge"}},
				},
			} {
				name := genTestName([]string{
					decoderParam.name,
					arcType.name,
					batteryMode.name,
				})
				p := paramData{
					Name:         string(name),
					SoftwareDeps: []string{decoderParam.swdepLiteral, "\"" + arcType.swdep + "\""},
					HardwareDeps: []string{batteryMode.hwdep},
					Val: append([]valMember{
						{"TestVideo", "\"" + decoderParam.dataName + "\""},
					}, batteryMode.val...),
					Data: []string{decoderParam.dataName, decoderParam.dataName + ".json"},
				}
				params = append(params, p)
			}
		}
	}
	code := genparams.Template(t, `{{ range . }}{
		Name: {{ .Name | fmt }},
		Val: video.DecodeTestOptions{
			{{ range .Val }}{{ .Key }}: {{ .Value }},
			{{ end }}
		},
		ExtraSoftwareDeps: []string{
			{{ range .SoftwareDeps }}{{ . }},
			{{ end }}
		},
		ExtraHardwareDeps: hwdep.D({{ range .HardwareDeps }}{{ . }},{{ end }}),
		ExtraData: {{ .Data | fmt }},
	},
	{{ end }}`, params)
	genparams.Ensure(t, "power_video_decode_perf.go", code)
}
