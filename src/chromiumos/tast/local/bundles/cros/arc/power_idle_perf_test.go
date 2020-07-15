// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"chromiumos/tast/common/generate"
	"fmt"
	"strings"
	"testing"
)

// "l" + "" => "l"
// "l" + "r" => "l_r"
// "" + "r" => "r"
func addSuffix(body, suffix string) string {
	if len(suffix) == 0 {
		return body
	}
	if len(body) == 0 {
		return suffix
	}
	return fmt.Sprintf("%v_%v", body, suffix)
}

func TestPowerIdlePerfParams(t *testing.T) {
	var params []string
	for _, battery := range []struct {
		suffix string
		attrs  []string
	}{
		{"", []string{
			`ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge())`,
			`
				Val: testArgsForPowerIdlePerf{
					setupOption: setup.ForceBatteryDischarge,
				},
				`,
		}},
		{"nobatterymetrics", []string{
			`ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge())`,
			`
				Val: testArgsForPowerIdlePerf{
					setupOption: setup.NoBatteryDischarge,
				},
				`,
		}}} {
		for _, variant := range []struct {
			name  string
			attrs []string
		}{
			{"noarc", []string{
				`ExtraSoftwareDeps: []string{"arc"}`,
				`Pre: chrome.LoggedIn()`,
			}},
			{"", []string{
				`ExtraSoftwareDeps: []string{"android_p"}`,
				`Pre: arc.Booted()`,
			}},
			{"vm", []string{
				`ExtraSoftwareDeps: []string{"android_vm"}`,
				`Pre: arc.Booted()`,
			}},
		} {
			name := addSuffix(variant.name, battery.suffix)
			attrs := strings.Join(
				[]string{
					strings.Join(variant.attrs, ",\n"),
					strings.Join(battery.attrs, ",\n"),
				}, ",\n")
			p := fmt.Sprintf(`
{
	Name: "%v",
	%v
},
`, name, attrs)
			t.Log(p)
			params = append(params, p)
		}
	}
	generate.Params(t, "power_idle_perf.go", strings.Join(params, "\n"))
}
