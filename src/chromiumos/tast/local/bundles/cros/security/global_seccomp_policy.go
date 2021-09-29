// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"syscall"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: GlobalSeccompPolicy,
		Desc: "Tests that the global seccomp policy is installed and enforcing properly",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"nvaa@google.com",      // Security team
			"chromeos-security@google.com",
		},
		SoftwareDeps: []string{"global_seccomp_policy"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

// GlobalSeccompPolicy checks that calling ioctl with LOOP_CHANGE_FD
// fails with a permission denied error.
func GlobalSeccompPolicy(ctx context.Context, s *testing.State) {
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, 0, 0x4C06, 0)
	if err == 0 {
		s.Error("LOOP_CHANGE_FD ioctl succeeded")
	}
	if err != syscall.EPERM {
		s.Errorf("LOOP_CHANGE_FD ioctl failed with %q, not with EPERM",
			err.Error())
	}

}
