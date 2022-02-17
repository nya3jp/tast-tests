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
		Func:         APIMemoryInfo,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Tests chrome.os.telemetry.getMemoryInfo Chrome Extension API function exposed to Telemetry Extension",
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
				Fixture:           "telemetryExtension",
				ExtraHardwareDeps: dep.TargetModels(),
			},
			{
				Name:              "low_priority_target_models",
				Fixture:           "telemetryExtension",
				ExtraHardwareDeps: dep.LowPriorityTargetModels(),
			},
			{
				Name:              "target_models_managed",
				Fixture:           "telemetryExtensionManaged",
				ExtraHardwareDeps: dep.TargetModels(),
			},
			{
				Name:              "low_priority_target_models_managed",
				Fixture:           "telemetryExtensionManaged",
				ExtraHardwareDeps: dep.LowPriorityTargetModels(),
			},
		},
	})
}

// APIMemoryInfo tests chrome.os.telemetry.getMemoryInfo Chrome Extension API functionality.
func APIMemoryInfo(ctx context.Context, s *testing.State) {
	v := s.FixtValue().(*fixture.Value)

	type response struct {
		TotalMemoryKiB          int64 `json:"totalMemoryKiB"`
		FreeMemoryKiB           int64 `json:"freeMemoryKiB"`
		AvailableMemoryKiB      int64 `json:"availableMemoryKiB"`
		PageFaultsSinceLastBoot int64 `json:"pageFaultsSinceLastBoot"`
	}

	var resp response
	if err := v.ExtConn.Call(ctx, &resp,
		"tast.promisify(chrome.os.telemetry.getMemoryInfo)",
	); err != nil {
		s.Fatal("Failed to get response from Telemetry extenion service worker: ", err)
	}

	s.Logf("Response: %+v", resp)
}
