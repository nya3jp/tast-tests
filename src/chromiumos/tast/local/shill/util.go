// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file provides small helpers, packaging shill APIs in ways to ease their
// use by others.

package shill

import (
	"context"
	"os/exec"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// WaitForOnline waits for Internet connectivity, a shorthand which is useful so external packages don't have to worry
// about Shill details (e.g., Service, Manager). Tests that require Internet connectivity (e.g., for a real GAIA login)
// need to ensure that before trying to perform Internet requests. This function is one way to do that.
// Returns an error if we don't come back online within a reasonable amount of time.
func WaitForOnline(ctx context.Context) error {
	m, err := NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to shill's Manager")
	}

	expectProps := map[string]interface{}{
		shillconst.ServicePropertyState: shillconst.ServiceStateOnline,
	}
	if _, err := m.WaitForServiceProperties(ctx, expectProps, 15*time.Second); err != nil {
		return errors.Wrap(err, "network did not come back online")
	}

	return nil
}

// ExecFuncOnChromeOffline disconnect Chrome browser internet connection through iptables.
// Then it executes given function and reverts it back to the original state.
func ExecFuncOnChromeOffline(ctx context.Context, f func() error) (result error) {
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
	iptablesBlockArgs := []string{"OUTPUT", "-m", "owner", "--uid-owner", "chronos", "!", "-d", "127.0.0.1", "-j", "REJECT"}

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
