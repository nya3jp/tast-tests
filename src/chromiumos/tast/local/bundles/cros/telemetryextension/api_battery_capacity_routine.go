// Copyright 2022 The ChromiumOS Authors.
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
		Func:         APIBatteryCapacityRoutine,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests chrome.os.diagnostics.runBatteryCapacityRoutine Chrome Extension API function exposed to Telemetry Extension",
		Contacts: []string{
			"lamzin@google.com",    // Test and Telemetry Extension author
			"bkersting@google.com", // Test author
			"cros-oem-services-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:              "stable",
				Fixture:           "telemetryExtension",
				ExtraHardwareDeps: dep.StableModels(),
			},
			{
				Name:              "non_stable",
				Fixture:           "telemetryExtensionOverrideOEMName",
				ExtraHardwareDeps: dep.NonStableModels(),
			},
			{
				Name:              "stable_lacros",
				Fixture:           "telemetryExtensionLacros",
				ExtraHardwareDeps: dep.StableModels(),
			},
			{
				Name:              "non_stable_lacros",
				Fixture:           "telemetryExtensionOverrideOEMNameLacros",
				ExtraHardwareDeps: dep.NonStableModels(),
			},
			{
				Name:              "stable_managed",
				Fixture:           "telemetryExtensionManaged",
				ExtraHardwareDeps: dep.StableModels(),
			},
			{
				Name:              "non_stable_managed",
				Fixture:           "telemetryExtensionOverrideOEMNameManaged",
				ExtraHardwareDeps: dep.NonStableModels(),
			},
			{
				Name:              "stable_managed_lacros",
				Fixture:           "telemetryExtensionManagedLacros",
				ExtraHardwareDeps: dep.StableModels(),
			},
			{
				Name:              "non_stable_managed_lacros",
				Fixture:           "telemetryExtensionOverrideOEMNameManagedLacros",
				ExtraHardwareDeps: dep.NonStableModels(),
			},
		},
	})
}

func APIBatteryCapacityRoutine(ctx context.Context, s *testing.State) {
	v := s.FixtValue().(*fixture.Value)

	type response struct {
		Status string `json:"status"`
	}

	// get result from the diagnostics API.
	var resp response
	if err := v.ExtConn.Call(ctx, &resp,
		"tast.promisify(chrome.os.diagnostics.runBatteryCapacityRoutine)"); err != nil {
		s.Fatal("Failed to get response from Telemetry extenion service worker: ", err)
	}

	// Assert the status is "passed" or "failed". We allow a failing or unsupported routine
	//  (note: no "error", "cancelled", etc.) because we can't make assumptions about the
	// DUT's battery state. "passed", "unsupported" and "failed" signal that the request
	// successfully reached cros_healthd.
	if resp.Status != "passed" && resp.Status != "failed" && resp.Status != "unsupported" {
		s.Errorf(`Unexpected routine status: got %q; want "passed" or "failed" or "unsupported"`, resp.Status)
	}
}
