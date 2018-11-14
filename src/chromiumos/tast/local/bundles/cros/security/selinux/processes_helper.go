// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package selinux

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
)

// Process represents a running process with an SELinux context.
type Process struct {
	PID       int
	Cmdline   string
	Exe       string
	Comm      string
	SEContext string
}

// String returns a human-readable string representation for struct Process.
func (p Process) String() string {
	// Cmdline is usually enough for most cases for human inspection.
	return fmt.Sprintf("[%d %s %q]", p.PID, p.SEContext, p.Cmdline)
}

// GetProcesses returns currently-running processes.
func GetProcesses() ([]Process, error) {
	ps, err := process.Processes()
	if err != nil {
		return nil, errors.Wrap(err, "failed to list processes")
	}
	var processes []Process
	for _, p := range ps {
		proc := Process{PID: int(p.Pid)}

		if proc.Exe, err = p.Exe(); err != nil && !os.IsNotExist(err) {
			// Kernel process may have exe throwing no such file when readlink.
			// We don't want to skip kernel process.
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
			if !os.IsNotExist(err) {
				return nil, err
			}
			continue
		}
		if len(secontext) == 0 || secontext[len(secontext)-1] != 0 {
			return nil, errors.Errorf("invalid secontext %q", secontext)
		}
		proc.SEContext = string(secontext[:len(secontext)-1])

		processes = append(processes, proc)
	}
	return processes, nil
}

// FindProcessesByExe returns processes from ps with Exe fields matching exe.
func FindProcessesByExe(ps []Process, exe string) []Process {
	var found []Process
	for _, proc := range ps {
		if proc.Exe == exe {
			found = append(found, proc)
		}
	}
	return found
}

// FindProcessesByCmdline returns processes from ps with Cmdline fields matching
// partial regular expression cmdlineRegex.
func FindProcessesByCmdline(ps []Process, cmdlineRegex string) ([]Process, error) {
	var found []Process
	for _, proc := range ps {
		matched, err := regexp.MatchString(cmdlineRegex, proc.Cmdline)
		if err != nil {
			return nil, err
		}
		if matched {
			found = append(found, proc)
		}
	}
	return found, nil
}
