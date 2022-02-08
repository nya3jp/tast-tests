// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ResolveLocalHostname,
		Desc: "Verifies .local mDNS hostnames are resolved via avahi",
		Contacts: []string{
			"batrapranav@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		Attr: []string{
			"group:mainline",
			"informational",
		},
	})
}

func ResolveLocalHostname(ctx context.Context, s *testing.State) {
	// Get the mDNS hostname of the machine.
	out, err := testexec.CommandContext(ctx, "avahi-resolve-address", "127.0.0.1").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Log("output: ", string(out))
		s.Fatal("avahi-resolve-address failed: ", err)
	}
	parts := strings.Fields(string(out))
	if len(parts) != 2 {
		s.Fatal("Invalid output: ", parts)
	}
	hostname := parts[1]
	if len(hostname) < 7 || hostname[len(hostname)-6:] != ".local" {
		s.Fatal("Invalid hostname: ", hostname)
	}

	reader, err := syslog.NewReader(ctx)
	if err != nil {
		s.Fatal("syslog.NewReader failed: ", err)
	}
	defer reader.Close()

	// Resolve the mDNS hostname to an IP address via getent hosts. If avahi is not used
	// to resolve the hostname or avahi fails to resolve the hostname, gethostbyname2() will
	// fail and getent hosts will return an error code.
	s.Log("Resolving ", hostname)
	if err := testexec.CommandContext(ctx, "getent", "hosts", hostname).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("getent hosts failed: ", err)
	}
	// getent hosts is a small program which calls gethostbyname2().
	cmd := testexec.CommandContext(ctx, "getent", "hosts", "INVALID_ADDRESS.local")
	if code, ok := testexec.ExitCode(cmd.Run()); !ok || code != 2 {
		cmd.DumpLog(ctx)
		s.Fatal("getent hosts failed")
	}

	if _, err := reader.Wait(ctx, 10*time.Second, func(e *syslog.Entry) bool {
		return e.Program == "avahi-daemon" && e.Severity == "INFO" && strings.HasPrefix(e.Content, fmt.Sprintf("Resolved hostname %s ", hostname))
	}); err != nil {
		s.Fatal("Avahi log message not found: ", err)
	}
}
