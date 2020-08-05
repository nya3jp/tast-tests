// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Restart,
		Desc:     "Tests that we can shut down and restart crostini (where the VM image is a build artifact)",
		Contacts: []string{"hollingum@chromium.org", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline"},
		Params: []testing.Param{{
			Name:              "artifact",
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniStable,
		}, {
			Name:              "artifact_unstable",
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniUnstable,
			ExtraAttr:         []string{"informational"},
		}, {
			Name:      "download_stretch",
			Pre:       crostini.StartedByDownloadStretch(),
			Timeout:   10 * time.Minute,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "download_buster",
			Pre:       crostini.StartedByDownloadBuster(),
			Timeout:   10 * time.Minute,
			ExtraAttr: []string{"informational"},
		}},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Vars:         []string{"crostini.Restart.numRestarts"},
	})
}

// varInt returns the value for the named variable, or defaultVal if it is
// not supplied or unparseable.
func varInt(s *testing.State, name string, defaultVal int) int {
	if str, ok := s.Var(name); ok {
		val, err := strconv.Atoi(str)
		if err == nil {
			return val
		}
		s.Logf("Cannot parse argument %s %s: %v", name, str, err)
	}
	return defaultVal
}

func Restart(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	cont := pre.Container
	tconn := pre.TestAPIConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	numRestarts := varInt(s, "crostini.Restart.numRestarts", 2)

	startupTime, err := startTime(ctx, cont)
	if err != nil {
		s.Fatal("Failed to get startup time: ", err)
	}

	terminal, err := ui.LaunchTerminal(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to lauch terminal: ", err)
	}

	for i := 0; i < numRestarts; i++ {
		s.Logf("Restart #%d, startup time was %v", i+1, startupTime)
		if err := terminal.ShutdownCrostini(ctx); err != nil {
			s.Fatal("Failed to shutdown crostini: ", err)
		}

		err := testing.Poll(ctx, func(ctx context.Context) error {
			// While the VM is down, this command is expected to fail.
			if out, err := cont.Command(ctx, "pwd").Output(); err == nil {
				return errors.Errorf("expected command to fail while the container was shut down, but got: %q", string(out))
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second})
		if err != nil {
			s.Fatal("VM failed to stop: ", err)
		}

		// Start the VM and container.
		terminal, err = ui.LaunchTerminal(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to lauch terminal: ", err)
		}

		if err := pre.Connect(ctx); err != nil {
			s.Fatal("Failed to connect to restarted container: ", err)
		}

		// Compare start times.
		newStartupTime, err := startTime(ctx, cont)
		if err != nil {
			s.Fatal("Failed to get new startup time: ", err)
		}
		if !newStartupTime.After(startupTime) {
			s.Errorf("Restarted container didnt have a later startup time, %v vs %v", startupTime, newStartupTime)
		}
		startupTime = newStartupTime
	}
}

func startTime(ctx context.Context, cont *vm.Container) (time.Time, error) {
	out, err := cont.Command(ctx, "uptime", "--since").Output(testexec.DumpLogOnError)
	if err != nil {
		return time.Time{}, errors.Wrap(err, "failed to run uptime cmd")
	}
	t, err := time.Parse("2006-01-02 15:04:05\n", string(out))
	if err != nil {
		return time.Time{}, errors.Wrap(err, "failed to parse uptime")
	}
	return t, nil
}
