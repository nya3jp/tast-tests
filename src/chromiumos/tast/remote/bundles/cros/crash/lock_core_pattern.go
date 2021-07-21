// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"

	"chromiumos/tast/dut"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LockCorePattern,
		Desc:         "Verify locked |core_pattern| after `crash_reporter --init` on kernels <= 3.18",
		Contacts:     []string{"sarthakkukreti@chromium.org", "cros-telemetry@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"lock_core_pattern", "reboot"},
	})
}

// initCrashReporter invokes the crash reporter initialization as expected during boot.
func initCrashReporter(ctx context.Context, d *dut.DUT) error {
	if err := d.Conn().CommandContext(ctx, "touch", "/run/crash_reporter/crash-test-in-progress").Run(); err != nil {
		return err
	}

	return d.Conn().CommandContext(ctx, "/sbin/crash_reporter", "--init").Run()
}

func LockCorePattern(ctx context.Context, s *testing.State) {
	d := s.DUT()

	// Reboot device: other tests may need to modify the |core_pattern|.
	defer func() {
		s.Log("Rebooting DUT")
		if err := d.Reboot(ctx); err != nil {
			s.Fatal("Failed to reboot: ", err)
		}
	}()

	if err := initCrashReporter(ctx, d); err != nil {
		s.Fatal("Unable to initialize crash reporter: ", err)
	}

	// Try to modify core_pattern.
	cmd := d.Conn().CommandContext(ctx, "sh", "-c", "echo 'hello' > /proc/sys/kernel/core_pattern")
	if err := cmd.Run(); err == nil {
		s.Fatal("|core_pattern| writable after crash_reporter initialization")
	} else {
		// TODO(sarthakkukreti): Verify whether the error type was an expected one, or remove this message.
		s.Log("Write to |core_pattern| failed as expected: ", err)
	}
}
