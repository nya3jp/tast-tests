// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
		Func:         MonitorUsbEvent,
		Desc:         "Monitors the USB event detected properly or not",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "crosHealthdRunning",
	})
}

func MonitorUsbEvent(ctx context.Context, s *testing.State) {
	// Run monitor command in background.
	var stdoutBuf, stderrBuf bytes.Buffer
	monitorCmd := testexec.CommandContext(ctx, "cros-health-tool", "event", "--category=usb", "--length_seconds=10")
	monitorCmd.Stdout = &stdoutBuf
	monitorCmd.Stderr = &stderrBuf

	if err := monitorCmd.Start(); err != nil {
		s.Fatal("Failed to run healthd monitor command: ", err)
	}

	// Trigger USB event.
	if err := testexec.CommandContext(ctx, "udevadm", "trigger", "-s", "usb", "-c", "add").Run(); err != nil {
		s.Fatal("Failed to trigger usb add event: ", err)
	}

	monitorCmd.Wait()

	stderr := string(stderrBuf.Bytes())
	if stderr != "" {
		s.Fatal("Failed to detect USB event, stderr: ", stderr)
	}

	stdout := string(stdoutBuf.Bytes())
	deviceAddedPattern := regexp.MustCompile(`"event": "Add"`)
	if !deviceAddedPattern.MatchString(stdout) {
		s.Fatal("Failed to detect USB event, event output: ", stdout)
	}
}
