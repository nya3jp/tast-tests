// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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
		Func: DevicePermission,
		Desc: "Checks that /dev/cros_fp has ",
		Contacts: []string{
			"hesling@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
	})
}

// DevicePermission checks that /dev/cros_fp has a reduced access permissions.
//
// We don't check what exact group owns the device, but we do ensure that it
// isn't root.
func DevicePermission(ctx context.Context, s *testing.State) {
	var info unix.Stat_t
	err := unix.Stat("/dev/cros_fp", &info)
	if err != nil {
		s.Fatal("Failed to stat /dev/cros_fp: ", err)
	}

	// Remove type and extra bits.
	permTest := info.Mode & 0o777
	// Expect "u=rw,g=rw,o=" permission.
	permExpected := uint32(0o660)

	if permTest != permExpected {
		s.Errorf("The /dev/cros_fp file has permission %o, but we expected %o",
			permTest, permExpected)
	}
	if id := info.Uid; id != 0 {
		s.Errorf("The /dev/cros_fp file owner is uid %d, but we expected root (0)", id)
	}
	if id := info.Gid; id == 0 {
		s.Error("The /dev/cros_fp file group is root (0), but we expected not root (0)")
	}
}
