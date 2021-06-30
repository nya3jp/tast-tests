// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

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
			"group:paper-io",
			"paper-io_printing",
			"informational",
		},
	})
}

func ResolveLocalHostname(ctx context.Context, s *testing.State) {
	reader, err := syslog.NewReader(ctx)
	if err != nil {
		s.Fatal("syslog.NewReader failed: ", err)
	}
	defer reader.Close()
	// gethostip is a small program in syslinux which calls gethostbyname().
	cmd := testexec.CommandContext(ctx, "gethostip", "INVALID_ADDRESS.local")
	if out, _ := cmd.CombinedOutput(); string(out) != "INVALID_ADDRESS.local: Unknown host\n" {
		cmd.DumpLog(ctx)
		s.Fatal("gethostip failed: ", string(out))
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
