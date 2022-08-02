// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/security/selinux"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SELinuxProcessesARC,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that host processes are running in correct SELinux domain after ARC boots",
		Contacts:     []string{"niwa@chromium.org", "fqj@chromium.org", "jorgelo@chromium.org", "chromeos-security@google.com"},
		SoftwareDeps: []string{"selinux", "chrome"},
		Pre:          arc.Booted(),
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func SELinuxProcessesARC(ctx context.Context, s *testing.State) {
	// Check Android init domain.
	a := s.PreValue().(arc.PreData).ARC
	c, err := a.Command(ctx, "cat", "/proc/1/attr/current").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Error("Failed to check Android init context: ", err)
	} else {
		const expected = "u:r:init:s0\x00"
		received := string(c)
		if received == "" {
			s.Errorf("ARC failed to start, Android init context must be %q but is empty", expected)
		} else if received != expected {
			s.Errorf("Android init context must be %q but got %q", expected, received)
		}
	}

	// Check everything else.
	selinux.ProcessesTestInternal(ctx, s, []selinux.ProcessTestCaseSelector{selinux.Stable})
}
