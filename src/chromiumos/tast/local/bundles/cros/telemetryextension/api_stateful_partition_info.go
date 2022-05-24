// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package telemetryextension

import (
	"context"
	"encoding/json"
	"math"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/telemetryextension/dep"
	"chromiumos/tast/local/bundles/cros/telemetryextension/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         APIStatefulPartitionInfo,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Tests chrome.os.diagnostics.getStatefulPartitionInfo Chrome Extension API function exposed to Telemetry Extension",
		Contacts: []string{
			"bkersting@google.com", // Test author
			"lamzin@google.com",    // Telemetry Extension author
			"mgawad@google.com",    // Telemetry Extension author
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

func APIStatefulPartitionInfo(ctx context.Context, s *testing.State) {
	v := s.FixtValue().(*fixture.Value)

	// get the actual data from cros-health-tool.
	wantResp, err := getCrosHealthToolPartitionInfo(ctx)
	if err != nil {
		s.Fatal("Failed to retrieve cros-health-tool information: ", err)
	}

	// available space is rounded down to the next 100MB.
	const c100MB = 100 * 1024 * 1024
	wantResp.AvailableSpace = math.Floor(wantResp.AvailableSpace/c100MB) * c100MB

	// get result from the telemetry API.
	type response struct {
		AvailableSpace float64 `json:"availableSpace"`
		TotalSpace     float64 `json:"totalSpace"`
	}

	var gotResp response
	if err := v.ExtConn.Call(ctx, &gotResp,
		"tast.promisify(chrome.os.telemetry.getStatefulPartitionInfo)",
	); err != nil {
		s.Fatal("Failed to get response from Telemetry extenion service worker: ", err)
	}

	// test if both results match.
	if gotResp.TotalSpace != wantResp.TotalSpace {
		s.Errorf("Unexpected total space: got %f; want %f", gotResp.TotalSpace, wantResp.TotalSpace)
	}

	if gotResp.AvailableSpace != wantResp.AvailableSpace {
		s.Errorf("Unexpected available space: got %f; want %f", gotResp.AvailableSpace, wantResp.AvailableSpace)
	}
}

// Type for parsing the JSON object returned by the cros health tool (CHT) command:
// cros-health-tool telem --category=stateful_partition
type partitionInfoCHT struct {
	AvailableSpace float64 `json:"available_space,string"`
	TotalSpace     float64 `json:"total_space,string"`
}

func getCrosHealthToolPartitionInfo(ctx context.Context) (*partitionInfoCHT, error) {
	bytes, err := testexec.CommandContext(ctx, "cros-health-tool", "telem", "--category=stateful_partition").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get data from cros-health-tool")
	}

	var response partitionInfoCHT
	if err = json.Unmarshal(bytes, &response); err != nil {
		return nil, errors.Wrap(err, "failed to parse cros-health-tool JSON response")
	}

	return &response, nil
}
