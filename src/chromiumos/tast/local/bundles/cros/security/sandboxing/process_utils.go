// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package sandboxing provides functions for obtaining sandboxing-related information about running processes.
package sandboxing

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
)

// ProcSandboxInfo holds sandboxing-related information about a running process.
type ProcSandboxInfo struct {
	Name               string // "Name:" value from /proc/<pid>/status
	Exe                string // full executable path
	Cmdline            string // space-separated command line
	Ppid               int32  // parent PID
	Euid, Egid         uint32 // effective UID and GID
	PidNS, MntNS       int64  // PID and mount namespace IDs (-1 if unknown)
	Ecaps              uint64 // effective capabilities
	NoNewPrivs         bool   // no_new_privs is set (see "minijail -N")
	Seccomp            bool   // seccomp filter is active
	HasTestImageMounts bool   // has test-image-only mounts
}

// GetProcSandboxInfo returns sandboxing-related information about proc.
// An error is returned if any files cannot be read or if malformed data is encountered,
// but the partially-filled info is still returned.
func GetProcSandboxInfo(proc *process.Process) (*ProcSandboxInfo, error) {
	var info ProcSandboxInfo
	var firstErr error
	saveErr := func(err error) {
		if firstErr == nil {
			firstErr = err
		}
	}

	// Ignore errors for e.g. kernel processes.
	info.Exe, _ = proc.Exe()
	info.Cmdline, _ = proc.Cmdline()

	var err error
	if info.Ppid, err = proc.Ppid(); err != nil {
		saveErr(errors.Wrap(err, "failed to get parent"))
	}

	if uids, err := proc.Uids(); err != nil {
		saveErr(errors.Wrap(err, "failed to get UIDs"))
	} else {
		info.Euid = uint32(uids[1])
	}

	if gids, err := proc.Gids(); err != nil {
		saveErr(errors.Wrap(err, "failed to get GIDs"))
	} else {
		info.Egid = uint32(gids[1])
	}

	// Namespace data appears to sometimes be missing for (exiting?) processes: https://crbug.com/936703
	if info.PidNS, err = ReadProcNamespace(proc.Pid, "pid"); os.IsNotExist(err) && proc.Pid != 1 {
		info.PidNS = -1
	} else if err != nil {
		saveErr(errors.Wrap(err, "failed to read pid namespace"))
	}
	if info.MntNS, err = ReadProcNamespace(proc.Pid, "mnt"); os.IsNotExist(err) && proc.Pid != 1 {
		info.MntNS = -1
	} else if err != nil {
		saveErr(errors.Wrap(err, "failed to read mnt namespace"))
	}

	// Read additional info from /proc/<pid>/status.
	status, err := ReadProcStatus(proc.Pid)
	if err != nil {
		saveErr(errors.Wrap(err, "failed reading status"))
	} else {
		if info.Ecaps, err = strconv.ParseUint(status["CapEff"], 16, 64); err != nil {
			saveErr(errors.Wrapf(err, "failed parsing effective caps %q", status["CapEff"]))
		}
		info.Name = status["Name"]
		info.NoNewPrivs = status["NoNewPrivs"] == "1"
		info.Seccomp = status["Seccomp"] == "2" // 1 is strict, 2 is filter
	}

	// Check whether any mounts that only occur in test images are available to the process.
	// These are limited to the init mount namespace, so if a process has its own namespace,
	// it shouldn't have these (assuming that it called pivot_root()).
	if mnts, err := ReadProcMountpoints(proc.Pid); os.IsNotExist(err) || err == syscall.EINVAL {
		// mounts files are sometimes missing or unreadable: https://crbug.com/936703#c14
	} else if err != nil {
		saveErr(errors.Wrap(err, "failed reading mountpoints"))
	} else {
		for _, mnt := range mnts {
			for _, tm := range []string{"/usr/local", "/var/db/pkg", "/var/lib/portage"} {
				if mnt == tm {
					info.HasTestImageMounts = true
					break
				}
			}
		}
	}

	return &info, firstErr
}

// ReadProcMountpoints returns all mountpoints listed in /proc/<pid>/mounts.
// This may return os.ErrNotExist or syscall.EINVAL for zombie processes: https://crbug.com/936703
func ReadProcMountpoints(pid int32) ([]string, error) {
	b, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/mounts", pid))
	// ioutil.ReadFile can return an *os.PathError. If it's os.ErrNotExist, we return it directly
	// since it's easy to check, but for other errors, we return the inner error (which is a syscall.Errno)
	// so that callers can inspect it.
	if pathErr, ok := err.(*os.PathError); ok && !os.IsNotExist(err) {
		return nil, pathErr.Err
	} else if err != nil {
		return nil, err
	}
	var mounts []string
	for _, ln := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if ln == "" {
			continue
		}
		// Example line:
		// run /var/run tmpfs rw,seclabel,nosuid,nodev,noExec,relatime,mode=755 0 0
		parts := strings.Fields(ln)
		if len(parts) != 6 {
			return nil, errors.Errorf("failed to parse line %q", ln)
		}
		mounts = append(mounts, parts[1])
	}
	return mounts, nil
}

// ReadProcNamespace returns pid's namespace ID for name (e.g. "pid" or "mnt"),
// per /proc/<pid>/ns/<name>. This may return os.ErrNotExist: https://crbug.com/936703
func ReadProcNamespace(pid int32, name string) (int64, error) {
	v, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/%s", pid, name))
	if err != nil {
		return -1, err
	}
	// The link value should have the form ":[<id>]"
	pre := name + ":["
	suf := "]"
	if !strings.HasPrefix(v, pre) || !strings.HasSuffix(v, suf) {
		return -1, errors.Errorf("unexpected value %q", v)
	}
	return strconv.ParseInt(v[len(pre):len(v)-len(suf)], 10, 64)
}

// procStatusLineRegexp is used to split a line from /proc/<pid>/status. Example content:
// Name:	powerd
// State:	S (sleeping)
// Tgid:	1249
// ...
var procStatusLineRegexp = regexp.MustCompile(`^([^:]+):\t(.*)$`)

// ReadProcStatus parses /proc/<pid>/status and returns its key/value pairs.
func ReadProcStatus(pid int32) (map[string]string, error) {
	b, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return nil, err
	}

	vals := make(map[string]string)
	for _, ln := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		// Skip blank lines: https://bugs.launchpad.net/ubuntu/+source/linux/+bug/1772671
		if ln == "" {
			continue
		}
		ms := procStatusLineRegexp.FindStringSubmatch(ln)
		if ms == nil {
			return nil, errors.Errorf("failed to parse line %q", ln)
		}
		vals[ms[1]] = ms[2]
	}
	return vals, nil
}
