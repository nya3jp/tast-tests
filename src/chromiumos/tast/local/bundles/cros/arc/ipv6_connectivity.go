// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IPv6Connectivity,
		Desc:         "Checks IPv6 connectivity inside ARC",
		Contacts:     []string{"taoyl@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android", "chrome"},
		Pre:          arc.Booted(),
	})
}

func IPv6Connectivity(ctx context.Context, s *testing.State) {
	const (
		pingTimeout      = 10 * time.Second
		googleDNSIpv6    = "2001:4860:4860::8888"
		googleDotComIpv6 = "ipv6.google.com"
	)
	a := s.PreValue().(arc.PreData).ARC

	// Verify global IPv6 address is configured correctly.
	out, err := a.Command(ctx, "/system/bin/ip", "-6", "addr", "show", "scope", "global").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("Failed to get address information: %s", err)
	}
	if len(out) == 0 {
		s.Fatal("No global IPv6 address is configured")
	}

	// Verify connectivity to literal IPv6 address destination.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return a.Command(ctx, "/system/bin/ping6", "-c1", "-w1", googleDNSIpv6).Run()
	}, &testing.PollOptions{Timeout: pingTimeout}); err != nil {
		s.Errorf("Cannot ping %s: %s", googleDNSIpv6, err)
	}

	// Verify connectivity to IPv6-only host name.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return arc.BootstrapCommand(ctx, "/system/bin/ping6", "-c1", "-w1", googleDotComIpv6).Run()
	}, &testing.PollOptions{Timeout: pingTimeout}); err != nil {
		s.Errorf("Cannot ping %s: %s", googleDotComIpv6, err)
	}
}
