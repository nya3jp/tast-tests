// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package telemetryextension

import (
	"context"

	"chromiumos/tast/local/bundles/cros/telemetryextension/dep"
	"chromiumos/tast/local/bundles/cros/telemetryextension/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         APIAvailableRoutines,
		LacrosStatus: testing.LacrosVariantNeeded,
		Fixture:      "telemetryExtension",
		Desc:         "Tests chrome.os.diagnostics.getAvailableRoutines Chrome Extension API function exposed to Telemetry Extension",
		Contacts: []string{
			"lamzin@google.com", // Test and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:              "target_models",
				ExtraHardwareDeps: dep.TargetModels(),
			},
			{
				Name:              "low_priority_target_models",
				ExtraHardwareDeps: dep.LowPriorityTargetModels(),
			},
		},
	})
}

// APIAvailableRoutines tests chrome.os.diagnostics.getAvailableRoutines Chrome Extension API functionality.
func APIAvailableRoutines(ctx context.Context, s *testing.State) {
	v := s.FixtValue().(*fixture.Value)

	type response struct {
		Routines []string `json:"routines"`
	}

	var resp response
	if err := v.ExtConn.Call(ctx, &resp,
		"tast.promisify(chrome.os.diagnostics.getAvailableRoutines)",
	); err != nil {
		s.Fatal("Failed to get response from Telemetry extenion service worker: ", err)
	}

	gotRoutines := make(map[string]struct{})
	for _, got := range resp.Routines {
		gotRoutines[got] = struct{}{}
	}

	wantRoutines := []string{
		"battery_capacity",
		"battery_health",
		"cpu_cache",
		"cpu_stress",
		"cpu_floating_point_accuracy",
		"cpu_prime_search",
		"battery_discharge",
		"battery_charge",
		"memory",
	}

	for _, want := range wantRoutines {
		if _, exist := gotRoutines[want]; !exist {
			s.Errorf("Wanted %q routine is missing in available routines %v", want, resp.Routines)
		}
	}
}
