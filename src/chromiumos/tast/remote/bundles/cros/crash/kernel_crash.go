// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
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
	})
}

// waitForNonEmptyGlobsWithTimeout waits for file matching each glob specified in |files| to exist and be non-empty, up until |timeout|.
func waitForNonEmptyGlobsWithTimeout(ctx context.Context, d *dut.DUT, files []string, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		for _, glob := range files {
			if out, err2 := d.Run(ctx, "test -s "+glob); err2 != nil {
				msg := glob + ": " + err2.Error() + "\n output: \n"
				if len(out) != 0 {
					msg += string(out)
				}
				err = errors.Wrap(err, "\n\n"+msg)
			}
		}
		return err
	}, &testing.PollOptions{Interval: 500 * time.Millisecond,
		Timeout: timeout})
}

func KernelCrash(ctx context.Context, s *testing.State) {
	d, ok := dut.FromContext(ctx)
	if !ok {
		s.Fatal("Failed to get DUT")
	}

	if _, err := d.Run(ctx, "rm -rf "+systemCrashDir); err != nil {
		s.Fatal("Failed to clean crash dir")
	}
	// Sync filesystem so that when kernel crashes the rm is persisted.
	if _, err := d.Run(ctx, "sync --file-system /var/spool"); err != nil {
		s.Fatal("Failed to sync crash dir's filesystem")
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

	timeout := time.Second * 30
	globs := []string{systemCrashDir + "/kernel.*.0.bios_log",
		systemCrashDir + "/kernel.*.0.kcrash",
		systemCrashDir + "/kernel.*.0.meta"}

	s.Log("Waiting for files to become present")
	if err := waitForNonEmptyGlobsWithTimeout(ctx, d, globs, timeout); err != nil {
		s.Error("Failed to find crash files: " + err.Error())
	}

	if out, err := d.Run(ctx, "rm -f "+systemCrashDir+"/*"); err != nil {
		s.Fatal("Failed to clean crash dir: " + string(out))
	}
}
