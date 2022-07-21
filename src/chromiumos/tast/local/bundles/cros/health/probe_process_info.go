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

// psCommandHelper runs ps command and compare results with cros_healthd.
func psCommandHelper(ctx context.Context, info *processInfo, command, got string) error {
	want, err := testexec.CommandContext(ctx, "ps", "-p", "1", "-o", command).Output()
	if err != nil {
		return errors.Errorf("failed to run ps command. %s", err)
	} else if strings.TrimSpace(string(want)) != got {
		return errors.Errorf("value doesn't match: got %s; want %s", got, strings.TrimSpace(string(want)))
	}
	return nil
}

// validateProcessInfo validates process name, ppid, pgid, process command values.
func validateProcessInfo(ctx context.Context, info *processInfo) error {
	err := psCommandHelper(ctx, info, "comm=", info.Name)
	if err != nil {
		return err
	}
	ppidString, _ := json.Marshal(info.ParentProcessID)
	err = psCommandHelper(ctx, info, "ppid=", string(ppidString))
	if err != nil {
		return err
	}
	pgidString, _ := json.Marshal(info.ProcessGroupID)
	err = psCommandHelper(ctx, info, "pgid=", string(pgidString))
	if err != nil {
		return err
	}
	err = psCommandHelper(ctx, info, "args=", info.Command)
	if err != nil {
		return err
	}
	return nil
}

// readFileHelper uses ioutil to
func readFileHelper(path string) error {
	if _, err := ioutil.ReadFile(path); err != nil {
		return errors.Errorf("failed to read %s file: %s", path, err)
	}
	return nil
}

// validateProcFile validates needed /proc files (/stat, /statam, /status, /cmdline) can be read.
func validateProcFile() error {
	if err := readFileHelper("/proc/1/stat"); err != nil {
		return err
	}
	if err := readFileHelper("/proc/1/statm"); err != nil {
		return err
	}
	if err := readFileHelper("/proc/1/status"); err != nil {
		return err
	}
	if err := readFileHelper("/proc/1/cmdline"); err != nil {
		return err
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
