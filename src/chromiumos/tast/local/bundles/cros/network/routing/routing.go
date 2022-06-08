// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package routing contains the common utils shared by routing tests.
package routing

import (
	"context"
	"os/exec"
	"time"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/virtualnet"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/env"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/subnet"
	localping "chromiumos/tast/local/network/ping"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

// testEnv contains all the variables used in a routing test. In each routing
// test, two networks (interfaces managed by shill) will be used: the base
// network and the test network. The base network has a fixed configuration, and
// it is mainly for isolating the physical networks (it always has a higher
// priority than the physical Ethernet) and used in some verifications. The test
// network is configured according to the needs in a test and used to simulate
// different network environments.
type testEnv struct {
	popTestProfile func()

	// Manager wraps the Manager D-Bus object in shill.
	Manager *shill.Manager
	// Pool is the subnet pool used in this test.
	Pool *subnet.Pool
	// BaseService wraps the Service D-Bus object for the base network.
	BaseService *shill.Service
	// BaseRouter is the router env (the local subnet) for the base network.
	BaseRouter *env.Env
	// BaseServer is the server env (beyond local subnet) for the base network.
	BaseServer *env.Env
	// TestService wraps the Service D-Bus object for the test network.
	TestService *shill.Service
	// TestRouter is the router env (the local subnet) for the test network.
	TestRouter *env.Env
	// TestServer is the server env (beyond local subnet) for the test network.
	TestServer *env.Env
}

// Priorities used in the routing tests. The priority of the base network is
// BasePriority. This is mapped to EphemeralPriority property of a shill
// service, and affects how shill orders services.
const (
	HighPriority = 3
	BasePriority = 2
	LowPriority  = 1
)

// Suffixes used in the name of virtualnet.Env objects in routing tests.
const (
	BaseSuffix = "b"
	TestSuffix = "t"
)

// DHCPTimeout is the timeout value used in shill for DHCP lease acquisition.
const DHCPTimeout = 30 * time.Second

// NewTestEnv creates a new testEnv object for routing tests.
func NewTestEnv() *testEnv {
	return &testEnv{Pool: subnet.NewPool()}
}

// SetUp configures shill and brings up the base network.
func (e *testEnv) SetUp(ctx context.Context) error {
	success := false
	defer func() {
		if success {
			return
		}
		if err := e.TearDown(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to tear down routing test env: ", err)
		}
	}()

	var err error
	e.Manager, err = shill.NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create manager proxy")
	}

	// Push a test profile to guarantee that all changes related to shill
	// profile will be undone:
	// 1) after the test if the test ends normally;
	// 2) when restarting shill if a crash happened in the test.
	e.popTestProfile, err = e.Manager.PushTestProfile(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to push test profile in shill")
	}

	testing.ContextLog(ctx, "Disabling portal detection on ethernet")
	if err := e.Manager.SetProperty(ctx, shillconst.ProfilePropertyCheckPortalList, "wifi,cellular"); err != nil {
		return errors.Wrap(err, "failed to disable portal detection on ethernet")
	}

	opts := virtualnet.EnvOptions{
		Priority:   BasePriority,
		NameSuffix: BaseSuffix,
		EnableDHCP: true,
		RAServer:   true,
	}
	e.BaseService, e.BaseRouter, e.BaseServer, err = virtualnet.CreateRouterServerEnv(ctx, e.Manager, e.Pool, opts)
	if err != nil {
		return errors.Wrap(err, "failed to create base virtualnet env")
	}

	if err := e.WaitForServiceOnline(ctx, e.BaseService); err != nil {
		return errors.Wrap(err, "failed to wait for base service online")
	}

	success = true
	return nil
}

// CreateNetworkEnvForTest creates the test network.
func (e *testEnv) CreateNetworkEnvForTest(ctx context.Context, opts virtualnet.EnvOptions) error {
	var err error
	e.TestService, e.TestRouter, e.TestServer, err = virtualnet.CreateRouterServerEnv(ctx, e.Manager, e.Pool, opts)
	if err != nil {
		return errors.Wrap(err, "failed to create test virtualnet env")
	}
	return nil
}

// WaitForServiceOnline waits for a shill Service becoming Online.
func (e *testEnv) WaitForServiceOnline(ctx context.Context, svc *shill.Service) error {
	return svc.WaitForProperty(ctx, shillconst.ServicePropertyState, shillconst.ServiceStateOnline, 10*time.Second)
}

// TearDown tears down the routing test environment.
func (e *testEnv) TearDown(ctx context.Context) error {
	var lastErr error

	for _, netEnv := range []*env.Env{e.BaseRouter, e.BaseServer, e.TestRouter, e.TestServer} {
		if err := netEnv.Cleanup(ctx); err != nil {
			lastErr = errors.Wrapf(err, "failed to cleanup %s", netEnv.NetNSName)
			testing.ContextLog(ctx, "Failed to cleanup TestEnv: ", lastErr)
		}
	}

	if e.popTestProfile != nil {
		e.popTestProfile()
	}

	return lastErr
}

// ExpectPingSuccess pings |addr| as |user|, and returns nil if ping succeeded.
func ExpectPingSuccess(ctx context.Context, addr, user string) error {
	if err := deletePingEntriesInConntrack(ctx); err != nil {
		return errors.Wrap(err, "failed to reset conntrack before pinging")
	}
	testing.ContextLog(ctx, "Start to ping ", addr)
	pr := localping.NewLocalRunner()
	// Only ping once, continuous pings will be very likely to be affected by the
	// connection pinging so it does not make sense. In the routing tests, all the
	// ping targets are in the DUT, so use a small timeout value here.
	res, err := pr.Ping(ctx, addr, ping.Count(1), ping.User(user), ping.Timeout(2*time.Second))
	if err != nil {
		return err
	}
	if res.Received == 0 {
		return errors.New("no response received")
	}
	return nil
}

// ExpectPingSuccessWithTimeout keeps pinging |addr| as |user| with |timeout|,
// and returns nil if ping succeeds.
func ExpectPingSuccessWithTimeout(ctx context.Context, addr, user string, timeout time.Duration) error {
	numRetries := 0
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		err := ExpectPingSuccess(ctx, addr, user)
		if err == nil {
			return nil
		}
		numRetries++
		return errors.Wrapf(err, "failed to ping with %d retries", numRetries)
	}, &testing.PollOptions{Timeout: timeout, Interval: 500 * time.Millisecond}); err != nil {
		return errors.Wrap(err, "failed to ping with polling")
	}
	return nil
}

// ExpectPingFailure pings |addr| as |user|, and returns nil if ping failed.
func ExpectPingFailure(ctx context.Context, addr, user string) error {
	if err := deletePingEntriesInConntrack(ctx); err != nil {
		return errors.Wrap(err, "failed to reset conntrack before pinging")
	}
	testing.ContextLog(ctx, "Start to ping ", addr)
	pr := localping.NewLocalRunner()
	// Only ping once, continuous pings will be very likely to be affected by the
	// connection pinging so it does not make sense. In the routing tests, all the
	// ping targets are in the DUT, so use a small timeout value here.
	res, err := pr.Ping(ctx, addr, ping.Count(1), ping.User(user), ping.Timeout(2*time.Second))
	if err != nil {
		// An error definitely means a ping failure.
		return nil
	}
	if res.Received != 0 {
		return errors.New("received ping reply but not expected")
	}
	return nil
}

// deletePingEntriesInConntrack removes all the ping entries in conntrack table
// to avoid ping being affected by connection pinning. This should be called
// before each ping attempt, otherwise ping packets may go to a wrong interface
// due to connection pinning, which is not desired in routing tests.
func deletePingEntriesInConntrack(ctx context.Context) error {
	// `conntrack -D` will exit with 1 if no entry is deleted, so we ignore this
	// case when checking err.
	checkRunError := func(err error) error {
		if err == nil {
			return nil
		}
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				return nil
			}
		}
		return err
	}
	if err := testexec.CommandContext(ctx, "conntrack", "-D", "-f", "ipv4", "-p", "icmp").Run(); checkRunError(err) != nil {
		return errors.Wrap(err, "failed to delete IPv4 ICMP entries in conntrack table")
	}
	if err := testexec.CommandContext(ctx, "conntrack", "-D", "-f", "ipv6", "-p", "icmpv6").Run(); checkRunError(err) != nil {
		return errors.Wrap(err, "failed to delete IPv6 ICMPv6 entries in conntrack table")
	}
	return nil
}

// WaitDualStackIPsInEnv polls the IP addresses configured on the interface
// inside |e|, until there are both v4 and v6 addresses.
func WaitDualStackIPsInEnv(ctx context.Context, e *env.Env) (*env.IfaceAddrs, error) {
	var addrs *env.IfaceAddrs
	if err := testing.Poll(ctx, func(c context.Context) error {
		var err error
		addrs, err = e.GetVethInAddrs(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to get addrs of from env %s", e.NetNSName)
		}
		if addrs.IPv4Addr == nil || len(addrs.IPv6Addrs) == 0 {
			return errors.Errorf("the number of addrs is not expected %v", addrs)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return nil, errors.Wrapf(err, "failed to wait for addrs in env %s", e.NetNSName)
	}
	return addrs, nil
}
