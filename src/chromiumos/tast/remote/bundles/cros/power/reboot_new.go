// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RebootNew,
		Desc:         "Verifies that system comes back after rebooting (with a new reboot method)",
		Contacts:     []string{"nya@chromium.org", "tast-owners@google.com"},
		SoftwareDeps: []string{"reboot"},
		Attr:         []string{"informational"},
	})
}

func RebootNew(ctx context.Context, s *testing.State) {
	d, ok := dut.FromContext(ctx)
	if !ok {
		s.Fatal("Failed to get DUT")
	}

	if err := reboot(ctx, d); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}
}

// reboot reboots the DUT.
//
// TODO(crbug.com/971024): Move this method to dut.DUT after making sure it is stable.
// TODO(crbug.com/971024): Remove verbose logging when we move this method to dut.DUT.
func reboot(ctx context.Context, d *dut.DUT) error {
	testing.ContextLog(ctx, "Rebooting DUT")

	readBootID := func(ctx context.Context) (string, error) {
		out, err := d.Command("cat", "/proc/sys/kernel/random/boot_id").Output(ctx)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	}

	initID, err := readBootID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to read initial boot_id")
	}
	testing.ContextLogf(ctx, "Initial boot_id = %s", initID)

	// Run the reboot command with a short timeout. This command can block for long time
	// if the network interface of the DUT goes down before the SSH command finishes.
	rebootCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	d.Command("reboot").Run(rebootCtx) // ignore the error

	testing.ContextLog(ctx, "Waiting for DUT to boot")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Set a short timeout to the iteration in case of any of SSH operations
		// blocking for long time.
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := d.WaitConnect(ctx); err != nil {
			return errors.Wrap(err, "failed to connect to DUT")
		}
		curID, err := readBootID(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to read boot_id")
		}
		if curID == initID {
			return errors.New("boot_id did not change")
		}
		testing.ContextLogf(ctx, "New boot_id = %s", curID)
		return nil
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return errors.Wrap(err, "failed to wait for DUT to reboot")
	}
	return nil
}
