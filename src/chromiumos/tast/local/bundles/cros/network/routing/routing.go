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
	"chromiumos/tast/local/bundles/cros/network/ifaceaddrs"
	"chromiumos/tast/local/bundles/cros/network/virtualnet"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/env"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/subnet"
	localping "chromiumos/tast/local/network/ping"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

// testEnv contains all the varaibles used in a routing test. In each routing
// test, two networks (interfaces managed by shill) will be used: the base
// network and the test network. The base network has a fixed configuration, and
// it is mainly for isolating the physical networks (it always has a higher
// priority than the physical Ethernet) and used in some verifications. The test
// network is configured according to the needs in a test and used to simulate
// different network environment.
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
// BasePriority.
const (
	HighPriority = 3
	BasePriority = 2
	LowPriority  = 1
)

// Suffixes used in the name of routing tests.
const (
	BaseSuffix = "b"
	TestSuffix = "t"
)

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
	// profile will be undo:
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

// WaitForServiceDualStackConnected waits for a shill Service becoming connected
// for both IPv4 and IPv6. Note that the state a Service will become Connected
// (or Online) as long as either family is connected, and thus this function is
// helpful when we want layer 3 connectivity on both family. Currently this is
// implemented by checking if the corresponding inteface has both v4 and v6
// addresses. This implementation may not be reliable depends on how we define
// "dual-stack connected", since several parts (e.g., ip rules, routes, iptables
// rules) are involved in the layer 3 setup on DUT, it may be complicated and
// tricky to check them all.
func (e *testEnv) WaitForServiceDualStackConnected(ctx context.Context, svc *shill.Service) error {
	device, err := svc.GetDevice(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to get associated device on service %s", svc.String())
	}

	props, err := device.GetProperties(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to get properties on device %s", device.String())
	}
	ifname, err := props.GetString(shillconst.DevicePropertyInterface)
	if err != nil {
		return errors.Wrapf(err, "failed to get interface name on device %s", device.String())
	}

	if err := testing.Poll(ctx, func(context.Context) error {
		addrs, err := ifaceaddrs.ReadFromInterface(ifname)
		if err != nil {
			return errors.Wrapf(err, "failed to get addrs on %s", ifname)
		}
		if addrs.IPv4Addr == nil || len(addrs.IPv6Addrs) == 0 {
			return errors.Errorf("no IPv4 addr or IPv6 addr on the interface, current addrs: %v", addrs)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrapf(err, "failed to wait for IP addrs on interface %s", ifname)
	}

	return nil
}

// TearDown tears down the routing test environment.
func (e *testEnv) TearDown(ctx context.Context) error {
	var lastErr error
	updateLastErrAndLog := func(err error) {
		lastErr = err
		testing.ContextLog(ctx, "Cleanup failed: ", lastErr)
	}

	if e.BaseRouter != nil {
		if err := e.BaseRouter.Cleanup(ctx); err != nil {
			updateLastErrAndLog(errors.Wrap(err, "failed to cleanup base router"))
		}
	}
	if e.BaseServer != nil {
		if err := e.BaseServer.Cleanup(ctx); err != nil {
			updateLastErrAndLog(errors.Wrap(err, "failed to cleanup base server"))
		}
	}

	if e.TestRouter != nil {
		if err := e.TestRouter.Cleanup(ctx); err != nil {
			updateLastErrAndLog(errors.Wrap(err, "failed to cleanup test router"))
		}
	}
	if e.TestServer != nil {
		if err := e.TestServer.Cleanup(ctx); err != nil {
			updateLastErrAndLog(errors.Wrap(err, "failed to cleanup test server"))
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
	res, err := pr.Ping(ctx, addr, ping.Count(3), ping.User(user))
	if err != nil {
		return err
	}
	if res.Received == 0 {
		return errors.New("no response received")
	}
	return nil
}

// ExpectPingSuccessWithTimeout keeps pinging |addr| as |user| with |timeout|,
// and returns nil if ping succeeds. Between two ping attempts, icmp entries in
// conntrack will be deleted to avoid affection by connection pinning.
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
	res, err := pr.Ping(ctx, addr, ping.Count(1), ping.User(user))
	if err != nil {
		// An error definitely means a ping failure.
		return nil
	}
	if res.Received != 0 {
		return errors.New("received ping reply but not expected")
	}
	return nil
}

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
