// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"fmt"

	"chromiumos/tast/dut"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LockCorePattern,
		Desc:         "Verify locked |core_pattern| after `crash_reporter --init` on kernels < 3.18",
		Contacts:     []string{"sarthakkukreti@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"reboot"},
	})
}

func checkFileExists(ctx context.Context, d *dut.DUT, filename string) bool {
	cmd := fmt.Sprintf("stat %s", shutil.Escape(filename))

	_, err := d.Run(ctx, cmd)

	return err == nil
}

func initCrashReporter(ctx context.Context, d *dut.DUT) bool {
	cmd := `touch /run/crash_reporter/crash-test-in-progress
	/sbin/crash_reporter --init`

	_, err := d.Run(ctx, cmd)

	return err == nil
}

func reboot(ctx context.Context, d *dut.DUT, s *testing.State) {
	// Run the reboot command in the background to avoid the DUT potentially going down before
	// success is reported over the SSH connection. Redirect all I/O streams to ensure that the
	// SSH exec request doesn't hang (see https://en.wikipedia.org/wiki/Nohup#Overcoming_hanging).
	cmd := "nohup sh -c 'sleep 2; reboot' >/dev/null 2>&1 </dev/null &"
	if _, err := d.Run(ctx, cmd); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	s.Log("Waiting for DUT to become unreachable")
	if err := d.WaitUnreachable(ctx); err != nil {
		s.Fatal("Failed to wait for DUT to become unreachable: ", err)
	}
	s.Log("DUT became unreachable (as expected)")
	s.Log("Reconnecting to DUT")
	if err := d.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}
	s.Log("Reconnected to DUT")
}

func LockCorePattern(ctx context.Context, s *testing.State) {
	d, ok := dut.FromContext(ctx)

	if !ok {
		s.Fatal("Failed to get DUT")
	}

	if checkFileExists(ctx, d, "/proc/sys/kernel/lock_core_pattern") {
		if !initCrashReporter(ctx, d) {
			s.Fatal("Unable to initialize crash reporter")
		}

		cmd := `echo "hello" > /proc/sys/kernel/core_pattern`
		if _, err := d.Run(ctx, cmd); err == nil {
			s.Fatal("|core_pattern| writeable after crash_reporter initialization")
		} else {
			s.Log("Expected failure to write to |core_pattern|: ", err)
		}

		// Reboot device: other tests may need to modify the |core_pattern|.
		s.Log("Rebooting DUT")
		reboot(ctx, d, s)
	}
}
