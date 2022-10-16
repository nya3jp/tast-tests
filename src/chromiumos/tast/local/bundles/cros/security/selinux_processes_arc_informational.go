// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/security/selinux"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SELinuxProcessesARCInformational,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that processes are running in correct SELinux domain (new and flaky tests) after ARC boots",
		Contacts:     []string{"niwa@chromium.org", "jorgelo@chromium.org", "chromeos-security@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"selinux", "chrome"},
		Pre:          arc.Booted(),
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func SELinuxProcessesARCInformational(ctx context.Context, s *testing.State) {
	selinux.ProcessesTestInternal(ctx, s, []selinux.ProcessTestCaseSelector{selinux.Unstable})
}
