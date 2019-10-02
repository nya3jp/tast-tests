// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
		Func:         SELinuxProcessesInformational,
		Desc:         "Checks that processes are running in correct SELinux domain (new and flaky tests)",
		Contacts:     []string{"fqj@chromium.org", "jorgelo@chromium.org", "chromeos-security@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"selinux"},
	})
}

func SELinuxProcessesInformational(ctx context.Context, s *testing.State) {
	selinux.ProcessesTestInternal(ctx, s, []selinux.ProcessTestCaseSelector{selinux.Unstable})
}
