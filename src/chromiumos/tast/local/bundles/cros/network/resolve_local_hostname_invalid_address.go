// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

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
		s.Fatal("getent hosts failed")
	}

	// The log message is generated here:
	// https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/third_party/chromiumos-overlay/net-dns/avahi/files/avahi-0.7-cache-host-name-record-A.patch;drc=935c1f3e104123ff72cb16bc5faca58cfdfc5293;l=90
	const avahiInvalidAddrError = "Failed to resolve hostname INVALID_ADDRESS.local (interface -1, protocol -1, flags 0, IPv4 yes, IPv6 no)"

	if _, err := reader.Wait(ctx, 10*time.Second, func(e *syslog.Entry) bool {
		return e.Program == "avahi-daemon" && e.Severity == "WARNING" && e.Content == avahiInvalidAddrError
	}); err != nil {
		s.Fatal("Avahi log message not found: ", err)
	}
}
