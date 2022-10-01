// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package biod

import (
	"context"

	"golang.org/x/sys/unix"

	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DevicePermissions,
		Desc: "Checks /dev/cros_fp's permissions and owner/group",
		Contacts: []string{
			"hesling@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
	})
}

// DevicePermissions checks that /dev/cros_fp has the correct file permission
// and a reasonable owner/group.
//
// We don't check what exact group owns the device, but we do ensure that it
// isn't root.
func DevicePermissions(ctx context.Context, s *testing.State) {
	var info unix.Stat_t
	if err := unix.Stat("/dev/cros_fp", &info); err != nil {
		s.Fatal("Failed to stat /dev/cros_fp: ", err)
	}

	// Remove type and extra bits.
	permTest := info.Mode & 0o777
	// Expect "u=rw,g=rw,o=" permission.
	permExpected := uint32(0o660)

	if permTest != permExpected {
		s.Errorf("Unexpected permissions for /dev/cros_fp: got %o, want %o",
			permTest, permExpected)
	}
	if id := info.Uid; id != 0 {
		s.Errorf("Unexpected file owner for /dev/cros_fp: got uid %d, want root", id)
	}
	if info.Gid == 0 {
		s.Error("Unexpected file group for /dev/cros_fp: got root(0), want non-root")
	}
}
