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
	pid       int
	cmdline   string
	exe       string
	comm      string
	secontext string
}

func GetProcesses() ([]Process, error) {
	var processes []Process
	ps, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("Failed to list processes: %v", err)
	}
	for _, p := range ps {
		pid := p.Pid
		exe, err := p.Exe()
		if err != nil {
			// kernel process may have exe throwing no such file when readlink.
			// we don't want to skip kernel process.
			if !os.IsNotExist(err) {
				return nil, err
			}
		}

		// Read /proc/<pid>/{cmdline,comm,attr/current}
		// Ignore this process if it doesn't exist.
		cmdline, err := p.Cmdline()
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			} else {
				continue
			}
		}

		comm, err := ioutil.ReadFile("/proc/" + strconv.Itoa(int(pid)) + "/comm")
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			} else {
				continue
			}
		}

		secontext, err := ioutil.ReadFile("/proc/" + strconv.Itoa(int(pid)) + "/attr/current")
		if err != nil {
			return nil, err
		}
		processes = append(processes, Process{int(pid), cmdline, exe, string(comm), string(secontext)})
	}
	return processes, nil
}

// GetSEContext returns from Process proc, with terminating NUL-byte stripped.
func GetSEContext(proc Process) (string, error) {
	if proc.secontext[len(proc.secontext)-1] == '\x00' {
		return proc.secontext[:len(proc.secontext)-1], nil
	}
	// Shouldn't happen.
	return "", fmt.Errorf("secontext terminates with non-NUL byte: %q", proc)
}

func SearchProcessByExe(ps []Process, exe string) []Process {
	var foundProcess []Process
	for _, proc := range ps {
		if proc.exe == exe {
			foundProcess = append(foundProcess, proc)
		}
	}
	return foundProcess
}
