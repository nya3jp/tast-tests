// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Restart,
		Desc:     "Tests that we can shut down and restart crostini (where the VM image is a build artifact)",
		Contacts: []string{"hollingum@chromium.org", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:      "artifact",
			Pre:       crostini.StartedByArtifact(),
			ExtraData: []string{crostini.ImageArtifact},
			Timeout:   7 * time.Minute,
		}, {
			Name:    "download",
			Pre:     crostini.StartedByDownload(),
			Timeout: 10 * time.Minute,
		}, {
			Name:    "download_buster",
			Pre:     crostini.StartedByDownloadBuster(),
			Timeout: 10 * time.Minute,
		}},
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func Restart(ctx context.Context, s *testing.State) {
	cont := s.PreValue().(crostini.PreData).Container
	numRestarts := 2

	startupTime, err := startTime(ctx, cont)
	if err != nil {
		s.Fatal("Failed to get startup time: ", err)
	}

	for i := 0; i < numRestarts; i++ {
		s.Logf("Restart #%d, startup time was %v", i+1, startupTime)
		if err := cont.VM.Stop(ctx); err != nil {
			s.Fatal("Failed to close VM: ", err)
		}

		// While the VM is down, this command is expected to fail.
		if out, err := cont.Command(ctx, "pwd").Output(); err == nil {
			s.Fatalf("Expected command to fail while the container was shut down, but got: %q", string(out))
		} else {
			s.Log("Received an expected error running a container command: ", err)
		}

		// Start the VM and container.
		if err := cont.VM.Start(ctx); err != nil {
			s.Fatal("Failed to start VM: ", err)
		}
		if err := cont.StartAndWait(ctx, s.OutDir()); err != nil {
			s.Fatal("Failed to start container: ", err)
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
