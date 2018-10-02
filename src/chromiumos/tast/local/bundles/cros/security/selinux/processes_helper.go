// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package selinux

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"sync"
)

type Process struct {
	pid       int64
	cmdline   string
	exe       string
	comm      string
	secontext string
}

var processes []Process
var processesOnce sync.Once

func refreshProcesses() {
	processes = nil
	fis, err := ioutil.ReadDir("/proc")
	if err != nil {
		panic(fmt.Errorf("Failed to read /proc: %v", err))
	}
	for _, fi := range fis {
		name := fi.Name()
		if pid, err := strconv.ParseInt(name, 10, 64); err == nil {
			exe, err := os.Readlink("/proc/" + name + "/exe")
			if err != nil {
				// kernel process may have exe throwing no such file when readlink.
				// we don't want to skip kernel process.
				if !os.IsNotExist(err) {
					panic(err)
				}
			}

			// Read /proc/<pid>/{cmdline,comm,attr/current}
			// Ignore this process if it doesn't exist.
			cmdline, err := ioutil.ReadFile("/proc/" + name + "/cmdline")
			if err != nil {
				if !os.IsNotExist(err) {
					panic(err)
				} else {
					continue
				}
			}

			comm, err := ioutil.ReadFile("/proc/" + name + "/comm")
			if err != nil {
				if !os.IsNotExist(err) {
					panic(err)
				} else {
					continue
				}
			}

			secontext, err := ioutil.ReadFile("/proc/" + name + "/attr/current")
			if err != nil {
				if !os.IsNotExist(err) {
					panic(err)
				} else {
					continue
				}
			}
			processes = append(processes, Process{pid, string(cmdline[:]), exe, string(comm[:]), string(secontext[:])})
		}
	}
}

func GetSEContext(proc Process) string {
	return proc.secontext
}

func SearchProcessByExe(exe string) []Process {
	var foundProcess []Process
	processesOnce.Do(refreshProcesses)
	for _, proc := range processes {
		if proc.exe == exe {
			foundProcess = append(foundProcess, proc)
		}
	}
	return foundProcess
}
