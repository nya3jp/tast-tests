// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"bytes"
	"context"
	"regexp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MonitorBluetoothEvent,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Monitors the Bluetooth event detected properly or not",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "crosHealthdRunning",
	})
}

func MonitorBluetoothEvent(ctx context.Context, s *testing.State) {
	// Run monitor command in background.
	var stdoutBuf, stderrBuf bytes.Buffer
	monitorCmd := testexec.CommandContext(ctx, "cros-health-tool", "event", "--category=bluetooth", "--length_seconds=10")
	monitorCmd.Stdout = &stdoutBuf
	monitorCmd.Stderr = &stderrBuf

	if err := monitorCmd.Start(); err != nil {
		s.Fatal("Failed to run healthd monitor command: ", err)
	}

	for _, action := range []string{"off", "on"} {
		// Trigger USB event.
		if err := testexec.CommandContext(ctx, "bluetoothctl", "power", action).Run(); err != nil {
			s.Fatal("Failed to trigger Bluetooth power ", action, " event: ", err)
		}

		monitorCmd.Wait()

		stderr := string(stderrBuf.Bytes())
		if stderr != "" {
			s.Fatal("Failed to detect Bluetooth ", action, " event, stderr: ", stderr)
		}

		stdout := string(stdoutBuf.Bytes())
		bluetoothEventPattern := regexp.MustCompile("Bluetooth event received")
		if !bluetoothEventPattern.MatchString(stdout) {
			s.Fatal("Failed to detect Bluetooth ", action, " event, event output: ", stdout)
		}
	}
}
