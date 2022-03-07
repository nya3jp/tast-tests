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

func TestQuickCheckCUJParamsAreGenerated(t *testing.T) {
	type paramData struct {
		Name         string
		BrowserType  string
		BatteryMode  string
		Fixture      string
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
			name    string
			browser string
			fixture string
			swdeps  []string
		}{
			{
				browser: "browser.TypeAsh",
				fixture: "loggedInToCUJUser",
			},
			{
				name:    "lacros",
				browser: "browser.TypeLacros",
				fixture: "loggedInToCUJUserLacros",
				swdeps:  []string{"lacros"},
			},
		} {
			p := paramData{
				Name:         genTestName([]string{test.name, battery.name}),
				BrowserType:  test.browser,
				BatteryMode:  battery.batteryMode,
				Fixture:      test.fixture,
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
	Val: quickCheckCUJParams{
		browserType: {{ .BrowserType }},
		batteryMode: {{ .BatteryMode }},
	},
	Fixture: {{ .Fixture | fmt }},
	{{ if .SoftwareDeps }}
	ExtraSoftwareDeps: {{ .SoftwareDeps | fmt }},
	{{ end }}
	ExtraHardwareDeps: hwdep.D({{ range .HardwareDeps }}{{ . }},{{ end }}),
	},
	{{ end }}`, params)
	genparams.Ensure(t, "quick_check_cuj.go", code)
}
