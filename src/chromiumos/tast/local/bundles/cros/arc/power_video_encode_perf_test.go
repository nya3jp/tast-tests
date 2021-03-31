// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"testing"

	"chromiumos/tast/common/genparams"
)

func TestPowerVideoEncodePerfParamsAreGenerated(t *testing.T) {
	type valMember struct {
		Key   string
		Value string
	}
	type paramData struct {
		Name         string
		SoftwareDeps []string
		HardwareDeps []string
		Pre          string
		Val          []valMember
		Attr         []string
	}
	var params []paramData
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
		const arcBooted = "arc.Booted()"
		for _, arcType := range []struct {
			name  string
			swdep string
			pre   string
			attr  []string
		}{
			{
				swdep: "android_p",
				pre:   arcBooted,
				attr:  []string{"group:crosbolt", "crosbolt_nightly"},
			},
			{
				name:  "vm",
				swdep: "android_vm",
				pre:   arcBooted,
			},
		} {
			name := genTestName([]string{
				"h264_1080p_i420",
				arcType.name,
				batteryMode.name,
			})
			p := paramData{
				Name:         string(name),
				SoftwareDeps: []string{arcType.swdep},
				HardwareDeps: []string{batteryMode.hwdep},
				Pre:          arcType.pre,
				Val: append([]valMember{
					{"Profile", "videotype.H264MainProf"},
					{"Params", "video.Crowd1080P"},
					{"PixelFormat", "videotype.I420"},
				}, batteryMode.val...),
				Attr: arcType.attr,
			}
			params = append(params, p)
		}
	}

	code := genparams.Template(t, `{{ range . }}{
		Name: {{ .Name | fmt }},
		Val: video.EncodeTestOptions{
			{{ range .Val }}{{ .Key }}: {{ .Value }},
			{{ end }}
		},
		ExtraData:         []string{video.Crowd1080P.Name},
		{{ if .SoftwareDeps }}
		ExtraSoftwareDeps: {{ .SoftwareDeps | fmt }},
		{{ end }}
		ExtraHardwareDeps: hwdep.D({{ range .HardwareDeps }}{{ . }},{{ end }}),
		{{ if .Attr }}
		ExtraAttr: {{ .Attr | fmt }},
		{{ end }}
	},
	{{ end }}`, params)
	genparams.Ensure(t, "power_video_encode_perf.go", code)
}
