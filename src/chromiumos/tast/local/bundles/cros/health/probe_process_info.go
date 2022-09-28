// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/jsontypes"
	"chromiumos/tast/testing"
)

type processInfo struct {
	BytesRead             jsontypes.Uint64 `json:"bytes_read"`
	BytesWritten          jsontypes.Uint64 `json:"bytes_written"`
	CancelledBytesWritten jsontypes.Uint64 `json:"cancelled_bytes_written"`
	Command               string           `json:"command"`
	FreeMemoryKiB         jsontypes.Uint32 `json:"free_memory_kib"`
	Name                  string           `json:"name"`
	Nice                  int8             `json:"nice"`
	ParentProcessID       jsontypes.Uint32 `json:"parent_process_id"`
	ProcessGroupID        jsontypes.Uint32 `json:"process_group_id"`
	ProcessID             jsontypes.Uint32 `json:"process_id"`
	PhysicalBytesRead     jsontypes.Uint64 `json:"physical_bytes_read"`
	PhysicalBytesWritten  jsontypes.Uint64 `json:"physical_bytes_written"`
	Priority              int8             `json:"priority"`
	ReadSystemCalls       jsontypes.Uint64 `json:"read_system_calls"`
	ResidentMemoryKiB     jsontypes.Uint32 `json:"resident_memory_kib"`
	State                 string           `json:"state"`
	Threads               jsontypes.Uint32 `json:"threads"`
	TotalMemoryKiB        jsontypes.Uint32 `json:"total_memory_kib"`
	UptimeTicks           jsontypes.Uint64 `json:"uptime_ticks"`
	UserID                jsontypes.Uint32 `json:"user_id"`
	WriteSystemCalls      jsontypes.Uint64 `json:"write_system_calls"`
}

type probeError struct {
	Msg       string `json:"msg"`
	ErrorType string `json:"type"`
}

type multipleProcessInfo struct {
	Errors       map[jsontypes.Uint32]probeError `json:"errors"`
	ProcessInfos map[string]*processInfo         `json:"process_infos"`
	//TODO: Define the correct error types. errors is so complicated...
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeProcessInfo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check that we can probe cros_healthd for single process info",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
		Timeout:      1 * time.Minute,
	})
}

// validateProcessInfo validates name, parent_process_id, command values by comparing with results from gopsutil.
func validateProcessInfo(info *processInfo, p *process.Process) error {
	if name, err := p.Name(); err != nil {
		return errors.Errorf("can't get process name with gopsutil: %s", err)
	} else if info.Name != name {
		return errors.Errorf("unexpected name; got %s, want %s", info.Name, name)
	}

	if ppid, err := p.Ppid(); err != nil {
		return errors.Errorf("can't get parent_process_id with gopsutil: %s", err)
	} else if int(info.ParentProcessID) != int(ppid) {
		return errors.Errorf("unexpected parent_process_id; got %d, want %d", int(info.ParentProcessID), int(ppid))
	}

	if command, err := p.Cmdline(); err != nil {
		return errors.Errorf("can't get process command with gopsutil: %s", err)
	} else if info.Command != command {
		return errors.Errorf("unexpected process command; got %s, want %s", info.Command, command)
	}

	return nil
}

// validateSingleProcessInfo tests that process info with pid=1 (init) can be successfully and correctly fetched.
func validateSingleProcessInfo(ctx context.Context, s *testing.State) error {
	params := croshealthd.TelemParams{PIDs: []int{1}}
	var info processInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &info); err != nil {
		return errors.Errorf("failed to get process telemetry info: %s", err)
	}

	p, err := process.NewProcess(1)
	if err != nil {
		return errors.Errorf("process with pid=1 does not exist: %s", err)
	}

	if err := validateProcessInfo(&info, p); err != nil {
		return errors.Errorf("failed to validate process info data: %s", err)
	}

	return nil
}

// validateMultipleProcessInfo tests that multiple process info with pid=1 (init) and pid=2 (kthreadd) can be successfully and correctly fetched.
func validateMultipleProcessInfo(ctx context.Context, s *testing.State) error {
	testingPids := []int{1, 2}
	params := croshealthd.TelemParams{PIDs: testingPids}
	var info multipleProcessInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &info); err != nil {
		return errors.Errorf("failed to get process telemetry info: %s", err)
	}

	for _, pid := range testingPids {
		if _, ok := info.ProcessInfos[strconv.Itoa(pid)]; !ok {
			return errors.Errorf("process with pid=%v is not captured by healthd", pid)
		}
	}

	pInit, err := process.NewProcess(1)
	if err != nil {
		return errors.Errorf("process with pid=1 does not exist: %s", err)
	}

	pKthreadd, err := process.NewProcess(2)
	if err != nil {
		return errors.Errorf("process with pid=2 does not exist: %s", err)
	}

	if err := validateProcessInfo(info.ProcessInfos["1"], pInit); err != nil {
		return errors.Errorf("failed to validate init process info data: %s", err)
	}

	if err := validateProcessInfo(info.ProcessInfos["2"], pKthreadd); err != nil {
		return errors.Errorf("failed to validate kthreadd process info data: %s", err)
	}

	return nil
}

// ProbeProcessInfo tests that different processes can be successfully and correctly fetched.
func ProbeProcessInfo(ctx context.Context, s *testing.State) {
	if err := validateSingleProcessInfo(ctx, s); err != nil {
		s.Fatal("Failed to validate single process data: ", err)
	}
	if err := validateMultipleProcessInfo(ctx, s); err != nil {
		s.Fatal("Failed to validate multiple process data: ", err)
	}
}
