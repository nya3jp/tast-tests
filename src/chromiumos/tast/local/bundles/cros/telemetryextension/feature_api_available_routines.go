// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package telemetryextension

import (
	"context"

	"chromiumos/tast/local/bundles/cros/telemetryextension/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FeatureAPIAvailableRoutines,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests chrome.os.diagnostics.getAvailableRoutines Chrome Extension API function exposed to Telemetry Extension and check for optional routines",
		Contacts: []string{
			"lamzin@google.com",    // Test and Telemetry Extension author
			"bkersting@google.com", // Test author
			"cros-oem-services-team@google.com",
		},
		Attr:         []string{"group:telemetry_extension_hw"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			// Depend on a battery.
			{
				Name:    "battery_health_routine",
				Fixture: "telemetryExtension",
				Val:     "battery_health",
			},
			{
				Name:    "battery_capacity_routine",
				Fixture: "telemetryExtension",
				Val:     "battery_capacity",
			},
			{
				Name:    "battery_discharge_routine",
				Fixture: "telemetryExtension",
				Val:     "battery_discharge",
			},
			{
				Name:    "battery_charge_routine",
				Fixture: "telemetryExtension",
				Val:     "battery_charge",
			},
			// Depend on an NVME capable disk.
			{
				Name:    "nvme_wear_level_routine",
				Fixture: "telemetryExtension",
				Val:     "nvme_wear_level",
			},
			// Depend on SmartCTL support.
			{
				Name:    "smart_ctl_check_routine",
				Fixture: "telemetryExtension",
				Val:     "smartctl_check",
			},
			// Depend on FIO support.
			{
				Name:    "disk_read_routine",
				Fixture: "telemetryExtension",
				Val:     "disk_read",
			},
		},
	})
}

func FeatureAPIAvailableRoutines(ctx context.Context, s *testing.State) {
	v := s.FixtValue().(*fixture.Value)

	routineName, ok := s.Param().(string)
	if !ok {
		s.Fatal("Failed to convert params value into string: ", s.Param())
	}

	type response struct {
		Routines []string `json:"routines"`
	}

	var resp response
	if err := v.ExtConn.Call(ctx, &resp,
		"tast.promisify(chrome.os.diagnostics.getAvailableRoutines)",
	); err != nil {
		s.Fatal("Failed to get response from Telemetry extenion service worker: ", err)
	}

	if !contains(resp.Routines, routineName) {
		s.Errorf(`Unexpected result from "GetAvailableRoutines": expected %q to be present, but was not`, routineName)
	}
}

func contains(list []string, want string) bool {
	for _, elem := range list {
		if elem == want {
			return true
		}
	}
	return false
}
