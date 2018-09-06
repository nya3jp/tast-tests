// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"os"
	"syscall"

	"chromiumos/tast/local/bundles/cros/security/selinux"
	"chromiumos/tast/testing"
)

type checkArgs struct {
	path      string
	context   string
	recursive bool
	filter    selinux.SELinuxFileLabelCheckFilter
}

// SELinux file label check args
var testArgs = []checkArgs{
	checkArgs{"/sbin/init", "u:object_r:chromeos_init_exec:s0", false, nil},
	checkArgs{"/run/cras", "u:object_r:cras_socket:s0", true, nil},
	checkArgs{"/sys/fs/cgroup", "u:object_r:tmpfs:s0", false, nil},
	checkArgs{"/sys/fs/cgroup", "u:object_r:cgroup:s0", true, selinux.IgnorePath("/sys/fs/cgroup")},
	checkArgs{"/sys/fs/pstore", "u:object_r:pstorefs:s0", false, nil},
	checkArgs{"/sys/fs/selinux", "u:object_r:selinuxfs:s0", true, selinux.IgnorePath("/sys/fs/selinux/null")},
	checkArgs{"/sys/fs/selinux", "u:object_r:null_device:s0", true, selinux.FilterReverse(selinux.IgnorePath("/sys/fs/selinux/null"))},
	checkArgs{"/run/chrome/wayland-0", "u:object_r:wayland_socket:s0", false, nil},
	checkArgs{"/sys/kernel/config", "u:object_r:configfs:s0", false, nil},
	checkArgs{"/sys/kernel/debug", "u:object_r:debugfs:s0", false, nil},
	checkArgs{"/sys/kernel/debug/tracing", "u:object_r:debugfs_tracing:s0", false, nil},
	checkArgs{"/sys/kernel/debug/debugfs_tracing_on", "u:object_r:debugfs_tracing:s0", false, selinux.SkipNonExist},
	checkArgs{"/sys/kernel/debug/trace_marker", "u:object_r:debugfs_trace_marker:s0", false, selinux.SkipNonExist},
	checkArgs{"/sys/devices/system/cpu", "u:object_r:sysfs_devices_system_cpu:s0", true, func(s *testing.State, p string, f os.FileInfo) bool {
		mode := f.Mode()
		if mode.IsRegular() && ((mode.Perm() & (syscall.S_IWUSR | syscall.S_IWGRP | syscall.S_IWOTH)) > 0) {
			// The writable file should be u:object_r:sysfs:s0
			selinux.AssertSELinuxFileContext(s, p, "u:object_r:sysfs:s0")
			return true
		}
		return false
	}},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SELinuxFileLabel,
		Desc:         "Checks that SELinux file labels are set correctly.",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"selinux"},
	})
}

func SELinuxFileLabel(s *testing.State) {
	for _, testArg := range testArgs {
		stat, _ := os.Lstat(testArg.path)
		if testArg.filter == nil || (!testArg.filter(s, testArg.path, stat)) {
			selinux.AssertSELinuxFileContext(s, testArg.path, testArg.context)
		}
		if testArg.recursive {
			selinux.CheckSELinuxFileContextRecursively(s, testArg.path, testArg.context, testArg.filter)
		}
	}

}
