// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"chromiumos/tast/testing"

	"github.com/shirou/gopsutil/process"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckProcesses,
		Desc: "Checks that all expected processes are running",
		Attr: []string{"bvt"},
	})
}

func CheckProcesses(s *testing.State) {
	expected := []string{
		"debugd",
		"metrics_daemon",
		"powerd",
		"tlsdated",
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

	for _, name := range expected {
		if _, ok := running[name]; !ok {
			s.Errorf("%v not running", name)
		} else {
			s.Logf("%v is running", name)
		}
	}
}
