// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MonitorThunderboltEvent,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Monitors the Thunderbolt event detected properly or not",
		Contacts: []string{"pathan.jilani@intel.com",
			"cros-tdm-tpe-eng@google.com",
			"intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "crosHealthdRunning",
	})
}

func MonitorThunderboltEvent(ctx context.Context, s *testing.State) {
	// Checking whether device supports thunderbolt or not.
	outFiles, err := testexec.CommandContext(ctx, "ls", "/sys/bus/thunderbolt/devices").Output()
	if err != nil {
		s.Log("Failed to execute ls /sys/bus/thunderbolt/devices/ command: ", err)
	}
	isThunderboltSupport := false
	requiredFiles := []string{"0-0", "1-0", "domain0", "domain1"}
	for _, file := range requiredFiles {
		if strings.Contains(string(outFiles), file) {
			isThunderboltSupport = true
		}
	}
	// Run monitor command in background.
	var stdoutBuf, stderrBuf bytes.Buffer
	monitorCmd := testexec.CommandContext(ctx, "cros-health-tool", "event", "--category=thunderbolt", "--length_seconds=10")
	monitorCmd.Stdout = &stdoutBuf
	monitorCmd.Stderr = &stderrBuf

	if err := monitorCmd.Start(); err != nil {
		s.Fatal("Failed to run healthd monitor command: ", err)
	}

	// Trigger Thunderbolt event.
	udevadmToggle := func(udevAction string) (string, error) {
		var stdout string
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := testexec.CommandContext(ctx, "udevadm", "trigger", "-s", "thunderbolt", "-c", udevAction).Run(); err != nil {
				return errors.Wrap(err, "failed to trigger thunderbolt add event")
			}
			stderr := string(stderrBuf.Bytes())
			if stderr != "" {
				return errors.New("failed to detect thunderbolt event, stderr")
			}

			stdout = string(stdoutBuf.Bytes())
			if !strings.Contains(stdout, udevAction) {
				return errors.New("failed to get command output")
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return "", errors.Wrapf(err, "failed to verify thunderbolt devices after %s", udevAction)
		}
		return stdout, nil
	}

	deviceAddedPattern := regexp.MustCompile("Device added")
	stdOut, err := udevadmToggle("add")
	if isThunderboltSupport {
		if !deviceAddedPattern.MatchString(stdOut) {
			s.Fatal("Failed to detect thunderbolt event, event output: ", err)
		}
	} else {
		if deviceAddedPattern.MatchString(stdOut) {
			s.Fatalf("Failed , thunderbolt event deteceted for non supported thunderbolt devices: %s ", stdOut)
		}
	}

	monitorCmd.Wait()
}
