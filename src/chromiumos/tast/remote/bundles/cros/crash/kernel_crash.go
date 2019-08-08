// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"

	"chromiumos/tast/dut"
	//"chromiumos/tast/local/bundles/cros/platform/crash"
	"chromiumos/tast/testing"
)

const (
	systemCrashDir = "/var/spool/crash"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: KernelCrash,
		Desc: "Verify artificial kernel crash creates crash files",
		Contacts: []string{"mutexlox@chromium.org",
			"cros-monitoring-forensics@google.com"},
		Attr: []string{"informational"},
		/*Data: []string{
			crash.MockMetricsOnPolicyFile,
			crash.MockMetricsOwnerKeyFile,
		},*/
	})
}

func KernelCrash(ctx context.Context, s *testing.State) {
	d, ok := dut.FromContext(ctx)
	if !ok {
		s.Fatal("Failed to get DUT")
	}

	/*if err := crash.SetConsent(ctx, s.DataPath(crash.MockMetricsOnPolicyFile), s.DataPath(crash.MockMetricsOwnerKeyFile)); err != nil {
		s.Fatal("Failed to set consent: ", err)
	}*/

	if _, err := d.Run(ctx, "rm -rf "+systemCrashDir); err != nil {
		s.Fatal("Failed to clean crash dir")
	}

	// Trigger a panic
	// Run the triggering command in the background to avoid the DUT potentially going down before
	// success is reported over the SSH connection. Redirect all I/O streams to ensure that the
	// SSH exec request doesn't hang (see https://en.wikipedia.org/wiki/Nohup#Overcoming_hanging).
	cmd := `nohup sh -c 'sleep 2
	if [ -f /sys/kernel/debug/provoke-crash/DIRECT ]; then
		echo PANIC > /sys/kernel/debug/provoke-crash/DIRECT
	else
		echo panic > /proc/breakme
	fi' >/dev/null 2>&1 </dev/null &`
	if _, err := d.Run(ctx, cmd); err != nil {
		s.Fatal("Failed to panic DUT: ", err)
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

	if _, err := d.Run(ctx, "test -s /var/spool/crash/kernel.*.0.bios_log"); err != nil {
		s.Error("Failed to create bios_log file")
	}

	if _, err := d.Run(ctx, "test -s /var/spool/crash/kernel.*.0.kcrash"); err != nil {
		s.Error("Failed to create kcrash file")
	}

	if _, err := d.Run(ctx, "test -s /var/spool/crash/kernel.*.0.meta"); err != nil {
		s.Error("Failed to create kcrash file")
	}
}
