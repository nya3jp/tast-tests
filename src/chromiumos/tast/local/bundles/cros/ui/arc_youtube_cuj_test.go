// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

// To update test parameters after modifying this file, run:
// TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 chromiumos/tast/local/bundles/cros/ui/

import (
	"strings"
	"testing"

	"chromiumos/tast/common/genparams"
)

func genTestName(components []string) string {
	var nonEmptyComponents []string
	for _, s := range components {
		if s != "" {
			nonEmptyComponents = append(nonEmptyComponents, s)
		}
	}
	return strings.Join(nonEmptyComponents, "_")
}

func TestArcYoutubeCUJParamsAreGenerated(t *testing.T) {
	type paramData struct {
		Name         string
		Val          string
		SoftwareDeps []string
		HardwareDeps []string
	}
	var params []paramData
	for _, battery := range []struct {
		name        string
		batteryMode string
		hwdep       string
	}{
		{
			name:        "",
			batteryMode: "setup.ForceBatteryDischarge",
			hwdep:       "hwdep.ForceDischarge()",
		},
		{
			name:        "nobatterymetrics",
			batteryMode: "setup.NoBatteryDischarge",
			hwdep:       "hwdep.NoForceDischarge()",
		},
	} {
		for _, test := range []struct {
			name   string
			swdeps []string
		}{
			{
				swdeps: []string{"android_p"},
			},
			{
				name:   "vm",
				swdeps: []string{"android_vm"},
			},
		} {
			p := paramData{
				Name:         genTestName([]string{test.name, battery.name}),
				Val:          battery.batteryMode,
				SoftwareDeps: test.swdeps,
				HardwareDeps: []string{battery.hwdep},
			}
			params = append(params, p)
		}
	}

	code := genparams.Template(t, `{{ range . }}{
	{{ if .Name }}
	Name: {{ .Name | fmt }},
	{{ end }}
	Val: {{ .Val }},
	ExtraSoftwareDeps: {{ .SoftwareDeps | fmt }},
	ExtraHardwareDeps: hwdep.D({{ range .HardwareDeps }}{{ . }},{{ end }}),
	},
	{{ end }}`, params)
	genparams.Ensure(t, "arc_youtube_cuj.go", code)
}
