// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io/ioutil"
	"os"

	"chromiumos/tast/dut"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LockCorePattern,
		Desc:         "Verify locked |core_pattern| after `crash_reporter --init` on kernels < 3.18",
		Contacts:     []string{"sarthakkukreti@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"lock_core_pattern", "reboot"},
	})
}

// initCrashReporter invokes the crash reporter initialization as expected during boot.
func initCrashReporter(ctx context.Context, d *dut.DUT) error {
	if err := d.Command("touch", "/run/crash_reporter/crash-test-in-progress").Run(ctx); err != nil {
		return err
	}

	return d.Command("/sbin/crash_reporter", "--init").Run(ctx)
}

func LockCorePattern(ctx context.Context, s *testing.State) {
	d := s.DUT()

	// Reboot device: other tests may need to modify the |core_pattern|.
	defer func() {
		// Cleanup test in progress marker.
		if err := d.Command("rm", "-f", "/run/crash_reporter/crash-test-in-progress").Run(ctx); err != nil {
			s.Log("Failed to cleanup crash-reporter test-in-progress marker")
		}
		s.Log("Rebooting DUT")
		if err := d.Reboot(ctx); err != nil {
			s.Fatal("Failed to reboot: ", err)
		}
	}()

	if err := initCrashReporter(ctx, d); err != nil {
		s.Fatal("Unable to initialize crash reporter: ", err)
	}

	// Try to modify |core_pattern|.
	if err := ioutil.WriteFile("/proc/sys/kernel/core_pattern", []byte("hello"), os.FileMode(0644)); err == nil {
		s.Fatal("|core_pattern| writable after crash_reporter initialization")
	} else {
		// TODO(sarthakkukreti): Verify it the error type was as expected, or remove this message.
		s.Log("Write to |core_pattern| failed as expected: ", err)
	}
}
