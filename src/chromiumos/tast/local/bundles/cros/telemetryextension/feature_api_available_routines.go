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
		Desc:         "Tests chrome.os.diagnostics.getAvailableRoutines Chrome Extension API function exposed to Telemetry Extension and check availability of all routines",
		Contacts: []string{
			"lamzin@google.com",    // Test and Telemetry Extension author
			"bkersting@google.com", // Test author
			"cros-oem-services-team@google.com",
		},
		Attr:         []string{"group:telemetry_extension_hw"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.TelemetryExtension,
		Params: []testing.Param{
			// Available everywhere.
			{
				Name: "ac_power",
				Val:  "ac_power",
			},
			{
				Name: "cpu_cache",
				Val:  "cpu_cache",
			},
			{
				Name: "cpu_stress",
				Val:  "cpu_stress",
			},
			{
				Name: "cpu_floating_point_accuracy",
				Val:  "cpu_floating_point_accuracy",
			},
			{
				Name: "cpu_prime_search",
				Val:  "cpu_prime_search",
			},
			{
				Name: "lan_connectivity",
				Val:  "lan_connectivity",
			},
			{
				Name: "memory",
				Val:  "memory",
			},
			// Depend on a battery.
			{
				Name: "battery_health",
				Val:  "battery_health",
			},
			{
				Name: "battery_capacity",
				Val:  "battery_capacity",
			},
			{
				Name: "battery_discharge",
				Val:  "battery_discharge",
			},
			{
				Name: "battery_charge",
				Val:  "battery_charge",
			},
			// Depend on an NVMe capable disk.
			{
				Name: "nvme_wear_level",
				Val:  "nvme_wear_level",
			},
			// Depend on SMART support.
			{
				Name: "smart_ctl_check",
				Val:  "smartctl_check",
			},
			// Depend on FIO support.
			{
				Name: "disk_read",
				Val:  "disk_read",
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

	contains := func(list []string, want string) bool {
		for _, elem := range list {
			if elem == want {
				return true
			}
		}
		return false
	}

	if !contains(resp.Routines, routineName) {
		s.Errorf(`Unexpected result from "getAvailableRoutines": %q is not present in %v as expected`, routineName, resp.Routines)
	}
}
