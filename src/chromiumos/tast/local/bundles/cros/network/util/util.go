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
// a log file with |filename| in the context OutDir, and returns the first error
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
		if _, err := f.WriteString("$ " + fullCmd + "\n"); err != nil {
			return errors.Wrap(err, "failed to write header for "+fullCmd)
		}

		// Writes command output on success, or failure message on failure.
		var contents string
		o, execErr := testexec.CommandContext(ctx, cmd, args...).Output()
		if execErr != nil {
			contents = "Execution failed " + execErr.Error() + "\n"
			execErr = errors.Wrap(execErr, "failed to execute "+fullCmd)
		} else {
			contents = string(o) + "\n"
		}

		if _, err := f.WriteString(contents); err != nil {
			return errors.Wrap(err, "failed to write contents for "+fullCmd)
		}

		return execErr
	}

	var errs []error

	// Dumps iptables.
	for _, iptablesCmd := range []string{"iptables", "ip6tables"} {
		for _, table := range []string{"filter", "nat", "mangle"} {
			if err := runCmdAndLog(iptablesCmd, "-L", "-x", "-v", "-t", table); err != nil {
				errs = append(errs, err)
			}
		}
	}

	// Dumps ip-rule.
	for _, family := range []string{"-4", "-6"} {
		if err := runCmdAndLog("ip", family, "rule"); err != nil {
			errs = append(errs, err)
		}
	}

	// Dumps ip-route.
	for _, family := range []string{"-4", "-6"} {
		if err := runCmdAndLog("ip", family, "route", "list", "table", "all"); err != nil {
			errs = append(errs, err)
		}
	}

	// Dumps conntrack.
	for _, family := range []string{"ipv4", "ipv6"} {
		if err := runCmdAndLog("conntrack", "-L", "-f", family); err != nil {
			errs = append(errs, err)
		}
	}

	// Dumps socket statistics.
	for _, family := range []string{"-4", "-6"} {
		if err := runCmdAndLog("ss", family, "-api"); err != nil {
			errs = append(errs, err)
		}
	}

	// Dump shill status.
	if err := runCmdAndLog("/usr/local/lib/flimflam/test/list-manager"); err != nil {
		errs = append(errs, err)
	}
	if err := runCmdAndLog("/usr/local/lib/flimflam/test/list-profiles"); err != nil {
		errs = append(errs, err)
	}
	if err := runCmdAndLog("/usr/local/lib/flimflam/test/list-devices"); err != nil {
		errs = append(errs, err)
	}
	if err := runCmdAndLog("/usr/local/lib/flimflam/test/list-connected-services"); err != nil {
		errs = append(errs, err)
	}

	for _, err := range errs {
		testing.ContextLog(ctx, "DumpNetworkInfo failed: ", err)
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
