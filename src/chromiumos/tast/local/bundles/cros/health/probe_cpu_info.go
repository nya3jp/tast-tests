// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

type temperatureChannelInfo struct {
	Label              *string `json:"label"`
	TemperatureCelsius int     `json:"temperature_celsius"`
}

type cStateInfo struct {
	Name                       string `json:"name"`
	TimeInStateSinceLastBootUs string `json:"time_in_state_since_last_boot_us"`
}

type logicalCPUInfo struct {
	UserTimeUserHz             string       `json:"user_time_user_hz"`
	SystemTimeUserHz           string       `json:"system_time_user_hz"`
	MaxClockSpeedKhz           int          `json:"max_clock_speed_khz"`
	ScalingMaxFrequencyKhz     int          `json:"scaling_max_frequency_khz"`
	ScalingCurrentFrequencyKhz int          `json:"scaling_current_frequency_khz"`
	IdleTimeUserHz             int          `json:"idle_time_user_hz"`
	CStates                    []cStateInfo `json:"c_states"`
}

type physicalCPUInfo struct {
	ModelName   *string          `json:"model_name"`
	LogicalCPUs []logicalCPUInfo `json:"logical_cpus"`
}

type cpuInfo struct {
	Architecture        string                   `json:"architecture"`
	NumTotalThreads     int                      `json:"num_total_threads"`
	TemperatureChannels []temperatureChannelInfo `json:"temperature_channels"`
	PhysicalCPUs        []physicalCPUInfo        `json:"physical_cpus"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeCPUInfo,
		Desc: "Check that we can probe cros_healthd for CPU info",
		Contacts: []string{
			"khegde@google.com",
			"pmoy@google.com",
			"cros-tdm@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func verifyPhysicalCPU(physicalCPU physicalCPUInfo) error {
	if len(physicalCPU.LogicalCPUs) < 1 {
		return errors.New("can't find any logical cpu info")
	}

	for _, logicalCPU := range physicalCPU.LogicalCPUs {
		if err := verifyLogicalCPU(logicalCPU); err != nil {
			return errors.Wrap(err, "failed to verify logical CPU")
		}
	}

	return nil
}

func verifyLogicalCPU(logicalCPU logicalCPUInfo) error {
	if parsed, err := strconv.ParseUint(logicalCPU.UserTimeUserHz, 10, 64); err != nil {
		return errors.Wrapf(err, "failed to convert user_time_user_hz to uint64: %q", logicalCPU.UserTimeUserHz)
	} else if parsed < 0 {
		return errors.Errorf("invalid user_time_user_hz: %v", parsed)
	}
	if parsed, err := strconv.ParseUint(logicalCPU.SystemTimeUserHz, 10, 64); err != nil {
		return errors.Wrapf(err, "failed to convert system_time_user_hz to uint64: %q", logicalCPU.SystemTimeUserHz)
	} else if parsed < 0 {
		return errors.Errorf("invalid system_time_user_hz: %v", parsed)
	}

	if logicalCPU.MaxClockSpeedKhz < 0 {
		return errors.Errorf("invalid max_clock_speed_khz: %v", logicalCPU.MaxClockSpeedKhz)
	}
	if logicalCPU.ScalingMaxFrequencyKhz < 0 {
		return errors.Errorf("invalid scaling_max_frequency_khz: %v", logicalCPU.ScalingMaxFrequencyKhz)
	}
	if logicalCPU.ScalingCurrentFrequencyKhz < 0 {
		return errors.Errorf("invalid scaling_current_frequency_khz: %v", logicalCPU.ScalingCurrentFrequencyKhz)
	}
	if logicalCPU.IdleTimeUserHz < 0 {
		return errors.Errorf("invalid idle_time_user_hz: %v", logicalCPU.IdleTimeUserHz)
	}

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

	if parsed, err := strconv.ParseInt(cState.TimeInStateSinceLastBootUs, 10, 64); err != nil {
		return errors.Wrapf(err, "failed to convert time_in_state_since_last_boot_us to integer: %q", cState.TimeInStateSinceLastBootUs)
	} else if parsed < 0 {
		return errors.Errorf("invalid time_in_state_since_last_boot_us: %d", parsed)
	}

	return nil
}

func validateCPUData(info cpuInfo) error {
	// Every board should have at least one physical CPU
	if len(info.PhysicalCPUs) < 1 {
		return errors.New("can't find any physical cpu info")
	}

	if info.NumTotalThreads <= 0 {
		return errors.New("invalid num_total_threads")
	}
	if info.Architecture == "" {
		return errors.New("Empty architecture")
	}

	for _, physicalCPU := range info.PhysicalCPUs {
		if err := verifyPhysicalCPU(physicalCPU); err != nil {
			return errors.Wrap(err, "failed to verify physical CPU")
		}
	}

	return nil
}

func ProbeCPUInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryCPU}
	rawData, err := croshealthd.RunTelem(ctx, params, s.OutDir())
	if err != nil {
		s.Fatal("Failed to run telem command: ", err)
	}

	dec := json.NewDecoder(strings.NewReader(string(rawData)))
	dec.DisallowUnknownFields()

	var info cpuInfo
	if err := dec.Decode(&info); err != nil {
		s.Fatalf("Failed to decode cpu data [%q], err [%v]", rawData, err)
	}

	if err := validateCPUData(info); err != nil {
		s.Fatalf("Failed to validate cpu data, err [%v]", err)
	}
}
