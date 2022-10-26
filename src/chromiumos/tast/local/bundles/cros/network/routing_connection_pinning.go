// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"net"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/network/routing"
	"chromiumos/tast/local/network/virtualnet"
	"chromiumos/tast/local/network/virtualnet/l4server"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RoutingConnectionPinning,
		Desc:         "Verify the connection pinning functionality in routing",
		Contacts:     []string{"jiejiang@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		LacrosStatus: testing.LacrosVariantUnneeded,
	})
}

// RoutingConnectionPinning verifies the connection pinning functionality in
// routing. In detail, this test sets up the base network as the low-priority
// network and creates several TCP and UDP connections on this network. These
// connections should still work when a new high-priority network is up later.
// Note that this test only verifies the TCP/UDP connections for the root user.
func RoutingConnectionPinning(ctx context.Context, s *testing.State) {
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

	// Message to be sent via TCP/UDP connections in this test.
	const msg = "abcdefg"
	const msgLen = len(msg)

	var conns []net.Conn

	ipAddrs, err := testEnv.BaseServer.WaitForVethInAddrs(ctx, true /*ipv4*/, true /*ipv6*/)
	if err != nil {
		s.Fatal("Failed to get IP addrs from the base server: ", err)
	}

	for _, serverConf := range []struct {
		fam  l4server.Family
		port int
	}{
		{l4server.TCP4, 10000},
		{l4server.TCP6, 10001},
		{l4server.UDP4, 10002},
		{l4server.UDP6, 10003},
	} {
		server := l4server.New(serverConf.fam, serverConf.port, msgLen, l4server.Reflector())
		if err := testEnv.BaseServer.StartServer(ctx, server.String(), server); err != nil {
			s.Fatalf("Failed to start %s server: %v", server.String(), err)
		}

		var addr string
		switch serverConf.fam {
		case l4server.TCP4, l4server.UDP4:
			addr = fmt.Sprintf("%s:%d", ipAddrs.IPv4Addr, serverConf.port)
		case l4server.TCP6, l4server.UDP6:
			// ipAddrs.IPv6Addrs[0] must be valid since we use WaitForVethInAddrs() above to get ipAddrs.
			addr = fmt.Sprintf("[%s]:%d", ipAddrs.IPv6Addrs[0], serverConf.port)
		}

		conn, err := net.Dial(serverConf.fam.String(), addr)
		if err != nil {
			s.Fatalf("Failed to create %s connection: %v", server, err)
		}

		conns = append(conns, conn)
	}

	verifyConns := func() error {
		for _, conn := range conns {
			if _, err = conn.Write([]byte(msg)); err != nil {
				return errors.Wrapf(err, "failed to write msg to %s", conn.RemoteAddr())
			}

			in := make([]byte, msgLen)
			if _, err = conn.Read(in); err != nil {
				return errors.Wrapf(err, "failed to read msg from %s", conn.RemoteAddr())
			}

			inStr := string(in)
			if inStr != msg {
				return errors.Errorf("msg does not match for %s: got %s, want %s", conn.RemoteAddr(), inStr, msg)
			}
		}
		return nil
	}

	if err := verifyConns(); err != nil {
		s.Fatal("Failed to verify connections before creating high-priority network: ", err)
	}

	testNetworkOpts := virtualnet.EnvOptions{
		Priority:   routing.HighPriority,
		NameSuffix: routing.TestSuffix,
		EnableDHCP: true,
		RAServer:   true,
	}
	if err := testEnv.CreateNetworkEnvForTest(ctx, testNetworkOpts); err != nil {
		s.Fatal("Failed to create network environment for test: ", err)
	}
	if err := testEnv.WaitForServiceOnline(ctx, testEnv.TestService); err != nil {
		s.Fatal("Failed to wait for service in test online: ", err)
	}
	if errs := testEnv.VerifyTestNetwork(ctx, routing.VerifyOptions{
		IPv4:      true,
		IPv6:      true,
		IsPrimary: true,
		Timeout:   30 * time.Second,
	}); len(errs) != 0 {
		for _, err := range errs {
			s.Error("Failed to verify test network: ", err)
		}
	}

	if err := verifyConns(); err != nil {
		s.Fatal("Failed to verify connections after creating high-priority network: ", err)
	}
}
