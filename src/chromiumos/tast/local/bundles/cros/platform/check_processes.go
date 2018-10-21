// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strings"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckProcesses,
		Desc: "Checks that all expected processes are running",
	})
}

func CheckProcesses(ctx context.Context, s *testing.State) {
	// Some jobs are restarted (possibly indirectly) by other tests or take time to start after booting.
	// If one of those tests runs just before this one or the system just booted, it's possible that some
	// processes won't be running yet, so wait a bit for their Upstart jobs.
	waitJobs := []string{"debugd", "powerd", "system-services"}
	jobCh := make(chan error)
	for _, job := range waitJobs {
		go func(job string) {
			if err := upstart.WaitForJobStatus(ctx, job, upstart.StartGoal, upstart.RunningState,
				upstart.TolerateWrongGoal, 10*time.Second); err == nil {
				jobCh <- nil
			} else {
				jobCh <- errors.Wrap(err, job)
			}
		}(job)
	}
	for range waitJobs {
		if err := <-jobCh; err != nil {
			s.Error("Failed waiting for job ", err)
		}
	}

	// Separate process names with | to allow multiple choices.
	// TODO(derat): Consider re-adding metrics_daemon if/when it starts sooner after boot: https://crbug.com/897521
	expected := []string{
		"anomaly_collector",
		"conntrackd|netfilter-queue-helper",
		"dbus-daemon",
		"debugd",
		"powerd",
		"shill",
		"systemd-udevd|udevd",
		"tlsdated",
		"update_engine",
		"wpa_supplicant",
	}

	procs, err := process.Processes()
	if err != nil {
		s.Fatal("Failed to get a list of processes: ", err)
	}

	running := make(map[string]struct{})
	for _, proc := range procs {
		if name, err := proc.Name(); err == nil {
			running[name] = struct{}{}
		}
	}

	for _, names := range expected {
		ok := false
		for _, name := range strings.Split(names, "|") {
			if _, ok = running[name]; ok {
				s.Logf("%v is running", name)
				break
			}
		}
		if !ok {
			s.Errorf("%v not running", names)
		}
	}
}
