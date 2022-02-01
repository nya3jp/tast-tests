// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ResolveLocalHostnameInvalidAddress,
		Desc: "Verifies avahi logs when attempts to resolve .local mDNS hostnames fail",
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

func ResolveLocalHostnameInvalidAddress(ctx context.Context, s *testing.State) {
	reader, err := syslog.NewReader(ctx)
	if err != nil {
		s.Fatal("syslog.NewReader failed: ", err)
	}
	defer reader.Close()
	// getent hosts is a small program which calls gethostbyname2().
	cmd := testexec.CommandContext(ctx, "getent", "hosts", "INVALID_ADDRESS.local")
	if code, ok := testexec.ExitCode(cmd.Run()); !ok || code != 2 {
		cmd.DumpLog(ctx)
		s.Fatal("getent hosts failed: ", err)
	}
	for {
		entry, err := reader.Read()
		if err != nil {
			s.Fatal("Avahi log message not found: ", err)
		}
		if entry.Program == "avahi-daemon" && entry.Severity == "WARNING" && entry.Content == "Failed to resolve hostname INVALID_ADDRESS.local (interface -1, protocol -1, flags 0, IPv4 yes, IPv6 no)" {
			break
		}
	}
}
