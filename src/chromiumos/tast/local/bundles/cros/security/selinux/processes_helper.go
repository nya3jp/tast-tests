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

		// Ignore all errors returned by gopsutil while reading process data; these typically
		// indicate that this is a kernel process (in the case of exe) or that the process
		// disappeared mid-test. gopsutil doesn't make any promises that the error that it
		// returns in the process-disappeared case will be os.ErrNotExist: https://crbug.com/918499
		if proc.Exe, err = p.Exe(); err != nil {
			continue
		}
		if proc.Cmdline, err = p.Cmdline(); err != nil {
			continue
		}

		if comm, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/comm", proc.PID)); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		} else {
			proc.Comm = string(comm)
		}

		if secontext, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/attr/current", proc.PID)); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		} else if len(secontext) == 0 || secontext[len(secontext)-1] != 0 {
			return nil, errors.Errorf("invalid secontext %q", secontext)
		} else {
			proc.SEContext = string(secontext[:len(secontext)-1])
		}

		processes = append(processes, proc)
	}
	return processes, nil
}

// FindProcessesByExe returns processes from ps with Exe fields matching exeRegex.
func FindProcessesByExe(ps []Process, exeRegex string, revese bool) ([]Process, error) {
	var found []Process
	for _, proc := range ps {
		matched, err := regexp.MatchString("^"+exeRegex+"$", proc.Exe)
		if err != nil {
			return nil, err
		}
		if matched != revese {
			found = append(found, proc)
		}
	}
	return found, nil
}

// FindProcessesByCmdline returns processes from ps with Cmdline fields
// matching(reverse=false) or not matching(reverse=true) partial regular
// expression cmdlineRegex.
func FindProcessesByCmdline(ps []Process, cmdlineRegex string, reverse bool) ([]Process, error) {
	var found []Process
	for _, proc := range ps {
		matched, err := regexp.MatchString(cmdlineRegex, proc.Cmdline)
		if err != nil {
			return nil, err
		}
		if matched != reverse {
			found = append(found, proc)
		}
	}
	return found, nil
}

// ProcessContextRegexp returns a regexp from context, by wrapping it like "^u:r:xxx:.*$".
func ProcessContextRegexp(context string) (*regexp.Regexp, error) {
	return regexp.Compile("^u:r:" + context + ":.*$")
}
