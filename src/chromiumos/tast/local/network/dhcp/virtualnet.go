// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dnsmasq provides the utils to run the dnsmasq server inside a
// virtualnet.Env, which will be used to provide the functionality of a DHCP
// server.

package dhcp

import (
	"context"
	"net"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/network/virtualnet/env"
	"chromiumos/tast/testing"
)

type VerifyFunc func() error

func StartTestServerInEnv(ctx context.Context, env *env.Env, subnet *net.IPNet, gateway net.IP, rules []HandlingRule) (VerifyFunc, error) {
	const (
		serverPort = 67
		clientPort = 68
	)

	listenAddr := net.IPv4(0, 0, 0, 0)
	broadcast := net.IPv4(255, 255, 255, 255)

	s := newDHCPTestServer(env.VethInName, listenAddr, broadcast, serverPort, clientPort)
	ctx, cancel := context.WithCancel(ctx)

	ec := make(chan error)

	go func() {
		cleanup, err := env.EnterNetNS(ctx)
		if err != nil {
			testing.ContextLog(ctx, err)
			ec <- errors.Wrapf(err, "failed to enter ns %s", env.NetNSName)
			return
		}
		defer cleanup()
		if err := s.setupAndBindSocket(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to run loop: ", err)
			ec <- err
			return
		}
		defer s.conn.Close()
		err = s.runLoop(ctx, rules)
		testing.ContextLog(ctx, "Failed to run loop: ", err)
		ec <- err
	}()

	return func() error {
		cancel()
		return <-ec
	}, nil
}

func GetBaseOptionMap(subnet *net.IPNet) OptionMap {
	subnetIP := subnet.IP.To4()
	gatewayIP := net.IPv4(subnetIP[0], subnetIP[1], subnetIP[2], 1)
	intendedIP := net.IPv4(subnetIP[0], subnetIP[1], subnetIP[2], 2)
	mask := net.IPv4(255, 255, 255, 0)

	return OptionMap{
		serverID:    gatewayIP.String(),
		subnetMask:  mask.String(),
		ipLeaseTime: uint32(86400),
		requestedIP: intendedIP.String(),
	}
}
