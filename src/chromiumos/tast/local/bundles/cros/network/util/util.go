// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package util contains common utilities which are used by various networking
// tests.
package util

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
// a log file, and returns the last error if there is any.
func DumpNetworkInfo(ctx context.Context) error {
	// Creates a file for output.
	dir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("failed to get ContextOutDir")
	}

	f, err := os.OpenFile(filepath.Join(dir, "network_dump.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	var lastErr error
	writeFile := func(fullCmd, s string) {
		if _, err := f.WriteString(s); err != nil {
			lastErr = err
			testing.ContextLogf(ctx, "Failed to write to log file for %s: %v", fullCmd, err)
		}
	}

	runCmdAndLog := func(cmd string, args ...string) {
		fullCmd := cmd + " " + strings.Join(args, " ")
		writeFile(fullCmd, "$ "+fullCmd+"\n")
		o, err := testexec.CommandContext(ctx, cmd, args...).Output()
		if err != nil {
			lastErr = err
			writeFile(fullCmd, "Execution failed "+err.Error()+"\n")
		} else {
			writeFile(fullCmd, string(o)+"\n")
		}
		writeFile(fullCmd, "\n")
	}

	// Dumps iptables.
	for _, iptablesCmd := range []string{"iptables", "ip6tables"} {
		for _, table := range []string{"filter", "nat", "mangle"} {
			runCmdAndLog(iptablesCmd, "-L", "-x", "-v", "-t", table)
		}
	}

	// Dumps ip-rule.
	for _, family := range []string{"-4", "-6"} {
		runCmdAndLog("ip", family, "rule")
	}

	// Dumps ip-route.
	for _, family := range []string{"-4", "-6"} {
		runCmdAndLog("ip", family, "route", "list", "table", "all")
	}

	// Dumps conntrack.
	for _, family := range []string{"ipv4", "ipv6"} {
		runCmdAndLog("conntrack", "-L", "-f", family)
	}

	// Dumps socket statistics.
	for _, family := range []string{"-4", "-6"} {
		runCmdAndLog("ss", family, "-api")
	}

	// Dump shill status.
	runCmdAndLog("/usr/local/lib/flimflam/test/list-manager")
	runCmdAndLog("/usr/local/lib/flimflam/test/list-profiles")
	runCmdAndLog("/usr/local/lib/flimflam/test/list-devices")
	runCmdAndLog("/usr/local/lib/flimflam/test/list-connected-services")

	return lastErr
}
