// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/jsontypes"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type temperatureChannelInfo struct {
	Label              *string `json:"label"`
	TemperatureCelsius int32   `json:"temperature_celsius"`
}

type cStateInfo struct {
	Name                       string           `json:"name"`
	TimeInStateSinceLastBootUs jsontypes.Uint64 `json:"time_in_state_since_last_boot_us"`
}

type logicalCPUInfo struct {
	UserTimeUserHz             jsontypes.Uint64 `json:"user_time_user_hz"`
	SystemTimeUserHz           jsontypes.Uint64 `json:"system_time_user_hz"`
	MaxClockSpeedKhz           jsontypes.Uint32 `json:"max_clock_speed_khz"`
	ScalingMaxFrequencyKhz     jsontypes.Uint32 `json:"scaling_max_frequency_khz"`
	ScalingCurrentFrequencyKhz jsontypes.Uint32 `json:"scaling_current_frequency_khz"`
	IdleTimeUserHz             jsontypes.Uint64 `json:"idle_time_user_hz"`
	CStates                    []cStateInfo     `json:"c_states"`
}

type physicalCPUInfo struct {
	ModelName   *string          `json:"model_name"`
	LogicalCPUs []logicalCPUInfo `json:"logical_cpus"`
}
type keylockerinfo struct {
	KeylockerConfigured bool `json:"keylocker_configured"`
}
type cpuInfo struct {
	Architecture        string                   `json:"architecture"`
	NumTotalThreads     jsontypes.Uint32         `json:"num_total_threads"`
	TemperatureChannels []temperatureChannelInfo `json:"temperature_channels"`
	PhysicalCPUs        []physicalCPUInfo        `json:"physical_cpus"`
	KeylockerInfo       keylockerinfo            `json:"keylocker_info"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeCPUInfo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check that we can probe cros_healthd for CPU info",
		Contacts: []string{
			"cros-tdm-tpe-eng@google.com",
			"pathan.jilani@intel.com",
			"intel-chrome-system-automation-team@intel.com",
		},
		Attr: []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics",
			// TODO(b/210950844): Reenable after plumbing through cpu frequency info.
			"no_manatee"},
		Fixture: "crosHealthdRunning",
		Params: []testing.Param{{
			Val: false,
		}, {
			Name:      "keylocker",
			Val:       true,
			ExtraAttr: []string{"informational"},
			// TODO(b/207569436): Define hardware dependency and get rid of hard-coding the models.
			ExtraHardwareDeps: hwdep.D(hwdep.Model("brya", "redrix", "kano", "anahera", "primus", "crota")),
		}},
	})
}

func verifyPhysicalCPU(physicalCPU *physicalCPUInfo) error {
	if len(physicalCPU.LogicalCPUs) < 1 {
		return errors.Errorf("invalid LogicalCPUs, got %d; want 1+", len(physicalCPU.LogicalCPUs))
	}

	for _, logicalCPU := range physicalCPU.LogicalCPUs {
		if err := verifyLogicalCPU(&logicalCPU); err != nil {
			return errors.Wrap(err, "failed to verify logical CPU")
		}
	}

	if *physicalCPU.ModelName == "" {
		return errors.New("empty CPU model name")
	}

	return nil
}

func verifyLogicalCPU(logicalCPU *logicalCPUInfo) error {
	for _, cState := range logicalCPU.CStates {
		if err := verifyCState(cState); err != nil {
			return errors.Wrap(err, "failed to verify c_state")
		}
	}

	return nil
}

func verifyCState(cState cStateInfo) error {
	if cState.Name == "" {
		return errors.New("empty name")
	}

	return nil
}

func validateCPUData(info *cpuInfo) error {
	// Every board should have at least one physical CPU
	if len(info.PhysicalCPUs) < 1 {
		return errors.Errorf("invalid PhysicalCPUs, got %d; want 1+", len(info.PhysicalCPUs))
	}

	if info.NumTotalThreads <= 0 {
		return errors.Errorf("invalid NumTotalThreads, got %d; want 1+", info.NumTotalThreads)
	}
	if info.Architecture == "" {
		return errors.New("empty architecture")
	}

	for _, physicalCPU := range info.PhysicalCPUs {
		if err := verifyPhysicalCPU(&physicalCPU); err != nil {
			return errors.Wrap(err, "failed to verify physical CPU")
		}
	}

	return nil
}

func validateKeyLocker(keylocker *keylockerinfo) error {
	if !keylocker.KeylockerConfigured {
		return errors.Errorf("failed to configure keylocker: %t", keylocker)
	}
	return nil
}

func ProbeCPUInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryCPU}
	var info cpuInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &info); err != nil {
		s.Fatal("Failed to run telem command: ", err)
	}

	if err := validateCPUData(&info); err != nil {
		s.Fatalf("Failed to validate cpu data, err [%v]", err)
	}

	if s.Param().(bool) {
		if err := validateKeyLocker(&info.KeylockerInfo); err != nil {
			s.Fatal("Failed to validate KeyLocker: ", err)
		}
	}
}
