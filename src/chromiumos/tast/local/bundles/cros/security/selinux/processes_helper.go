// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package selinux

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/shirou/gopsutil/process"
)

// Process represents a running process with a SELinux context.
type Process struct {
	Pid       int
	Cmdline   string
	Exe       string
	Comm      string
	SEContext string
}

func GetProcesses() ([]Process, error) {
	ps, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("failed to list processes: %v", err)
	}
	var processes []Process
	for _, p := range ps {
		proc := Process{Pid: int(p.Pid)}
		pidStr := strconv.Itoa(proc.Pid)

		if proc.Exe, err = p.Exe(); err != nil {
			// kernel process may have exe throwing no such file when readlink.
			// we don't want to skip kernel process.
			if !os.IsNotExist(err) {
				return nil, err
			}
		}

		// Read /proc/<pid>/{cmdline,comm,attr/current}
		// Ignore this process if it doesn't exist.
		if proc.Cmdline, err = p.Cmdline(); err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			} else {
				continue
			}
		}

		comm, err := ioutil.ReadFile("/proc/" + pidStr + "/comm")
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			} else {
				continue
			}
		}
		proc.Comm = string(comm)

		secontext, err := ioutil.ReadFile("/proc/" + pidStr + "/attr/current")
		if err != nil {
			return nil, err
		}
		if secontext[len(secontext)-1] == '\x00' {
			secontext = secontext[:len(secontext)-1]
		} else {
			return nil, fmt.Errorf("secontext terminates with non-NUL byte: %v, %q", proc, secontext)
		}
		proc.SEContext = string(secontext)

		processes = append(processes, proc)
	}
	return processes, nil
}

func SearchProcessByExe(ps []Process, exe string) []Process {
	var foundProcess []Process
	for _, proc := range ps {
		if proc.Exe == exe {
			foundProcess = append(foundProcess, proc)
		}
	}
	return foundProcess
}
