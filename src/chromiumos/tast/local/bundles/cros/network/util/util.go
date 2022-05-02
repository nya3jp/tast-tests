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
// a log file.
func DumpNetworkInfo(ctx context.Context) error {
	// Creates a file for output.
	dir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("failed to get ConextOutDir")
	}

	f, err := os.OpenFile(filepath.Join(dir, "network_dump.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	appendOutput := func(cmd string, args []string) {
		f.WriteString("$ " + cmd + " " + strings.Join(args, " ") + "\n")
		o, err := testexec.CommandContext(ctx, cmd, args...).Output()
		if err != nil {
			f.WriteString("Execution failed " + err.Error() + "\n")
		} else {
			f.WriteString(string(o) + "\n")
		}
		f.WriteString("\n")
	}

	// Dumps iptables.
	for _, iptablesCmd := range []string{"iptables", "ip6tables"} {
		for _, table := range []string{"filter", "nat", "mangle"} {
			appendOutput(iptablesCmd, []string{"-L", "-x", "-v", "-t", table})
		}
	}

	// Dumps ip-rule.
	for _, family := range []string{"-4", "-6"} {
		appendOutput("ip", []string{family, "rule"})
	}

	// Dumps ip-route.
	for _, family := range []string{"-4", "-6"} {
		appendOutput("ip", []string{family, "route", "list", "table", "all"})
	}

	// Dumps conntrack.
	for _, family := range []string{"ipv4", "ipv6"} {
		appendOutput("conntrack", []string{"-L", "-f", family})
	}

	// Dumps socket statistics.
	for _, family := range []string{"-4", "-6"} {
		appendOutput("ss", []string{family, "-ap"})
	}

	// Dump shill status.
	appendOutput("/usr/local/lib/flimflam/test/list-manager", []string{})
	appendOutput("/usr/local/lib/flimflam/test/list-profiles", []string{})
	appendOutput("/usr/local/lib/flimflam/test/list-devices", []string{})
	appendOutput("/usr/local/lib/flimflam/test/list-connected-services", []string{})

	return nil
}
