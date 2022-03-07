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

func TestMeetCUJParamsAreGenerated(t *testing.T) {
	type valMember struct {
		Key   string
		Value string
	}
	type paramData struct {
		Comment      string
		Name         string
		Timeout      string
		Val          []valMember
		Fixture      string
		SoftwareDeps []string
		HardwareDeps []string
	}
	var params []paramData
	for _, battery := range []struct {
		commentSuffix string
		nameSuffix    string
		batteryMode   string
		hwdep         string
	}{
		{
			commentSuffix: "",
			nameSuffix:    "",
			batteryMode:   "setup.ForceBatteryDischarge",
			hwdep:         "hwdep.ForceDischarge()",
		},
		{
			commentSuffix: " (nobatterymetrics)",
			nameSuffix:    "_nobatterymetrics",
			batteryMode:   "setup.NoBatteryDischarge",
			hwdep:         "hwdep.NoForceDischarge()",
		},
	} {
		for _, meet := range []struct {
			comment string
			name    string
			timeout string
			val     []valMember
			fixture string
			swdeps  []string
		}{
			{
				comment: "Base case. Note this runs a 30 min meet call",
				name:    "4p",
				timeout: "40 * time.Minute",
				val: []valMember{
					{"num", "4"},
					{"layout", "meetLayoutTiled"},
					{"cam", "true"},
					{"duration", "30 * time.Minute"},
				},
				fixture: "loggedInToCUJUser",
			},
			{
				comment: "Small meeting",
				name:    "4p_present_notes_split",
				timeout: "defaultTestTimeout",
				val: []valMember{
					{"num", "4"},
					{"layout", "meetLayoutTiled"},
					{"present", "true"},
					{"docs", "true"},
					{"split", "true"},
					{"cam", "true"},
				},
				fixture: "loggedInToCUJUser",
			},
			{
				comment: "Big meeting",
				name:    "16p",
				timeout: "defaultTestTimeout",
				val: []valMember{
					{"num", "16"},
					{"layout", "meetLayoutTiled"},
					{"cam", "true"},
				},
				fixture: "loggedInToCUJUser",
			},
			{
				comment: "Even bigger meeting",
				name:    "49p",
				timeout: "defaultTestTimeout",
				val: []valMember{
					{"num", "49"},
					{"layout", "meetLayoutTiled"},
					{"cam", "true"},
				},
				fixture: "loggedInToCUJUser",
			},
			{
				comment: "Big meeting with tracing",
				name:    "16p_trace",
				timeout: "20 * time.Minute",
				val: []valMember{
					{"num", "16"},
					{"layout", "meetLayoutTiled"},
					{"cam", "true"},
					{"tracing", "true"},
				},
				fixture: "loggedInToCUJUser",
			},
			{
				comment: "Validation test for big meeting",
				name:    "16p_validation",
				timeout: "20 * time.Minute",
				val: []valMember{
					{"num", "16"},
					{"layout", "meetLayoutTiled"},
					{"cam", "true"},
					{"validation", "true"},
				},
				fixture: "loggedInToCUJUser",
			},
			{
				comment: "Big meeting with notes",
				name:    "16p_notes",
				timeout: "defaultTestTimeout",
				val: []valMember{
					{"num", "16"},
					{"layout", "meetLayoutTiled"},
					{"docs", "true"},
					{"split", "true"},
					{"cam", "true"},
				},
				fixture: "loggedInToCUJUser",
			},
			{
				comment: "16p with jamboard test",
				name:    "16p_jamboard",
				timeout: "defaultTestTimeout",
				val: []valMember{
					{"num", "16"},
					{"layout", "meetLayoutTiled"},
					{"jamboard", "true"},
					{"split", "true"},
					{"cam", "true"},
				},
				fixture: "loggedInToCUJUser",
			},
			{
				comment: "Lacros 4p",
				name:    "lacros_4p",
				timeout: "defaultTestTimeout",
				val: []valMember{
					{"num", "4"},
					{"layout", "meetLayoutTiled"},
					{"cam", "true"},
					{"useLacros", "true"},
				},
				fixture: "loggedInToCUJUserLacros",
				swdeps:  []string{"lacros"},
			},
			{
				comment: "49p with vp8 video codec",
				name:    "49p_vp8",
				timeout: "defaultTestTimeout",
				val: []valMember{
					{"num", "49"},
					{"layout", "meetLayoutTiled"},
					{"cam", "true"},
					{"botsOptions", "[]bond.AddBotsOption{bond.WithVP9(false, false)}"},
				},
				fixture: "loggedInToCUJUser",
			},
		} {
			p := paramData{
				Comment:      meet.comment + battery.commentSuffix,
				Name:         meet.name + battery.nameSuffix,
				Timeout:      meet.timeout,
				Val:          append(meet.val, valMember{"batteryMode", battery.batteryMode}),
				Fixture:      meet.fixture,
				SoftwareDeps: meet.swdeps,
				HardwareDeps: []string{battery.hwdep},
			}
			params = append(params, p)
		}
	}

	code := genparams.Template(t, `{{ range . }}{
		// {{ .Comment }}
		Name: {{ .Name | fmt }},
		Timeout: {{ .Timeout }},
		Val: meetTest{
			{{ range .Val }}{{ .Key }}: {{ .Value }},
			{{ end }}
		},
		Fixture: {{ .Fixture | fmt }},
		{{ if .SoftwareDeps }}
		ExtraSoftwareDeps: {{ .SoftwareDeps | fmt }},
		{{ end }}
		ExtraHardwareDeps: hwdep.D({{ range .HardwareDeps }}{{ . }},{{ end }}),
	},
	{{ end }}`, params)
	genparams.Ensure(t, "meet_cuj.go", code)
}
