// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"fmt"

	"github.com/shirou/gopsutil/process"
)

const (
	chromeExecPath = "/opt/google/chrome/chrome" // path of Chrome executable
)

// GetPIDs returns all PIDs corresponding to Chrome processes.
func GetPIDs() ([]int, error) {
	all, err := process.Pids()
	if err != nil {
		return nil, err
	}

	pids := make([]int, 0)
	for _, pid := range all {
		if proc, err := process.NewProcess(pid); err != nil {
			// Assume that the process exited.
			continue
		} else if exe, err := proc.Exe(); err == nil && exe == chromeExecPath {
			pids = append(pids, int(pid))
		}
	}
	return pids, nil
}

// GetRootPID returns the PID of the root Chrome process.
// This corresponds to the browser process.
func GetRootPID() (int, error) {
	pids, err := GetPIDs()
	if err != nil {
		return -1, err
	}

	pm := make(map[int]struct{})
	for _, pid := range pids {
		pm[pid] = struct{}{}
	}
	for _, pid := range pids {
		// If we see errors, assume that the process exited.
		proc, err := process.NewProcess(int32(pid))
		if err != nil {
			continue
		}
		ppid, err := proc.Ppid()
		if err != nil || ppid <= 0 {
			continue
		}
		if _, ok := pm[int(ppid)]; !ok {
			return pid, nil
		}
	}
	return -1, fmt.Errorf("root not found")
}
