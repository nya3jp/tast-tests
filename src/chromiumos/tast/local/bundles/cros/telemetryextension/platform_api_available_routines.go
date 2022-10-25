// Copyright 2022 The ChromiumOS Authors
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
		Func:         PlatformAPIAvailableRoutines,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests chrome.os.diagnostics.getAvailableRoutines Chrome Extension API function exposed to Telemetry Extension",
		Contacts: []string{
			"chromeos-oem-services@google.com", // Use team email for tickets.
			"bkersting@google.com",
			"lamzin@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:              "stable",
				Fixture:           fixture.TelemetryExtension,
				ExtraHardwareDeps: dep.StableModels(),
			},
			{
				Name:              "non_stable",
				Fixture:           fixture.TelemetryExtension,
				ExtraHardwareDeps: dep.NonStableModels(),
			},
			{
				Name:              "stable_lacros",
				Fixture:           fixture.TelemetryExtensionLacros,
				ExtraHardwareDeps: dep.StableModels(),
			},
			{
				Name:              "non_stable_lacros",
				Fixture:           fixture.TelemetryExtensionLacros,
				ExtraHardwareDeps: dep.NonStableModels(),
			},
		},
	})
}

// PlatformAPIAvailableRoutines tests chrome.os.diagnostics.getAvailableRoutines Chrome Extension API functionality.
func PlatformAPIAvailableRoutines(ctx context.Context, s *testing.State) {
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
		"ac_power",
		"cpu_cache",
		"cpu_stress",
		"cpu_floating_point_accuracy",
		"cpu_prime_search",
		"lan_connectivity",
		"memory",
	}

	for _, want := range wantRoutines {
		if _, exist := gotRoutines[want]; !exist {
			s.Errorf("Wanted %q routine is missing in available routines %v", want, resp.Routines)
		}
	}
}
