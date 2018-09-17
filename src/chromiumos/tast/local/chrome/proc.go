// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"fmt"
	"strings"

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

// getProcesses returns Chrome processes with the --type=${t} flag.
func getProcesses(t string) ([]process.Process, error) {
	ps, err := process.Processes()
	if err != nil {
		return nil, err
	}

	// Wrap by whitespaces. Please see the comment below.
	flg := " --type=" + t + " "
	var ret []process.Process
	for _, proc := range ps {
		if exe, err := proc.Exe(); err != nil || exe != chromeExecPath {
			continue
		}

		// Process.CmdlineSliceWithContext() is more appropriate, but
		// 1) Chrome's /proc/*/cmdline is whitespace separated, so
		//    proc.CmdlineSlice/CmdlineSliceWithContext won't work.
		//    cf) https://bugs.gentoo.org/477538
		// 2) Our gopsutil is too old so that CmdlineSliceWithContext
		//    is not supported.
		// Thus, instead Cmdline() is used here. Please also find
		// whitespaces in |flg|.
		// cf) crbug.com/887875
		cmd, err := proc.Cmdline()
		if err != nil {
			continue
		}
		if strings.Contains(cmd, flg) {
			ret = append(ret, *proc)
		}
	}
	return ret, nil
}

// GetPluginProcesses returns Chrome plugin processes.
func GetPluginProcesses() ([]process.Process, error) {
	return getProcesses("plugin")
}

// GetRendererProcesses returns Chrome renderer processes.
func GetRendererProcesses() ([]process.Process, error) {
	return getProcesses("renderer")
}
