// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package selinux

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"syscall"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Common Filters for file label check
type SELinuxFileLabelCheckFilter func(*testing.State, string, os.FileInfo) bool

func IgnorePath(pathToIgnore string) SELinuxFileLabelCheckFilter {
	return func(_ *testing.State, p string, _ os.FileInfo) bool {
		return p == pathToIgnore
	}
}

func SkipNonExist(_ *testing.State, p string, f os.FileInfo) bool {
	if f != nil {
		return false
	}
	_, err := os.Lstat(p)
	if os.IsNotExist(err) {
		return true
	}
	return false
}

func FilterAny(filters ...SELinuxFileLabelCheckFilter) SELinuxFileLabelCheckFilter {
	return func(s *testing.State, p string, f os.FileInfo) bool {
		for _, filter := range filters {
			if filter(s, p, f) {
				return true
			}
		}
		return false
	}
}

func FilterReverse(filter SELinuxFileLabelCheckFilter) SELinuxFileLabelCheckFilter {
	return func(s *testing.State, p string, f os.FileInfo) bool {
		return !filter(s, p, f)
	}
}

type SELinuxFileLabelCheckArgs struct {
	path      string
	context   string
	recursive bool
	filter    SELinuxFileLabelCheckFilter
}

// SELinux file label check args
var SELinuxFileLabelTestArgs = []SELinuxFileLabelCheckArgs{
	SELinuxFileLabelCheckArgs{path: "/sbin/init", context: "u:object_r:chromeos_init_exec:s0"},
	SELinuxFileLabelCheckArgs{path: "/run/cras", context: "u:object_r:cras_socket:s0", recursive: true},
	SELinuxFileLabelCheckArgs{path: "/sys/fs/cgroup", context: "u:object_r:tmpfs:s0"},
	SELinuxFileLabelCheckArgs{path: "/sys/fs/cgroup", context: "u:object_r:cgroup:s0", recursive: true, filter: IgnorePath("/sys/fs/cgroup")},
	SELinuxFileLabelCheckArgs{path: "/sys/fs/pstore", context: "u:object_r:pstorefs:s0"},
	SELinuxFileLabelCheckArgs{path: "/sys/fs/selinux", context: "u:object_r:selinuxfs:s0", recursive: true, filter: IgnorePath("/sys/fs/selinux/null")},
	SELinuxFileLabelCheckArgs{path: "/sys/fs/selinux", context: "u:object_r:null_device:s0", recursive: true, filter: FilterReverse(IgnorePath("/sys/fs/selinux/null"))},
	SELinuxFileLabelCheckArgs{path: "/run/chrome/wayland-0", context: "u:object_r:wayland_socket:s0"},
	SELinuxFileLabelCheckArgs{path: "/sys/kernel/config", context: "u:object_r:configfs:s0"},
	SELinuxFileLabelCheckArgs{path: "/sys/kernel/debug", context: "u:object_r:debugfs:s0"},
	SELinuxFileLabelCheckArgs{path: "/sys/kernel/debug/tracing", context: "u:object_r:debugfs_tracing:s0"},
	SELinuxFileLabelCheckArgs{path: "/sys/kernel/debug/debugfs_tracing_on", context: "u:object_r:debugfs_tracing:s0", filter: SkipNonExist},
	SELinuxFileLabelCheckArgs{path: "/sys/kernel/debug/trace_marker", context: "u:object_r:debugfs_trace_marker:s0", filter: SkipNonExist},
	SELinuxFileLabelCheckArgs{path: "/sys/devices/system/cpu", context: "u:object_r:sysfs_devices_system_cpu:s0", recursive: true, filter: func(_ *testing.State, _ string, f os.FileInfo) bool {
		mode := f.Mode()
		return mode.IsRegular() && ((mode.Perm() & (syscall.S_IWUSR | syscall.S_IWGRP | syscall.S_IWOTH)) > 0)
	}},
}

func SELinuxFileLabel(s *testing.State) {
	getFileLabel := func(path string) (string, error) {
		b, err := testexec.CommandContext(s.Context(), "getfilecon", path).CombinedOutput()
		if err != nil {
			return "", err
		} else {
			bArray := strings.Split(strings.Trim(string(b), "\n"), "\t")
			if len(bArray) == 2 {
				return strings.Split(strings.Trim(string(b), "\n"), "\t")[1], nil
			}
			return "", fmt.Errorf("Unexpected getfilecon result %q", b)
		}
	}

	assertSELinuxFileContext := func(path string, expected string) {
		actual, err := getFileLabel(path)
		if err != nil {
			s.Errorf("Fail to get file context for %s: %s", path, err)
			return
		}
		if actual != expected {
			s.Errorf(
				"File context mismatch for file %s, expect %q, actual %q",
				path,
				expected,
				actual)
		}
	}

	var checkSELinuxFileContextRecursively func(filePath string, expect string, filter SELinuxFileLabelCheckFilter)

	checkSELinuxFileContextRecursively = func(filePath string, expect string, filter SELinuxFileLabelCheckFilter) {
		files, err := ioutil.ReadDir(filePath)
		if err != nil {
			s.Errorf("Fail to list directory %s: %s", filePath, err)
			return
		}
		for _, file := range files {
			subFilePath := path.Join(filePath, file.Name())
			if filter == nil || !filter(s, subFilePath, file) {
				assertSELinuxFileContext(subFilePath, expect)
			}
			if file.IsDir() {
				checkSELinuxFileContextRecursively(subFilePath, expect, filter)
			}
		}
	}

	for _, testArg := range SELinuxFileLabelTestArgs {
		stat, _ := os.Lstat(testArg.path)
		if testArg.filter == nil || (!testArg.filter(s, testArg.path, stat)) {
			assertSELinuxFileContext(testArg.path, testArg.context)
		}
		if testArg.recursive {
			checkSELinuxFileContextRecursively(testArg.path, testArg.context, testArg.filter)
		}
	}
}
