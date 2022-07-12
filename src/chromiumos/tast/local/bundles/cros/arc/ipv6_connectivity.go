// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"net"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/network/routing"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IPv6Connectivity,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks IPv6 connectivity inside ARC",
		Contacts:     []string{"taoyl@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
		Fixture:      "arcBooted",
	})
}

func IPv6Connectivity(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	testEnv := routing.NewTestEnv()
	if err := testEnv.SetUp(ctx); err != nil {
		s.Fatal("Failed to set up routing test env: ", err)
	}
	defer func(ctx context.Context) {
		if err := testEnv.TearDown(ctx); err != nil {
			s.Error("Failed to tear down routing test env: ", err)
		}
	}(cleanupCtx)

	// Capture and log IP address information in ARC for debugging
	out, err := a.Command(ctx, "/system/bin/ip", "-6", "addr", "show", "scope", "global").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get address information: ", err)
	}
	if len(out) == 0 {
		s.Fatal("No global IPv6 address is configured")
	}
	testing.ContextLog(ctx, "ARC address information: ", string(out))

	// ping virtual router address and virtual server address from ARC
	routerAddrs, err := testEnv.BaseRouter.WaitForVethInAddrs(ctx, false, true)
	if err != nil {
		s.Fatal("Failed to get inner addrs from router env: ", err)
	}
	serverAddrs, err := testEnv.BaseServer.WaitForVethInAddrs(ctx, false, true)
	if err != nil {
		s.Fatal("Failed to get inner addrs from server env: ", err)
	}

	pingTimeout := 5 * time.Second
	var pingAddrs []net.IP
	pingAddrs = append(pingAddrs, routerAddrs.IPv6Addrs...)
	pingAddrs = append(pingAddrs, serverAddrs.IPv6Addrs...)
	for _, ip := range pingAddrs {
		testing.ContextLog(ctx, "Start to ping ", ip.String(), " in ARC")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			return a.Command(ctx, "/system/bin/ping6", "-c1", "-w1", ip.String()).Run()
		}, &testing.PollOptions{Timeout: pingTimeout}); err != nil {
			s.Fatalf("Cannot ping %s: %v", ip.String(), err)
		}
	}
}
