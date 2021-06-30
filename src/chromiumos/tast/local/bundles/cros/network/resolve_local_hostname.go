// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"strings"

	"chromiumos/tast/common/testexec"
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
	out, err := testexec.CommandContext(ctx, "avahi-resolve-address", "127.0.0.1").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Log("output: ", string(out))
		s.Fatal("avahi-resolve-address failed: ", err)
	}
	parts := strings.Fields(string(out))
	if len(parts) != 2 {
		s.Fatal("Invalid output: ", parts)
	}
	addr := parts[1]
	if len(addr) < 7 || addr[len(addr)-6:] != ".local" {
		s.Fatal("Invalid address: ", addr)
	}
	if err := testexec.CommandContext(ctx, "gethostip", addr).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("gethostip failed: ", err)
	}
}
