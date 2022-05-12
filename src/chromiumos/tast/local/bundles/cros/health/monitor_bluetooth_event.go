// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"bytes"
	"context"
	"strings"

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
	// Set the power off first.
	b, err := testexec.CommandContext(ctx, "bluetoothctl", "power", "off").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to trigger Bluetooth power off event: ", err)
	}
	s.Log("bluetoothctl: ", strings.Trim(string(b), "\n"))

	// Run monitor command in background.
	var stdoutBuf, stderrBuf bytes.Buffer
	monitorCmd := testexec.CommandContext(ctx, "cros-health-tool", "event", "--category=bluetooth", "--length_seconds=3")
	monitorCmd.Stdout = &stdoutBuf
	monitorCmd.Stderr = &stderrBuf

	if err := monitorCmd.Start(); err != nil {
		s.Fatal("Failed to run healthd monitor command: ", err)
	}

	// Trigger Bluetooth event.
	b, err = testexec.CommandContext(ctx, "bluetoothctl", "power", "on").Output(testexec.DumpLogOnError)
	if err != nil {
		if cmdErr := monitorCmd.Kill(); cmdErr != nil {
			s.Log(ctx, "Error killing healthd monitor command: ", cmdErr)
		}
		monitorCmd.Wait()
		s.Fatal("Failed to trigger Bluetooth power on event: ", err)
	}
	s.Log("bluetoothctl: ", strings.Trim(string(b), "\n"))

	if err := monitorCmd.Wait(); err != nil {
		s.Fatal("Failed to wait healthd monitor command: ", err)
	}

	stderr := string(stderrBuf.Bytes())
	if stderr != "" {
		s.Fatal("Failed to detect Bluetooth on event, stderr: ", stderr)
	}

	stdout := string(stdoutBuf.Bytes())
	if !strings.Contains(stdout, "Bluetooth event received") {
		s.Fatal("Failed to detect Bluetooth on event, event output: ", stdout)
	}
}
