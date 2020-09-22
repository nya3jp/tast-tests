// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"os/exec"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// ExecFuncOffline disconnect Chrome browser internet connection through iptables.
// Then it executes given function and reverts it back to the original state.
func ExecFuncOffline(ctx context.Context, f func() error) (result error) {
	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to lock the check network hook")
	}
	defer unlock()

	const iptablesApp = "iptables"

	// Block all output traffic from chronos user, except localhost
	// Localhost must be excluded from this rule, otherwise tast server cannot be connected either.
	iptablesBlockArgs := []string{"OUTPUT", "-m", "owner", "--uid-owner", "chronos", "!", "-d", "127.0.0.1", "-j", "REJECT", "-w"}

	// Insert blocking rule into the first line
	blockChromeCommand := testexec.CommandContext(ctx, iptablesApp, append([]string{"-t", "filter", "-I"}, iptablesBlockArgs...)...)

	// Using exec.Command instead of testexec.CommandContext.
	// So it will be executed in case test failed on timeout.
	resumeChromeCommand := exec.Command(iptablesApp, append([]string{"-t", "filter", "-D"}, iptablesBlockArgs...)...)

	// Block Chrome traffic in iptables
	testing.ContextLog(ctx, "Blocking Chrome internet connection")
	if err := blockChromeCommand.Run(testexec.DumpLogOnError); err != nil {
		result = errors.Wrapf(err, "failed to block chrome traffic in iptables with command %v", iptablesBlockArgs)
		return
	}

	defer func() {
		if err := resumeChromeCommand.Run(); err != nil {
			result = errors.Wrapf(err, "failed to resume Chrome internet connection. Offline test result: %v", result)
		} else {
			testing.ContextLog(ctx, "Resumed Chrome internet connection")
		}
	}()

	result = f()
	return
}
