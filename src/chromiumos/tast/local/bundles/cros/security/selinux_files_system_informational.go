// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"

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
	// Files to be tested.
	testArgs := []selinux.FileTestCase{
		{Path: "/var/log/arc.log", Context: "cros_arc_log", Recursive: false, Filter: nil, Log: true},
		{Path: "/var/log/boot.log", Context: "cros_boot_log", Recursive: false, Filter: nil, Log: true},
		{Path: "/var/log/messages", Context: "cros_syslog", Recursive: false, Filter: nil, Log: true},
		{Path: "/var/log/net.log", Context: "cros_net_log", Recursive: false, Filter: nil, Log: true},
		{Path: "/var/log/secure", Context: "cros_secure_log", Recursive: false, Filter: nil, Log: true},
	}

	selinux.FilesTestInternal(ctx, s, testArgs)
}
