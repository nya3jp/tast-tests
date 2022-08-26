// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

// To update test parameters after modifying this file, run:
// TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 chromiumos/tast/local/bundles/cros/arc/

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
		Fixture      string
		Timeout      string
		Val          []valMember
	}
	var params []paramData
	for _, batteryMode := range []struct {
		name    string
		hwdep   string
		timeout string
		val     []valMember
	}{
		{
			"",
			"hwdep.ForceDischarge()",
			"15 * time.Minute",
			[]valMember{
				{"setupOption", "setup.ForceBatteryDischarge"},
				{"metrics", "power.TestMetrics()"},
				{"iterations", "30"}, // 5 minutes.
			},
		},
		{
			"discharge",
			"hwdep.ForceDischarge()",
			"40 * time.Minute",
			[]valMember{
				{"setupOption", "setup.ForceBatteryDischarge"},
				{"metrics", "[]perf.TimelineDatasource{power.NewSysfsBatteryMetrics()}"},
				{"iterations", "180"}, // 30 minutes.
			},
		},
		{
			"nobatterymetrics",
			"hwdep.NoForceDischarge()",
			"15 * time.Minute",
			[]valMember{
				{"setupOption", "setup.NoBatteryDischarge"},
				{"metrics", "power.TestMetrics()"},
				{"iterations", "30"}, // 5 minutes.
			},
		},
	} {
		for _, arcType := range []struct {
			name    string
			swdep   string
			fixture string
		}{
			{
				"noarc",
				"arc", // to prevent _noarc tests from running on non-ARC boards
				"chromeLoggedIn",
			},
			{
				"",
				"android_p",
				"arcBootedRestricted",
			},
			{
				"vm",
				"android_vm",
				"arcBootedRestricted",
			},
		} {
			name := genTestName([]string{arcType.name, batteryMode.name})
			p := paramData{
				Name:         string(name),
				SoftwareDeps: []string{arcType.swdep},
				HardwareDeps: []string{batteryMode.hwdep},
				Fixture:      arcType.fixture,
				Timeout:      batteryMode.timeout,
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
		Timeout: {{ .Timeout }},
		Fixture: "{{ .Fixture }}",
	},
	{{ end }}`, params)
	genparams.Ensure(t, "power_idle_perf.go", code)
}
