// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dumputil contains common utilities for dumping network state which
// are used by various networking tests.
package dumputil

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// DumpNetworkInfo dumps debug information about the current network status into
// a log file with |filename| in the context OutDir, and returns the last error
// if there is any.
func DumpNetworkInfo(ctx context.Context, filename string) error {
	// Creates a file for output.
	dir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("failed to get ContextOutDir")
	}

	f, err := os.OpenFile(filepath.Join(dir, filename), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	runCmdAndLog := func(cmd string, args ...string) error {
		fullCmd := cmd + " " + strings.Join(args, " ")
		header := "$ " + fullCmd + "\n"
		o, err := testexec.CommandContext(ctx, cmd, args...).Output()
		if err != nil {
			return errors.Wrap(err, "failed to execute "+fullCmd)
		}
		if _, err := f.WriteString(header + string(o) + "\n"); err != nil {
			return errors.Wrap(err, "failed to write contents for "+fullCmd)
		}
		return nil
	}

	var lastErr error

	// Dumps iptables.
	for _, iptablesCmd := range []string{"iptables", "ip6tables"} {
		for _, table := range []string{"filter", "nat", "mangle"} {
			if err := runCmdAndLog(iptablesCmd, "-L", "-x", "-v", "-t", table); err != nil {
				testing.ContextLog(ctx, "Failed to run and log iptables: ", err)
				lastErr = err
			}
		}
	}

	// Dumps ip-rule.
	for _, family := range []string{"-4", "-6"} {
		if err := runCmdAndLog("ip", family, "rule"); err != nil {
			testing.ContextLog(ctx, "Failed to run and log ip-rule: ", err)
			lastErr = err
		}
	}

	// Dumps ip-route.
	for _, family := range []string{"-4", "-6"} {
		if err := runCmdAndLog("ip", family, "route", "list", "table", "all"); err != nil {
			testing.ContextLog(ctx, "Failed to run and log ip-route: ", err)
			lastErr = err
		}
	}

	// Dumps conntrack.
	for _, family := range []string{"ipv4", "ipv6"} {
		if err := runCmdAndLog("conntrack", "-L", "-f", family); err != nil {
			testing.ContextLog(ctx, "Failed to run and log conntrack: ", err)
			lastErr = err
		}
	}

	// Dumps socket statistics.
	for _, family := range []string{"-4", "-6"} {
		if err := runCmdAndLog("ss", family, "-api"); err != nil {
			testing.ContextLog(ctx, "Failed to run and log ss: ", err)
			lastErr = err
		}
	}

	// Dump shill status.
	if err := runCmdAndLog("/usr/local/lib/flimflam/test/list-manager"); err != nil {
		testing.ContextLog(ctx, "Failed to run and log list-manager for shill: ", err)
		lastErr = err
	}
	if err := runCmdAndLog("/usr/local/lib/flimflam/test/list-profiles"); err != nil {
		testing.ContextLog(ctx, "Failed to run and log list-profiles for shill: ", err)
		lastErr = err
	}
	if err := runCmdAndLog("/usr/local/lib/flimflam/test/list-devices"); err != nil {
		testing.ContextLog(ctx, "Failed to run and log list-devices for shill: ", err)
		lastErr = err
	}
	if err := runCmdAndLog("/usr/local/lib/flimflam/test/list-connected-services"); err != nil {
		testing.ContextLog(ctx, "Failed to run and log list-connected-services for shill: ", err)
		lastErr = err
	}

	return lastErr
}
