// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/security/selinux"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SELinuxProcessesARC,
		Desc:         "Checks that host processes are running in correct SELinux domain after ARC boots",
		Contacts:     []string{"niwa@chromium.org", "fqj@chromium.org", "jorgelo@chromium.org", "chromeos-security@google.com"},
		SoftwareDeps: []string{"selinux", "chrome"},
		Pre:          arc.Booted(),
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			// TODO(b/182216018): Move this out of informational.
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraAttr:         []string{"informational"},
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
		if string(c) != expected {
			s.Errorf("Android init context must be %q but got %q", expected, string(c))
		}
	}

	// Check everything else.
	selinux.ProcessesTestInternal(ctx, s, []selinux.ProcessTestCaseSelector{selinux.Stable})
}
