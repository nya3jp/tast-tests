// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/local/faillog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"

	"github.com/shirou/gopsutil/process"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckProcesses,
		Desc: "Checks that all expected processes are running",
	})
}

func CheckProcesses(s *testing.State) {
	defer faillog.SaveIfError(s)

	// Some jobs are restarted (possibly indirectly) by other tests. If one of those tests runs
	// just before this one, it's possible that some processes won't be running yet, so wait a
	// bit for frequently-restarted jobs to start.
	waitJobs := []string{"debugd", "powerd"}
	jobCh := make(chan error)
	for _, job := range waitJobs {
		go func(job string) {
			err := testing.Poll(s.Context(), func(ctx context.Context) error {
				if running, _, err := upstart.JobStatus(ctx, job); err != nil {
					return err
				} else if !running {
					return errors.New("not running")
				}
				return nil
			}, &testing.PollOptions{Timeout: 5 * time.Second})

			if err == nil {
				jobCh <- nil
			} else {
				jobCh <- fmt.Errorf("%s: %v", job, err)
			}
		}(job)
	}
	for range waitJobs {
		if err := <-jobCh; err != nil {
			s.Error("Failed waiting for job ", err)
		}
	}

	// Separate process names with | to allow multiple choices.
	expected := []string{
		"conntrackd|netfilter-queue-helper",
		"dbus-daemon",
		"debugd",
		"metrics_daemon",
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
