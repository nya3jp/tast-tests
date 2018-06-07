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

var expectedProcessNames = []string{
	"debugd",
	"metrics_daemon",
	"powerd",
	"tlsdated",
}

func CheckProcesses(s *testing.State) {
	procs, err := process.Processes()
	if err != nil {
		s.Fatal("Failed to get a list of processes: ", err)
	}

	runnings := make(map[string]bool)
	for _, proc := range procs {
		if name, err := proc.Name(); err == nil {
			runnings[name] = true
		}
	}

	var missings []string
	for _, name := range expectedProcessNames {
		if !runnings[name] {
			missings = append(missings, name)
		} else {
			s.Log("Found: ", name)
		}
	}

	if len(missings) > 0 {
		s.Fatal("Process(es) not found: ", missings)
	}
}
