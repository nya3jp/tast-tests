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

func init() {
	testing.AddTest(&testing.Test{
		Func:         SELinuxFileLabel,
		Desc:         "Checks that SELinux file labels are set correctly",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"selinux"},
	})
}

func SELinuxFileLabel(s *testing.State) {
	systemCPUFilter := func(p string, fi os.FileInfo) bool {
		mode := fi.Mode()
		if mode.IsRegular() && ((mode.Perm() & (syscall.S_IWUSR | syscall.S_IWGRP | syscall.S_IWOTH)) > 0) {
			return true
		}
		return false
	}
	for _, testArg := range []struct {
		path, context string
		recursive     bool
		filter        selinux.FileLabelCheckFilter
	}{
		{"/sbin/init", "u:object_r:chromeos_init_exec:s0", false, nil},
		{"/run/cras", "u:object_r:cras_socket:s0", true, nil},
		{"/sys/fs/cgroup", "u:object_r:tmpfs:s0", false, nil},
		{"/sys/fs/cgroup", "u:object_r:cgroup:s0", true, selinux.IgnorePath("/sys/fs/cgroup")},
		{"/sys/fs/pstore", "u:object_r:pstorefs:s0", false, nil},
		{"/sys/fs/selinux", "u:object_r:selinuxfs:s0", true, selinux.IgnorePath("/sys/fs/selinux/null")},
		{"/sys/fs/selinux", "u:object_r:null_device:s0", true, selinux.InvertFilter(selinux.IgnorePath("/sys/fs/selinux/null"))},
		{"/run/chrome/wayland-0", "u:object_r:wayland_socket:s0", false, nil},
		{"/sys/kernel/config", "u:object_r:configfs:s0", false, selinux.SkipNonExist},
		{"/sys/kernel/debug", "u:object_r:debugfs:s0", false, nil},
		{"/sys/kernel/debug/tracing", "u:object_r:debugfs_tracing:s0", false, nil},
		{"/sys/kernel/debug/debugfs_tracing_on", "u:object_r:debugfs_tracing:s0", false, selinux.SkipNonExist},
		{"/sys/kernel/debug/tracing/trace_marker", "u:object_r:debugfs_trace_marker:s0", false, selinux.SkipNonExist},
		{"/sys/devices/system/cpu", "u:object_r:sysfs_devices_system_cpu:s0", true, systemCPUFilter},
		{"/sys/devices/system/cpu", "u:object_r:sysfs:s0", true, selinux.InvertFilter(systemCPUFilter)},
		{"/var/log/mount-encrypted.log", "u:object_r:cros_var_log:s0", false, nil},
	} {
		filter := testArg.filter
		if filter == nil {
			filter = selinux.CheckAll
		}
		selinux.CheckContext(s, testArg.path, testArg.context, testArg.recursive, filter)
	}

}
