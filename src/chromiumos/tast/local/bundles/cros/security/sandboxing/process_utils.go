// Copyright 2021 The ChromiumOS Authors
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

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
)

// Exclusions contains names (from the "Name:" field in /proc/<pid>/status) of processes to ignore
// in sandboxing-related test. These processes are either transient, not present on production images,
// or not sandboxing-relevant.
var Exclusions = []string{
	"agetty",
	"aplay", // sometimes left behind by Autotest audio tests
	"autotest",
	"autotestd",
	"autotestd_monitor",
	"check_ethernet.hook",
	"chrome",
	"chrome-sandbox",
	"cras_test_client",
	"crash_reporter",
	"endpoint",
	"evemu-device",
	"flock",
	"grep",
	"init",
	"logger",
	"login",
	"mosys", // used to get system info: https://crbug.com/963888
	"nacl_helper",
	"nacl_helper_bootstrap",
	"nacl_helper_nonsfi",
	"ping",
	"ply-image",
	"ps",
	"recover_duts",
	"sleep",
	"sshd",
	"sudo",
	"tail",
	"timeout",
	"x11vnc",
	"bash", // TODO: check against script name instead
	"dash",
	"python",
	"python2",
	"python3",
	"python3.4",
	"python3.5",
	"python3.6",
	"python3.7",
	"run_oci", // used to run other processes
	"sh",
	"minijail0", // just launches other daemons; also runs as root to drop privs
	"minijail-init",
	"(agetty)", // initial name when systemd starts serial-getty; changes to "agetty" later
	"adb",      // sometimes appears on test images: https://crbug.com/792541
	"postinst", // runs cros_installer
}

// IgnoredAncestors contains names of processes whose children we should ignore in sandboxing-related tests.
// These processes are either not relevant (like kernel processes), transient, or test-related.
var IgnoredAncestors = []string{
	"kthreadd",           // kernel processes
	"local_test_runner",  // Tast-related processes
	"periodic_scheduler", // runs cron scripts
	"arc-setup",          // runs patchoat and other Android programs
	"cros_installer",     // runs during system updates
	"python2.7",          // stale Autotest processes: https://crbug.com/936703#c39
	"dev_debug_vboot",    // executed by chromeos-setgoodkernel: https://crbug.com/962134
}

// IgnoredMoblabAncestors contains names of processes whose children we should ignore
// in sandboxing-related tests. They are used to implement the Moblab test harness.
var IgnoredMoblabAncestors = []string{
	"apache2",         // serves UI and runs other procs: https://crbug.com/962137
	"dockerd",         // Used to run envoy proxy required for grpc-web based UI
	"containerd-shim", // Used to run envoy proxy required for grpc-web based UI
	"containerd",      // Used to run envoy proxy required for grpc-web based UI
}

// TruncateProcName returns a shortened version of the process' name, matching what the kernel does.
//
// Per TASK_COMM_LEN, the kernel only uses 16 null-terminated bytes to hold process names
// (which we later read from /proc/<pid>/status), so we shorten names in all sandboxing-related tests.
// See https://stackoverflow.com/questions/23534263 for more discussion.
//
// Using "Name:" from /status matches what the Autotest code was doing, but it can lead to unexpected collisions.
// /exe is undesirable since executables like /usr/bin/coreutils implement many commands.
// /cmdline may be modified by the process.
func TruncateProcName(s string) string {
	const maxProcNameLen = 15

	if len(s) <= maxProcNameLen {
		return s
	}
	return s[:maxProcNameLen]
}

// ProcSandboxInfo holds sandboxing-related information about a running process.
type ProcSandboxInfo struct {
	Name               string          // "Name:" value from /proc/<pid>/status
	Exe                string          // full executable path
	Cmdline            string          // space-separated command line
	Ppid               int32           // parent PID
	Euid, Egid         uint32          // effective UID and GID
	PidNS, MntNS       int64           // PID and mount namespace IDs (-1 if unknown)
	Ecaps              uint64          // effective capabilities
	NoNewPrivs         bool            // no_new_privs is set (see "minijail -N")
	Seccomp            bool            // seccomp filter is active
	HasTestImageMounts bool            // has test-image-only mounts
	MountInfos         []ProcMountinfo // entries from /proc/<pid>/mountinfo
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
	if mnts, err := ReadProcMountpoints(proc.Pid); os.IsNotExist(err) || err == unix.EINVAL {
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

	if mountInfos, err := ReadProcMountinfo(proc.Pid); os.IsNotExist(err) || err == unix.EINVAL {
		// mountinfo files are sometimes missing or unreadable.
	} else if err != nil {
		saveErr(errors.Wrap(err, "failed reading mountinfo"))
	} else {
		info.MountInfos = mountInfos
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

// ProcHasAncestor returns true if pid has any of ancestorPIDs as an ancestor process.
// infos should contain the full set of processes and is used to look up data.
func ProcHasAncestor(pid int32, ancestorPIDs map[int32]struct{},
	infos map[int32]*ProcSandboxInfo) (bool, error) {
	info, ok := infos[pid]
	if !ok {
		return false, errors.Errorf("process %d not found", pid)
	}

	for {
		pinfo, ok := infos[info.Ppid]
		if !ok {
			return false, errors.Errorf("parent process %d not found", info.Ppid)
		}
		if _, ok := ancestorPIDs[info.Ppid]; ok {
			return true, nil
		}
		if info.Ppid == 1 {
			return false, nil
		}
		info = pinfo
	}
}

// ProcMountinfo holds information about /proc/<pid>/mountinfo entries.
type ProcMountinfo struct {
	MountID           uint32
	ParentID          uint32
	Major             uint32
	Minor             uint32
	Root              string
	MountPoint        string
	MountOptions      string
	OptFields         []string
	FsType            string
	MountSource       string
	SuperBlockOptions string
}

// ReadProcMountinfo returns all mountpoints listed in /proc/<pid>/mountinfo.
// This may return os.ErrNotExist or syscall.EINVAL for zombie processes: https://crbug.com/936703
//
// Example line:
// 347 254 8:1 /home /home rw,nosuid,nodev,noexec,noatime shared:96 - ext4 /dev/sda1 rw,seclabel,resgid=20119,commit=600,data=ordered
//
// (1) mount ID:  unique identifier of the mount (may be reused after umount)
// (2) parent ID:  ID of parent (or of self for the top of the mount tree)
// (3) major:minor:  value of st_dev for files on filesystem
// (4) root:  root of the mount within the filesystem
// (5) mount point:  mount point relative to the process's root
// (6) mount options:  per mount options
// (7) optional fields:  zero or more fields of the form "tag[:value]"
// (8) separator:  marks the end of the optional fields
// (9) filesystem type:  name of filesystem of the form "type[.subtype]"
// (10) mount source:  filesystem specific information or "none"
// (11) super options:  per super block options
//
// Parsers should ignore all unrecognised optional fields.  Currently the
// possible optional fields are:
// shared:X  mount is shared in peer group X
// master:X  mount is slave to peer group X
// propagate_from:X  mount is slave and receives propagation from peer group X (*)
// unbindable  mount is unbindable
func ReadProcMountinfo(pid int32) ([]ProcMountinfo, error) {
	const firstOptField = 6

	b, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/mountinfo", pid))
	// ioutil.ReadFile can return an *os.PathError. If it's os.ErrNotExist, we return it directly
	// since it's easy to check, but for other errors, we return the inner error (which is a syscall.Errno)
	// so that callers can inspect it.
	if pathErr, ok := err.(*os.PathError); ok && !os.IsNotExist(err) {
		return nil, pathErr.Err
	} else if err != nil {
		return nil, err
	}
	var mounts []ProcMountinfo
	for _, ln := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if ln == "" {
			continue
		}
		// Example line:
		// 347 254 8:1 /home /home rw,nosuid,nodev,noexec,noatime shared:96 - ext4 /dev/sda1 rw,seclabel,resgid=20119,commit=600,data=ordered
		var mi ProcMountinfo
		fields := strings.Fields(ln)

		var v uint64
		v, err = strconv.ParseUint(fields[0], 10, 32)
		if err != nil {
			return nil, err
		}
		mi.MountID = uint32(v)
		v, err = strconv.ParseUint(fields[1], 10, 32)
		if err != nil {
			return nil, err
		}
		mi.ParentID = uint32(v)
		majorMinor := strings.Split(fields[2], ":")
		v, err = strconv.ParseUint(majorMinor[0], 10, 32)
		if err != nil {
			return nil, err
		}
		mi.Major = uint32(v)
		v, err = strconv.ParseUint(majorMinor[1], 10, 32)
		if err != nil {
			return nil, err
		}
		mi.Minor = uint32(v)
		mi.Root = fields[3]
		mi.MountPoint = fields[4]
		mi.MountOptions = fields[5]

		var optFields []string
		for _, field := range fields[firstOptField:] {
			if field == "-" {
				// This is the separator, there are no more optional fields.
				mi.OptFields = optFields
				break
			} else {
				optFields = append(optFields, field)
			}
		}

		nextField := firstOptField + len(optFields) + 1 // + 1 for the separator.
		mi.FsType = fields[nextField]
		mi.MountSource = fields[nextField+1]
		mi.SuperBlockOptions = fields[nextField+2]
		mounts = append(mounts, mi)
	}
	return mounts, nil
}
