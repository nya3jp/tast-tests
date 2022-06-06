// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"bytes"
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MonitorBluetoothEvent,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Monitors the Bluetooth event detected properly or not",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
		HardwareDeps: hwdep.D(hwdep.Bluetooth()),
	})
}

func initiateBluetoothStatus(ctx context.Context, s *testing.State) error {
	// Set the power off first.
	b, err := testexec.CommandContext(ctx, "bluetoothctl", "power", "off").Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "failed to trigger Bluetooth power off: %s", string(b))
	}
	s.Log("bluetoothctl: ", strings.Trim(string(b), "\n"))

	return nil
}

func MonitorBluetoothEvent(ctx context.Context, s *testing.State) {
	if err := initiateBluetoothStatus(ctx, s); err != nil {
		s.Fatal("Failed to initiate bluetooth status, err: ", err)
	}

	// Run monitor command in background.
	var stdoutBuf, stderrBuf bytes.Buffer
	monitorCmd := testexec.CommandContext(ctx, "cros-health-tool", "event", "--category=bluetooth", "--length_seconds=3")
	monitorCmd.Stdout = &stdoutBuf
	monitorCmd.Stderr = &stderrBuf

	if err := monitorCmd.Start(); err != nil {
		s.Fatal("Failed to run healthd monitor command: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		stdout := string(stdoutBuf.Bytes())
		if !strings.Contains(stdout, "Subscribe to bluetooth events successfully") {
			return errors.Errorf("failed to subscirbe Bluetooth event, stdout: %s", stdout)
		}
		return nil
	}, &testing.PollOptions{Interval: 200 * time.Millisecond, Timeout: 1 * time.Second}); err != nil {
		s.Fatal("Failed to subscirbe event in healthd: ", err)
	}

	// Trigger Bluetooth event.
	b, err := testexec.CommandContext(ctx, "bluetoothctl", "power", "on").Output(testexec.DumpLogOnError)
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
