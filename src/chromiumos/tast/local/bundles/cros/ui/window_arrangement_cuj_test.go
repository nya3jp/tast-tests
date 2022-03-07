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

func TestWindowArrangementCUJParamsAreGenerated(t *testing.T) {
	type valMember struct {
		Key   string
		Value string
	}
	type paramData struct {
		Name         string
		Val          []valMember
		Fixture      string
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
			val     []valMember
			fixture string
			swdeps  []string
		}{
			{
				name: "clamshell_mode",
				val: []valMember{
					{"BrowserType", "browser.TypeAsh"},
				},
				fixture: "arcBootedInClamshellMode",
				swdeps:  []string{"android_p"},
			},
			{
				name: "tablet_mode",
				val: []valMember{
					{"BrowserType", "browser.TypeAsh"},
					{"Tablet", "true"},
				},
				swdeps: []string{"android_p"},
			},
			{
				name: "tablet_mode_trace",
				val: []valMember{
					{"BrowserType", "browser.TypeAsh"},
					{"Tablet", "true"},
					{"Tracing", "true"},
				},
				swdeps: []string{"android_p"},
			},
			{
				name: "tablet_mode_validation",
				val: []valMember{
					{"BrowserType", "browser.TypeAsh"},
					{"Tablet", "true"},
					{"Validation", "true"},
				},
				swdeps: []string{"android_p"},
			},
			{
				name: "lacros",
				val: []valMember{
					{"BrowserType", "browser.TypeLacros"},
				},
				fixture: "lacrosWithArcBooted",
				swdeps:  []string{"android_p", "lacros"},
			},
			{
				name: "vm",
				val: []valMember{
					{"BrowserType", "browser.TypeAsh"},
				},
				fixture: "arcBootedInClamshellMode",
				swdeps:  []string{"android_vm"},
			},
		} {
			p := paramData{
				Name:         testParam.name + battery.nameSuffix,
				Val:          append(testParam.val, valMember{"BatteryMode", battery.batteryMode}),
				Fixture:      testParam.fixture,
				SoftwareDeps: testParam.swdeps,
				HardwareDeps: []string{battery.hwdep},
			}
			params = append(params, p)
		}
	}

	code := genparams.Template(t, `{{ range . }}{
		Name: {{ .Name | fmt }},
		Val: windowarrangementcuj.TestParam{
			{{ range .Val }}{{ .Key }}: {{ .Value }},
			{{ end }}
		},
		{{ if .Fixture }}
		Fixture: {{ .Fixture | fmt }},
		{{ end }}
		ExtraSoftwareDeps: {{ .SoftwareDeps | fmt }},
		ExtraHardwareDeps: hwdep.D({{ range .HardwareDeps }}{{ . }},{{ end }}),
	},
	{{ end }}`, params)
	genparams.Ensure(t, "window_arrangement_cuj.go", code)
}
