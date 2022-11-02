// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dhcp

import (
	"context"
	"net"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/network/virtualnet/env"
	"chromiumos/tast/testing"
)

type testFunc func(ctx context.Context) error

// RunTestWithEnv does the following steps:
//  1. start the DHCP test server with rules in env;
//  2. run testFunc to execute the test logic;
//  3. stop the DHCP test server and check the rules.
func RunTestWithEnv(ctx context.Context, env *env.Env, rules []HandlingRule, testFunc testFunc) (testErr, svrErr error) {
	const (
		serverPort = 67
		clientPort = 68
	)

	listenAddr := net.IPv4(0, 0, 0, 0)
	broadcast := net.IPv4(255, 255, 255, 255)

	ec := make(chan error)

	s := newTestServer(env.VethInName, listenAddr, broadcast, serverPort, clientPort)
	serverCtx, cancel := context.WithCancel(ctx)

	go func(ctx context.Context) {
		cleanup, err := env.EnterNetNS(ctx)
		if err != nil {
			ec <- errors.Wrapf(err, "failed to enter ns %s", env.NetNSName)
			return
		}
		defer cleanup()
		if err := s.setupAndBindSocket(ctx); err != nil {
			ec <- err
			return
		}
		defer s.conn.Close()
		testing.ContextLog(ctx, "Test DHCP server started")
		ec <- s.runLoop(ctx, rules)
		testing.ContextLog(ctx, "Test DHCP server stopped")
	}(serverCtx)

	// Run the test function at first, and then stop the server if it is still
	// running and get the result.
	testErr = testFunc(ctx)
	testing.ContextLog(ctx, "Verify rules for DHCP server")
	cancel()
	svrErr = <-ec
	return testErr, svrErr
}

// GenerateOptionMap returns a minimum workable OptionMap for DHCP. Lease time
// is 24 hours by default.
func GenerateOptionMap(gatewayIP, clientIP net.IP) OptionMap {
	return OptionMap{
		serverID:    gatewayIP.String(),
		subnetMask:  "255.255.255.0",
		ipLeaseTime: uint32(86400), // 86400 seconds
		requestedIP: clientIP.String(),
	}
}
