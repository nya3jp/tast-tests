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

var (
	crosHealthToolCmd  = "cros-health-tool"
	crosHealthToolArgs = []string{"telem", "--category=cpu"}
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         APICPUInfo,
		LacrosStatus: testing.LacrosVariantNeeded,
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

// Types for parsing the JSON object returned by os.telemetry.getCpuInfo
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

// Types for parsing the JSON object returned by the cros health tool (Cht) command
// cros-health-tool telem --category=cpu
type cpuStateInfoCht struct {
	Name string `json:"name"`
}

type logicalCPUInfoCht struct {
	MaxClockSpeedKhz       int64             `json:"max_clock_speed_khz,string"`
	ScalingMaxFrequencyKhz int64             `json:"scaling_max_frequency_khz,string"`
	CStates                []cpuStateInfoCht `json:"c_states"`
}

type physicalCPUInfoCht struct {
	ModelName   string              `json:"model_name"`
	LogicalCPUs []logicalCPUInfoCht `json:"logical_cpus"`
}

type responseCht struct {
	NumTotalThreads int64                `json:"num_total_threads,string"`
	Architecture    string               `json:"architecture"`
	PhysicalCPUs    []physicalCPUInfoCht `json:"physical_cpus"`
}

// APICPUInfo tests chrome.os.telemetry.getCpuInfo Chrome Extension API functionality.
func APICPUInfo(ctx context.Context, s *testing.State) {
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
		s.Errorf("Unexpected CPU architecture: got %s; want %s", gotResp.Architecture, wantResp.Architecture)
	}

	if gotResp.NumTotalThreads != wantResp.NumTotalThreads {
		s.Errorf("Unexpected number of total threads: got %d; want %d", gotResp.NumTotalThreads, wantResp.NumTotalThreads)
	}

	if len(gotResp.PhysicalCPUs) != len(wantResp.PhysicalCPUs) {
		s.Errorf("Unexpected number of physical CPUs: got %d; want %d", len(gotResp.PhysicalCPUs), len(wantResp.PhysicalCPUs))
		return
	}

	// check the physical cpus.
	for i, pcpu := range gotResp.PhysicalCPUs {
		wantedPcpu := wantResp.PhysicalCPUs[i]

		if pcpu.ModelName != wantedPcpu.ModelName {
			s.Errorf("Unexpected CPU model name: want %s; got %s", pcpu.ModelName, wantedPcpu.ModelName)
		}

		if len(pcpu.LogicalCPUs) != len(wantedPcpu.LogicalCPUs) {
			s.Errorf("Unexpected number of logical CPUs: got %d; want %d", len(pcpu.LogicalCPUs), len(wantedPcpu.LogicalCPUs))
			return
		}

		// check the logical cpus.
		for i, lcpu := range pcpu.LogicalCPUs {
			wantedLcpu := wantedPcpu.LogicalCPUs[i]

			if lcpu.MaxClockSpeedKhz != wantedLcpu.MaxClockSpeedKhz {
				s.Errorf("Unexpected value for MaxClockSpeedKhz: got: %d; want: %d", lcpu.MaxClockSpeedKhz, wantedLcpu.MaxClockSpeedKhz)
			}

			if lcpu.ScalingMaxFrequencyKhz != wantedLcpu.ScalingMaxFrequencyKhz {
				s.Errorf("Unexpected value for ScalingMaxFrequencyKhz: got: %d; want: %d", lcpu.ScalingMaxFrequencyKhz, wantedLcpu.ScalingMaxFrequencyKhz)
			}

			if len(lcpu.CStates) != len(wantedLcpu.CStates) {
				s.Errorf("Unexpected number of logical CPU CStates: got %d; want %d", len(lcpu.CStates), len(wantedLcpu.CStates))
				return
			}

			// check the logical cpu states.
			for i, state := range lcpu.CStates {
				wantedState := wantedLcpu.CStates[i]

				if state.Name != wantedState.Name {
					s.Errorf("Unexpected CState name: got %s; want %s", state.Name, wantedState.Name)
				}
			}
		}
	}
}

// queryCrosHealthTool invokes the cros-health-tool and parses the output (JSON) into an object.
func queryCrosHealthTool(ctx context.Context) (*responseCht, error) {
	bytes, err := testexec.CommandContext(ctx, crosHealthToolCmd, crosHealthToolArgs...).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get data from cros-health-tool")
	}

	var response responseCht
	if err = json.Unmarshal(bytes, &response); err != nil {
		return nil, errors.Wrap(err, "failed to parse cros-health-tool JSON response")
	}

	return &response, nil
}
