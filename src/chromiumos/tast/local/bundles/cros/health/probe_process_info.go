// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/jsontypes"
	"chromiumos/tast/testing"
)

type processInfo struct {
	ByteRead              jsontypes.Uint64 `json:"bytes_read"`
	BytesWritten          jsontypes.Uint64 `json:"bytes_written"`
	CancelledBytesWritten jsontypes.Uint64 `json:"cancelled_bytes_written"`
	Command               string           `json:"command"`
	FreeMemoryKiB         jsontypes.Uint32 `json:"free_memory_kib"`
	Name                  string           `json:"name"`
	Nice                  int8             `json:"nice"`
	ParentProcessID       jsontypes.Uint32 `json:"parent_process_id"`
	ProcessGroupID        jsontypes.Uint32 `json:"process_group_id"`
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

// validateProcessInfo validates process name, ppid, pgid, process command with ps command results.
func validateProcessInfo(ctx context.Context, info *processInfo) error {
	name, err := testexec.CommandContext(ctx, "ps", "-p", "1", "-o", "comm=").Output()
	if err != nil {
		return errors.Errorf("failed to run ps command. (process name) %s", err)
	} else if strings.TrimSpace(string(name)) != info.Name {
		return errors.Errorf("process name doesn't match: got %s; want %s", info.Name, strings.TrimSpace(string(name)))
	}

	ppid, err := testexec.CommandContext(ctx, "ps", "-p", "1", "-o", "ppid=").Output()
	ppidString, _ := json.Marshal(info.ParentProcessID)
	if err != nil {
		return errors.Errorf("failed to run ps command. (ppid) %s", err)
	} else if strings.TrimSpace(string(ppid)) != string(ppidString) {
		return errors.Errorf("parent process id doesn't match: got %s; want %s", string(ppidString), strings.TrimSpace(string(ppid)))
	}

	pgid, err := testexec.CommandContext(ctx, "ps", "-p", "1", "-o", "pgid=").Output()
	pgidString, _ := json.Marshal(info.ProcessGroupID)
	if err != nil {
		return errors.Errorf("failed to run ps command. (pgid) %s", err)
	} else if strings.TrimSpace(string(pgid)) != string(pgidString) {
		return errors.Errorf("process group id doesn't match: got %s; want %s", string(pgidString), strings.TrimSpace(string(pgid)))
	}

	cmd, err := testexec.CommandContext(ctx, "ps", "-p", "1", "-o", "args=").Output()
	if err != nil {
		return errors.Errorf("failed to run ps command. (command) %ss", err)
	} else if strings.TrimSpace(string(cmd)) != info.Command {
		return errors.Errorf("process command doesn't match: got %s; want %s", info.Command, strings.TrimSpace(string(cmd)))
	}
	return nil
}

// validateProcFile validates needed /proc files (/stat, /statam, /status, /cmdline) can be read.
func validateProcFile() error {
	if _, err := ioutil.ReadFile("/proc/1/stat"); err != nil {
		return errors.Errorf("failed to read /proc/1/stat file: %s", err)
	}

	if _, err := ioutil.ReadFile("/proc/1/statm"); err != nil {
		return errors.Errorf("failed to read /proc/1/statm file: %s", err)
	}

	if _, err := ioutil.ReadFile("/proc/1/status"); err != nil {
		return errors.Errorf("failed to read /proc/1/status file: %s", err)
	}

	if _, err := ioutil.ReadFile("/proc/1/cmdline"); err != nil {
		return errors.Errorf("failed to read /proc/1/cmdline file: %s", err)
	}
	return nil
}

// ProbeProcessInfo tests process info with pid=1 (init) can be successfully fetched.
func ProbeProcessInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{PID: 1}
	var info processInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &info); err != nil {
		s.Fatal("Failed to get process telemetry info: ", err)
	}

	if err := validateProcFile(); err != nil {
		s.Fatal("Failed to read /proc files: ", err)
	}

	if err := validateProcessInfo(ctx, &info); err != nil {
		s.Fatal("Failed to validate process info data: ", err)
	}
}
