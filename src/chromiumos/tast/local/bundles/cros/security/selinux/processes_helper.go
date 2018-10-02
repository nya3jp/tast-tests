// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package selinux

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/shirou/gopsutil/process"
)

// Process represents a running process with a SELinux context.
type Process struct {
	PID       int
	Cmdline   string
	Exe       string
	Comm      string
	SEContext string
}

// Returns a list of current running Processes. If an error occurred, returns
// an error.
func GetProcesses() ([]Process, error) {
	ps, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("failed to list processes: %v", err)
	}
	var processes []Process
	for _, p := range ps {
		proc := Process{PID: int(p.Pid)}

		if proc.Exe, err = p.Exe(); err != nil && !os.IsNotExist(err) {
			// kernel process may have exe throwing no such file when readlink.
			// we don't want to skip kernel process.
			return nil, err
		}

		// Read /proc/<pid>/{cmdline,comm,attr/current}
		// Ignore this process if it doesn't exist.
		if proc.Cmdline, err = p.Cmdline(); err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}
			continue
		}

		comm, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/comm", proc.PID))
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}
			continue
		}
		proc.Comm = string(comm)

		secontext, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/attr/current", proc.PID))
		if err != nil {
			return nil, err
		}
		if len(secontext) <= 0 || secontext[len(secontext)-1] != 0 {
			return nil, fmt.Errorf("invalid secontext found %q", secontext)
		}
		proc.SEContext = string(secontext[:len(secontext)-1])

		processes = append(processes, proc)
	}
	return processes, nil
}

// Search processed from ps, and return list of process if exe of the process
// matches given exe in the parameter.
func SearchProcessByExe(ps []Process, exe string) []Process {
	var foundProcess []Process
	for _, proc := range ps {
		if proc.Exe == exe {
			foundProcess = append(foundProcess, proc)
		}
	}
	return foundProcess
}
