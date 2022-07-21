// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
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

// compareValueWithPsOutput runs ps command and compares results with cros_healthd.
func compareValueWithPsOutput(ctx context.Context, command, got string) error {
	if want, err := testexec.CommandContext(ctx, "ps", "-p", "1", "-o", command).Output(); err != nil {
		return errors.Errorf("ps command failed to run: %s", err)
	} else if trimmedWant := strings.TrimSpace(string(want)); trimmedWant != got {
		return errors.Errorf("value doesn't match: got %s; want %s", got, trimmedWant)
	}

	return nil
}

// validateProcessInfo validates name, parent_process_id, process_group_id, command values.
func validateProcessInfo(ctx context.Context, info *processInfo) error {
	if err := compareValueWithPsOutput(ctx, "comm=", info.Name); err != nil {
		return errors.Wrap(err, "name")
	}
	ppidString, _ := json.Marshal(info.ParentProcessID)
	if err := compareValueWithPsOutput(ctx, "ppid=", string(ppidString)); err != nil {
		return errors.Wrap(err, "parent_process_id")
	}
	pgidString, _ := json.Marshal(info.ProcessGroupID)
	if err := compareValueWithPsOutput(ctx, "pgid=", string(pgidString)); err != nil {
		return errors.Wrap(err, "process_group_id")
	}
	if err := compareValueWithPsOutput(ctx, "args=", info.Command); err != nil {
		return errors.Wrap(err, "command")
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

	if err := validateProcessInfo(ctx, &info); err != nil {
		s.Fatal("Failed to validate process info data: ", err)
	}
}
