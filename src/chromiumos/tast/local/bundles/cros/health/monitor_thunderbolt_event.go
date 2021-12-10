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
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"

	"github.com/pkg/errors"
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
		// TODO(b/207569436): Define hardware dependency and get rid of hard-coding the models.
		HardwareDeps: hwdep.D(hwdep.Model("brya", "redrix", "kano", "anahera", "primus", "crota")),
		Fixture:      "crosHealthdRunning",
	})
}

func MonitorThunderboltEvent(ctx context.Context, s *testing.State) {
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
				return errors.Wrap(err, "Failed to trigger thunderbolt add event")
			}
			stderr := string(stderrBuf.Bytes())
			if stderr != "" {
				return errors.New("failed to detect thunderbolt event, stderr")
			}

			stdout = string(stdoutBuf.Bytes())
			if !strings.Contains(stdout, udevAction) {
				return errors.New("Failed to get command output")
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return "", errors.Wrapf(err, "failed to verify thunderbolt devices after %s", udevAction)
		}
		return stdout, nil
	}

	stdOut, err := udevadmToggle("add")
	if err != nil {
		s.Fatal("Failed to add device: ", err)
	}

	deviceAddedPattern := regexp.MustCompile("Device added")
	if !deviceAddedPattern.MatchString(stdOut) {
		s.Fatal("Failed to detect thunderbolt event, event output: ")
	}

	monitorCmd.Wait()
}
