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
		Contacts:     []string{"fqj@chromium.org", "kroot@chromium.org", "chromeos-security@google.com"},
		Attr:         []string{"informational"},
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
		{"/sys/devices/system/cpu", "sysfs", true, systemCPUFilter(writable), false},
		{"/sys/devices/system/cpu", "sysfs_devices_system_cpu", true, systemCPUFilter(readonly), false},
		{"/var/log/arc.log", "cros_arc_log", false, nil, true},
		{"/var/log/authpolicy.log", "cros_authpolicy_log", false, nil, true},
		{"/var/log/boot.log", "cros_boot_log", false, nil, true},
		{"/var/log/eventlog.txt", "cros_var_log_eventlog", false, nil, true},
		{"/var/log/messages", "cros_syslog", false, nil, true},
		{"/var/log/mount-encrypted.log", "cros_var_log", false, nil, true},
		{"/var/log/net.log", "cros_net_log", false, nil, true},
		{"/var/log/secure", "cros_secure_log", false, nil, true},
		{"/var/log/tlsdate.log", "cros_tlsdate_log", false, nil, true},
		{"/var/log/asan", "cros_var_log_asan", true, nil, true},
	}

	selinux.FilesTestInternal(ctx, s, testArgs)
}
