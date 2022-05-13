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
		Func:         APICpuInfo,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Tests chrome.os.telemetry.getCpuInfo Chrome Extension API function exposed to Telemetry Extension",
		Contacts: []string{
			"lamzin@google.com", // Test and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"bkersting@google.com",
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
	Name                       string `json:"name"`
	TimeInStateSinceLastBootUs int64  `json:"timeInStateSinceLastBootUs"`
}

type logicalCPUInfo struct {
	MaxClockSpeedKhz           int64          `json:"maxClockSpeedKhz"`
	ScalingMaxFrequencyKhz     int64          `json:"scalingMaxFrequencyKhz"`
	ScalingCurrentFrequencyKhz int64          `json:"scalingCurrentFrequencyKhz"`
	IdleTimeMs                 int64          `json:"idleTimeMs"`
	CStates                    []cpuStateInfo `json:"cStates"`
}

type physicalCPUInfo struct {
	ModelName   string           `json:"modelName"`
	LogicalCpus []logicalCPUInfo `jsone:"logicalCpus"`
}

type response struct {
	NumTotalThreads int64             `json:"numTotalThreads"`
	Architecture    string            `json:"architecture"`
	PhysicalCpus    []physicalCPUInfo `json:"physicalCpus"`
}

// Types for parsing the JSON object returned by the cros health tool (Cht) command
// cros-health-tool telem --category=cpu
type cpuStateInfoCht struct {
	Name                       string `json:"name"`
	TimeInStateSinceLastBootUs int64  `json:"time_in_state_since_last_boot_us,string"`
}

type logicalCPUInfoCht struct {
	MaxClockSpeedKhz           int64             `json:"max_clock_speed_khz,string"`
	ScalingMaxFrequencyKhz     int64             `json:"scaling_max_frequency_khz,string"`
	ScalingCurrentFrequencyKhz int64             `json:"scaling_current_frequency_khz,string"`
	IdleTimeHz                 int64             `json:"idle_time_user_hz,string"`
	SystemTimeUserHz           int64             `json:"system_time_user_hz,string"`
	UserTimeUserHz             int64             `json:"user_time_user_hz,string"`
	CStates                    []cpuStateInfoCht `json:"c_states"`
}

type physicalCPUInfoCht struct {
	ModelName   string              `json:"model_name"`
	LogicalCpus []logicalCPUInfoCht `json:"logical_cpus"`
}

type responseCht struct {
	NumTotalThreads     int64                `json:"num_total_threads,string"`
	Architecture        string               `json:"architecture"`
	PhysicalCpus        []physicalCPUInfoCht `json:"physical_cpus"`
	KeylockerInfo       interface{}          `json:"-"`
	TemperatureChannels interface{}          `json:"-"`
}

// APICpuInfo tests chrome.os.telemetry.getCpuInfo Chrome Extension API functionality.
func APICpuInfo(ctx context.Context, s *testing.State) {
	v := s.FixtValue().(*fixture.Value)

	// get system info from cros-health-tool
	respCht, err := queryCrosHealthTool(ctx)
	if err != nil {
		s.Fatal("Failed to retrieve cros-health-tool information: ", err)
	}

	// get result from the telemetry API
	var resp response
	if err := v.ExtConn.Call(ctx, &resp,
		"tast.promisify(chrome.os.telemetry.getCpuInfo)",
	); err != nil {
		s.Fatal("Failed to get response from Telemetry extenion service worker: ", err)
	}

	// test if both results match
	if resp.Architecture != respCht.Architecture {
		s.Errorf("Unexpected CPU architecture: got %s; want %s", resp.Architecture, respCht.Architecture)
	}

	if resp.NumTotalThreads != respCht.NumTotalThreads {
		s.Errorf("Unexpected number of total threads: got %d, want %d", resp.NumTotalThreads, respCht.NumTotalThreads)
	}

	if len(resp.PhysicalCpus) != len(respCht.PhysicalCpus) {
		s.Errorf("Unexpected number of physical CPUs: got %d, want %d", len(resp.PhysicalCpus), len(respCht.PhysicalCpus))
		return
	}

	// check the physical cpus
	for i, pcpu := range resp.PhysicalCpus {
		wantedPcpu := respCht.PhysicalCpus[i]

		if pcpu.ModelName != wantedPcpu.ModelName {
			s.Errorf("Unexpected model name for CPU: want %s, got %s", pcpu.ModelName, wantedPcpu.ModelName)
		}

		if len(pcpu.LogicalCpus) != len(wantedPcpu.LogicalCpus) {
			s.Errorf("Unexpected number of logical CPUs: got %d, want %d", len(pcpu.LogicalCpus), len(wantedPcpu.LogicalCpus))
			return
		}

		// check the logical cpus
		for i, lcpu := range pcpu.LogicalCpus {
			wantedLcpu := wantedPcpu.LogicalCpus[i]

			if lcpu.MaxClockSpeedKhz != wantedLcpu.MaxClockSpeedKhz {
				s.Errorf("Unexpected value for MaxClockSpeedKhz: got: %d, want: %d", lcpu.MaxClockSpeedKhz, wantedLcpu.MaxClockSpeedKhz)
			}

			if lcpu.ScalingMaxFrequencyKhz != wantedLcpu.ScalingMaxFrequencyKhz {
				s.Errorf("Unexpected value for ScalingMaxFrequencyKhz: got: %d, want: %d", lcpu.ScalingMaxFrequencyKhz, wantedLcpu.ScalingMaxFrequencyKhz)
			}

			if len(lcpu.CStates) != len(wantedLcpu.CStates) {
				s.Errorf("Unexpected number of logical CPU CStates: got %d, want %d", len(lcpu.CStates), len(wantedLcpu.CStates))
				return
			}

			// check the logical cpu states
			for i, state := range lcpu.CStates {
				wantedState := wantedLcpu.CStates[i]

				if state.Name != wantedState.Name {
					s.Errorf("Unexpected CState name: got %s, want %s", state.Name, wantedState.Name)
				}
			}
		}
	}
}

// queryCrosHealthTool invokes the cros-health-tool and parses the output (JSON) into an object
func queryCrosHealthTool(ctx context.Context) (*responseCht, error) {
	bytes, err := testexec.CommandContext(ctx, crosHealthToolCmd, crosHealthToolArgs...).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get data from cros-health-tool")
	}

	var response responseCht
	if err = json.Unmarshal(bytes, &response); err != nil {
		return nil, errors.Wrap(err, "failed to parse cros-health-tool json response")
	}

	return &response, nil
}
