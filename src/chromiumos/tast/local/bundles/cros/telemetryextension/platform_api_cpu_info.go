// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package telemetryextension

import (
	"context"
	"encoding/json"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/telemetryextension/dep"
	"chromiumos/tast/local/bundles/cros/telemetryextension/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlatformAPICPUInfo,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests chrome.os.telemetry.getCpuInfo Chrome Extension API function exposed to Telemetry Extension",
		Contacts: []string{
			"lamzin@google.com",    // Test and Telemetry Extension author
			"mgawad@google.com",    // Telemetry Extension author
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

// Types for parsing the JSON object returned by os.telemetry.getCpuInfo.
type cpuStateInfo struct {
	Name string `json:"name"`
}

type logicalCPUInfo struct {
	MaxClockSpeedKhz       int64          `json:"maxClockSpeedKhz"`
	ScalingMaxFrequencyKhz int64          `json:"scalingMaxFrequencyKhz"`
	CStates                []cpuStateInfo `json:"cStates"`
}

type physicalCPUInfo struct {
	ModelName   string           `json:"modelName"`
	LogicalCPUs []logicalCPUInfo `json:"logicalCpus"`
}

type response struct {
	NumTotalThreads int64             `json:"numTotalThreads"`
	Architecture    string            `json:"architecture"`
	PhysicalCPUs    []physicalCPUInfo `json:"physicalCpus"`
}

// Types for parsing the JSON object returned by the cros health tool (CHT) command:
// cros-health-tool telem --category=cpu
type cpuStateInfoCHT struct {
	Name string `json:"name"`
}

type logicalCPUInfoCHT struct {
	MaxClockSpeedKhz       int64             `json:"max_clock_speed_khz,string"`
	ScalingMaxFrequencyKhz int64             `json:"scaling_max_frequency_khz,string"`
	CStates                []cpuStateInfoCHT `json:"c_states"`
}

type physicalCPUInfoCHT struct {
	ModelName   string              `json:"model_name"`
	LogicalCPUs []logicalCPUInfoCHT `json:"logical_cpus"`
}

type responseCHT struct {
	NumTotalThreads int64                `json:"num_total_threads,string"`
	Architecture    string               `json:"architecture"`
	PhysicalCPUs    []physicalCPUInfoCHT `json:"physical_cpus"`
}

// PlatformAPICPUInfo tests chrome.os.telemetry.getCpuInfo Chrome Extension API functionality.
func PlatformAPICPUInfo(ctx context.Context, s *testing.State) {
	v := s.FixtValue().(*fixture.Value)

	// get system info from cros-health-tool.
	wantResp, err := queryCrosHealthTool(ctx)
	if err != nil {
		s.Fatal("Failed to retrieve cros-health-tool information: ", err)
	}

	// get result from the telemetry API.
	var gotResp response
	if err := v.ExtConn.Call(ctx, &gotResp,
		"tast.promisify(chrome.os.telemetry.getCpuInfo)",
	); err != nil {
		s.Fatal("Failed to get response from Telemetry extenion service worker: ", err)
	}

	// test if both results match.
	if gotResp.Architecture != wantResp.Architecture {
		s.Errorf("Unexpected CPU architecture: got %q; want %q", gotResp.Architecture, wantResp.Architecture)
	}

	if gotResp.NumTotalThreads != wantResp.NumTotalThreads {
		s.Errorf("Unexpected number of total threads: got %d; want %d", gotResp.NumTotalThreads, wantResp.NumTotalThreads)
	}

	if len(gotResp.PhysicalCPUs) != len(wantResp.PhysicalCPUs) {
		s.Errorf("Unexpected number of physical CPUs: got %d; want %d", len(gotResp.PhysicalCPUs), len(wantResp.PhysicalCPUs))
		return
	}

	// check the physical CPUs.
	for i, gotPCPU := range gotResp.PhysicalCPUs {
		wantPCPU := wantResp.PhysicalCPUs[i]

		if gotPCPU.ModelName != wantPCPU.ModelName {
			s.Errorf("Unexpected CPU model name: want %s; got %s", gotPCPU.ModelName, wantPCPU.ModelName)
		}

		if len(gotPCPU.LogicalCPUs) != len(wantPCPU.LogicalCPUs) {
			s.Errorf("Unexpected number of logical CPUs: got %d; want %d", len(gotPCPU.LogicalCPUs), len(wantPCPU.LogicalCPUs))
			return
		}

		// check the logical CPUs.
		for i, gotLCPU := range gotPCPU.LogicalCPUs {
			wantLCPU := wantPCPU.LogicalCPUs[i]

			if gotLCPU.MaxClockSpeedKhz != wantLCPU.MaxClockSpeedKhz {
				s.Errorf("Unexpected value for MaxClockSpeedKhz: got: %d; want: %d", gotLCPU.MaxClockSpeedKhz, wantLCPU.MaxClockSpeedKhz)
			}

			if gotLCPU.ScalingMaxFrequencyKhz != wantLCPU.ScalingMaxFrequencyKhz {
				s.Errorf("Unexpected value for ScalingMaxFrequencyKhz: got: %d; want: %d", gotLCPU.ScalingMaxFrequencyKhz, wantLCPU.ScalingMaxFrequencyKhz)
			}

			if len(gotLCPU.CStates) != len(wantLCPU.CStates) {
				s.Errorf("Unexpected number of logical CPU CStates: got %d; want %d", len(gotLCPU.CStates), len(wantLCPU.CStates))
				return
			}

			// check the logical CPU states.
			for i, gotState := range gotLCPU.CStates {
				wantState := wantLCPU.CStates[i]

				if gotState.Name != wantState.Name {
					s.Errorf("Unexpected CState name: got %s; want %s", gotState.Name, wantState.Name)
				}
			}
		}
	}
}

// queryCrosHealthTool invokes the cros-health-tool and parses the output (JSON) into an object.
func queryCrosHealthTool(ctx context.Context) (*responseCHT, error) {
	bytes, err := testexec.CommandContext(ctx, "cros-health-tool", "telem", "--category=cpu").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get data from cros-health-tool")
	}

	var response responseCHT
	if err = json.Unmarshal(bytes, &response); err != nil {
		return nil, errors.Wrap(err, "failed to parse cros-health-tool JSON response")
	}

	return &response, nil
}
