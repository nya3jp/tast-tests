// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"strings"
	"testing"

	"chromiumos/tast/common/genparams"
)

func genTestName(components []string) string {
	var nonEmptyComponents []string
	for _, s := range components {
		if len(s) == 0 {
			continue
		}
		nonEmptyComponents = append(nonEmptyComponents, s)
	}
	return strings.Join(nonEmptyComponents, "_")
}

func TestPowerIdlePerfParamsAreGenerated(t *testing.T) {
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
	}
	var params []paramData
	for _, batteryMode := range []struct {
		name  string
		hwdep string
		val   []valMember
	}{
		{
			"",
			"hwdep.ForceDischarge()",
			[]valMember{{"setupOption", "setup.ForceBatteryDischarge"}},
		},
		{
			"nobatterymetrics",
			"hwdep.NoForceDischarge()",
			[]valMember{{"setupOption", "setup.NoBatteryDischarge"}},
		},
	} {
		for _, arcType := range []struct {
			name  string
			swdep string
			pre   string
		}{
			{
				"noarc",
				"arc", // to prevent _noarc tests from running on non-ARC boards
				"chrome.LoggedIn()",
			},
			{
				"",
				"android_p",
				"arc.Booted()",
			},
			{
				"vm",
				"android_vm",
				"arc.Booted()",
			},
		} {
			name := genTestName([]string{arcType.name, batteryMode.name})
			p := paramData{
				Name:         string(name),
				SoftwareDeps: []string{arcType.swdep},
				HardwareDeps: []string{batteryMode.hwdep},
				Pre:          arcType.pre,
				Val:          batteryMode.val,
			}
			params = append(params, p)
		}
	}

	code := genparams.Template(t, `{{ range . }}{
		{{ if .Name }}
		Name: {{ .Name | fmt }},
		{{ end }}
		ExtraSoftwareDeps: {{ .SoftwareDeps | fmt }},
		ExtraHardwareDeps: hwdep.D({{ range .HardwareDeps }}{{ . }},{{ end }}),
		Val: testArgsForPowerIdlePerf{
			{{ range .Val }}{{ .Key }}: {{ .Value }},
			{{ end }}
		},
		Pre: {{ .Pre }},
	},
	{{ end }}`, params)
	genparams.Ensure(t, "power_idle_perf.go", code)
}
