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
		Desc:         "Verify locked |core_pattern| after `crash_reporter --init` on kernels < 3.18",
		Contacts:     []string{"sarthakkukreti@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"lock_core_pattern", "reboot"},
	})
}

// initCrashReporter invokes the crash reporter initialization as expected during boot.
func initCrashReporter(ctx context.Context, d *dut.DUT) error {
	cmd := d.Command("touch", "/run/crash_reporter/crash-test-in-progress")
	if err := cmd.Run(ctx); err != nil {
		return err
	}

	crashReporterInit := d.Command("/sbin/crash_reporter", "--init")
	return crashReporterInit.Run(ctx)
}

func LockCorePattern(ctx context.Context, s *testing.State) {
	d, ok := dut.FromContext(ctx)
	if !ok {
		s.Fatal("Failed to get DUT")
	}

	// Reboot device: other tests may need to modify the |core_pattern|.
	defer func() {
		// Cleanup test in progress marker.
		cmd := d.Command("rm", "/run/crash_reporter/crash-test-in-progress")
		cmd.Run(ctx)
		s.Log("Rebooting DUT")
		if err := d.Reboot(ctx); err != nil {
			s.Fatal("Failed to reboot: ", err)
		}
	}()

	if err := initCrashReporter(ctx, d); err != nil {
		s.Fatal("Unable to initialize crash reporter: ", err)
	}

	// Try to modify |core_pattern|.
	cmd := d.Command("echo 'hello' > /proc/sys/kernel/core_pattern")
	if err := cmd.Run(ctx); err == nil {
		s.Fatal("|core_pattern| writeable after crash_reporter initialization")
	} else {
		s.Log("Expected failure to write to |core_pattern|: ", err)
	}
}
