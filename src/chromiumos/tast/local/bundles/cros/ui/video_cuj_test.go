// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

// To update test parameters after modifying this file, run:
// TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 chromiumos/tast/local/bundles/cros/ui/

import (
	"testing"

	"chromiumos/tast/common/genparams"
)

func TestVideoCUJParamsAreGenerated(t *testing.T) {
	type valMember struct {
		Key   string
		Value string
	}
	type paramData struct {
		Name         string
		Fixture      string
		Val          []valMember
		SoftwareDeps []string
		HardwareDeps []string
	}
	var params []paramData
	for _, battery := range []struct {
		nameSuffix  string
		batteryMode string
		hwdep       string
	}{
		{
			nameSuffix:  "",
			batteryMode: "setup.ForceBatteryDischarge",
			hwdep:       "hwdep.ForceDischarge()",
		},
		{
			nameSuffix:  "_nobatterymetrics",
			batteryMode: "setup.NoBatteryDischarge",
			hwdep:       "hwdep.NoForceDischarge()",
		},
	} {
		for _, testParam := range []struct {
			name    string
			fixture string
			val     []valMember
			swdeps  []string
		}{
			{
				name:    "clamshell",
				fixture: "loggedInToCUJUser",
				val: []valMember{
					{"bt", "browser.TypeAsh"},
				},
			},
			{
				name:    "clamshell_trace",
				fixture: "loggedInToCUJUser",
				val: []valMember{
					{"bt", "browser.TypeAsh"},
					{"tracing", "true"},
				},
			},
			{
				name:    "tablet",
				fixture: "loggedInToCUJUser",
				val: []valMember{
					{"bt", "browser.TypeAsh"},
					{"tablet", "true"},
				},
			},
			{
				name:    "lacros",
				fixture: "loggedInToCUJUserLacros",
				val: []valMember{
					{"bt", "browser.TypeLacros"},
				},
				swdeps: []string{"lacros"},
			},
			{
				name:    "lacros_tablet",
				fixture: "loggedInToCUJUserLacros",
				val: []valMember{
					{"bt", "browser.TypeLacros"},
					{"tablet", "true"},
				},
				swdeps: []string{"lacros"},
			},
		} {
			p := paramData{
				Name:         testParam.name + battery.nameSuffix,
				Fixture:      testParam.fixture,
				Val:          append(testParam.val, valMember{"batteryMode", battery.batteryMode}),
				SoftwareDeps: testParam.swdeps,
				HardwareDeps: []string{battery.hwdep},
			}
			params = append(params, p)
		}
	}

	code := genparams.Template(t, `{{ range . }}{
		Name: {{ .Name | fmt }},
		Fixture: {{ .Fixture | fmt }},
		Val: videoCUJTestParam{
			{{ range .Val }}{{ .Key }}: {{ .Value }},
			{{ end }}
		},
		{{ if .SoftwareDeps }}
		ExtraSoftwareDeps: {{ .SoftwareDeps | fmt }},
		{{ end }}
		ExtraHardwareDeps: hwdep.D({{ range .HardwareDeps }}{{ . }},{{ end }}),
	},
	{{ end }}`, params)
	genparams.Ensure(t, "video_cuj.go", code)
}
