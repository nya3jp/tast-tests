// Copyright 2020 The ChromiumOS Authors
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
			name    string
			swdep   string
			fixture string
		}{
			{
				"noarc",
				"arc", // to prevent _noarc tests from running on non-ARC boards
				"chromeLoggedInDisableSyncNoFwUpdate",
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
		Fixture: "{{ .Fixture }}",
	},
	{{ end }}`, params)
	genparams.Ensure(t, "power_idle_perf.go", code)
}
