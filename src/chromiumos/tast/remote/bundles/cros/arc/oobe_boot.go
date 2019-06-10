// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OOBEBoot,
		Desc: "Checks that Android boots in out-of-box experience flow",
		// TODO(nya): Add a proper contact of ARC boot tests.
		Contacts:     []string{"nya@chromium.org", "arc-eng@google.com"},
		SoftwareDeps: []string{"android", "chrome", "reboot"},
		Attr:         []string{"informational"},
		Timeout:      7 * time.Minute, // 3 min. for DUT reboot + 4 min. for ARC boot
	})
}

// reboot reboots the DUT.
//
// TODO(nya): Move this method to dut.DUT.
func reboot(ctx context.Context, d *dut.DUT) error {
	testing.ContextLog(ctx, "Rebooting DUT")

	readBootID := func() (string, error) {
		out, err := d.Run(ctx, "cat /proc/sys/kernel/random/boot_id")
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	}

	initID, err := readBootID()
	if err != nil {
		return errors.Wrap(err, "failed to read initial boot_id")
	}

	// Run the reboot command in the background to avoid the DUT potentially going down before
	// success is reported over the SSH connection. Redirect all I/O streams to ensure that the
	// SSH exec request doesn't hang (see https://en.wikipedia.org/wiki/Nohup#Overcoming_hanging).
	if _, err := d.Run(ctx, "nohup sh -c 'sleep 2; reboot' >/dev/null 2>&1 </dev/null &"); err != nil {
		return errors.Wrap(err, "reboot command failed")
	}

	testing.ContextLog(ctx, "Waiting for DUT to reboot")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := d.WaitConnect(ctx); err != nil {
			return errors.Wrap(err, "failed to connect to DUT")
		}
		if curID, err := readBootID(); err != nil {
			return errors.Wrap(err, "failed to read boot_id")
		} else if curID == initID {
			return errors.New("boot_id did not change")
		}
		return nil
	}, nil); err != nil {
		return errors.Wrap(err, "failed to wait for DUT to reboot")
	}
	return nil
}

func OOBEBoot(ctx context.Context, s *testing.State) {
	d, ok := dut.FromContext(ctx)
	if !ok {
		s.Fatal("Failed to get DUT")
	}

	// Clear TPM ownership to clear the stateful partition and enter OOBE after reboot.
	// NOTE: Do not clobber files in /mnt/stateful_partition. It puts the TPM to a bad state and
	// local_test_runner -waituntilready will block forever. See crbug.com/901363#c31.
	// TODO(nya): This does not work at least on VM. Think of alternative way.
	if _, err := d.Run(ctx, "crossystem clear_tpm_owner_request=1"); err != nil {
		s.Fatal("Failed to request clearing TPM ownership: ", err)
	}

	if err := reboot(ctx, d); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	s.Log("Waiting for system stabilization")
	if out, err := d.Run(ctx, "local_test_runner -waituntilready example.Pass"); err != nil {
		if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "local_test_runner.txt"), out, 0644); err != nil {
			s.Error("Failed to save local_test_runner output: ", err)
		}
		s.Fatal("Failed to wait for system stabilization: ", err)
	}

	// Make sure the stateful partition was cleared and we are going through the OOBE flow.
	if _, err := d.Run(ctx, "stat /home/chronos/.oobe_completed"); err == nil {
		s.Fatal("Failed to clear the stateful partition")
	}

	// Create a temporary directory to store local test output.
	out, err := d.Run(ctx, "mktemp -d -p /usr/local/tmp")
	if err != nil {
		s.Fatal("Failed to create a temporary directory: ", err)
	}
	remoteOutDir := strings.TrimSpace(string(out))

	// Adjust the timeout to allow some time for copying logs.
	testCtx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	const testName = "arc.Boot"

	s.Logf("Running %s", testName)
	// TODO(nya): Do RPC calls to test ARC boot, rather than running arc.Boot local test.
	// Be aware that local_test_runner runs arc.Boot test in the pre-installed cros test bundle.
	// Changes to the arc.Boot code don't apply even if you run "tast run -build=true".
	out, err = d.Run(testCtx, fmt.Sprintf("local_test_runner -outdir %s %s", shutil.Escape(remoteOutDir), shutil.Escape(testName)))
	if err != nil {
		s.Errorf("%s failed: %v", testName, err)
	} else {
		s.Logf("%s passed", testName)
	}

	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "local_test_runner.txt"), out, 0644); err != nil {
		s.Error("Failed to save local_test_runner output: ", err)
	}

	s.Log("Copying logs")

	// TODO(crbug.com/971024): Consider using pipes to avoid holding the entire output in memory.
	out, err = d.Run(ctx, fmt.Sprintf("tar cz -C %s .", shutil.Escape(remoteOutDir)))
	if err != nil {
		s.Fatal("Failed to run remote tar command: ", err)
	}

	cmd := testexec.CommandContext(ctx, "tar", "xz", "-C", s.OutDir())
	cmd.Stdin = bytes.NewBuffer(out)
	if err := cmd.Run(); err != nil {
		s.Error("Failed to run local tar command: ", err)
	}
}
