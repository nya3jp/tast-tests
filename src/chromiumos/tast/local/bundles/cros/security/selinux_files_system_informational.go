// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"os"
	"syscall"

	"chromiumos/tast/local/bundles/cros/security/selinux"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SELinuxFilesSystemInformational,
		Desc:         "Checks that SELinux file labels are set correctly for system files (new testcases, flaky testcases)",
		Contacts:     []string{"fqj@chromium.org", "jorgelo@chromium.org", "chromeos-security@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"selinux"},
	})
}

func SELinuxFilesSystemInformational(ctx context.Context, s *testing.State) {
	type rwFilter int
	const (
		readonly rwFilter = iota
		writable
	)
	systemCPUFilter := func(writableFilter rwFilter) selinux.FileLabelCheckFilter {
		return func(p string, fi os.FileInfo) (skipFile, skipSubdir selinux.FilterResult) {
			mode := fi.Mode()
			// Domain has search to both sysfs and sysfs_devices_system_cpu.
			if mode.IsDir() {
				return selinux.Skip, selinux.Check
			}

			isWritable := mode.IsRegular() && ((mode.Perm() & (syscall.S_IWUSR | syscall.S_IWGRP | syscall.S_IWOTH)) > 0)
			// Writable files
			if isWritable != (writableFilter == writable) {
				return selinux.Skip, selinux.Check
			}

			return selinux.Check, selinux.Check
		}
	}

	// Files to be tested.
	testArgs := []selinux.FileTestCase{
		{Path: "/var/log/arc.log", Context: "cros_arc_log", Log: true},
		{Path: "/var/log/boot.log", Context: "cros_boot_log", Log: true},
		{Path: "/var/log/messages", Context: "cros_syslog", Log: true},
		{Path: "/var/log/net.log", Context: "cros_net_log", Log: true},
		{Path: "/var/log/secure", Context: "cros_secure_log", Log: true},
		{Path: "/sys", Context: "sysfs.*", Recursive: true, Filter: selinux.IgnorePathsRegex(append(append([]string{
			"/sys/bus/iio/devices",
			"/sys/class/drm",
			"/sys/devices/system/cpu",
			"/sys/fs/cgroup",
			"/sys/fs/pstore",
			"/sys/fs/selinux",
			"/sys/kernel/config",
			"/sys/kernel/debug",
			// we don't have anything special of conntrack files than others. conntrack slab cache changes when connections established or closes, and may cause flakiness.
			"/sys/kernel/slab/nf_conntrack_.*",
		}, gpuDevices...), crosEcIioDevices...))},
		{Path: "/sys/devices/system/cpu", Context: "sysfs", Recursive: true, Filter: systemCPUFilter(writable)},
		{Path: "/sys/devices/system/cpu", Context: "sysfs_devices_system_cpu", Recursive: true, Filter: systemCPUFilter(readonly)},
		{Path: "/sys/fs/cgroup", Context: "cgroup", Recursive: true, Filter: selinux.IgnorePathButNotContents("/sys/fs/cgroup")},
		{Path: "/sys/fs/cgroup", Context: "tmpfs"},
		{Path: "/sys/fs/pstore", Context: "pstorefs"},
		{Path: "/sys/fs/selinux", Context: "selinuxfs", Recursive: true, Filter: selinux.IgnorePathButNotContents("/sys/fs/selinux/null")},
		{Path: "/sys/fs/selinux/null", Context: "null_device"},
		{Path: "/sys/kernel/config", Context: "configfs", IgnoreErrors: true},
		{Path: "/sys/kernel/debug", Context: "debugfs"},
		{Path: "/sys/kernel/debug/debugfs_tracing_on", Context: "debugfs_tracing", IgnoreErrors: true},
		{Path: "/sys/kernel/debug/tracing", Context: "debugfs_tracing"},
		{Path: "/sys/kernel/debug/tracing/trace_marker", Context: "debugfs_trace_marker", IgnoreErrors: true},
		{Path: "/sys/kernel/debug/sync", Context: "debugfs_sync", IgnoreErrors: true},
		{Path: "/sys/kernel/debug/sync/info", Context: "debugfs_sync", IgnoreErrors: true},
	}

	selinux.FilesTestInternal(ctx, s, testArgs)
}
