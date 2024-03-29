// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package routing contains the common utils shared by routing tests.
package routing

import (
	"context"
	"net"
	"os/exec"
	"time"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/network/dumputil"
	localping "chromiumos/tast/local/network/ping"
	"chromiumos/tast/local/network/virtualnet"
	"chromiumos/tast/local/network/virtualnet/env"
	"chromiumos/tast/local/network/virtualnet/subnet"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

// TestEnv contains all the variables used in a routing test. In each routing
// test, two networks (interfaces managed by shill) will be used: the base
// network and the test network. The base network has a fixed configuration, and
// it is mainly for isolating the test from physical networks (it always has a
// higher priority than any physical Ethernet) and used in some verifications.
// The test network is configured according to the needs in a test and used to
// simulate different network environments.
type TestEnv struct {
	// resetProfile indicates whether a test profile is pushed during setup.
	resetProfile   bool
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

const (
	// DHCPTimeout is the timeout value used in shill for DHCP lease acquisition.
	DHCPTimeout = 30 * time.Second
	// DHCPExtraTimeout is the DHCP timeout plus a small extra waiting time. This
	// can be used in the tests to guarantee that the timeout event has happened.
	DHCPExtraTimeout = DHCPTimeout + time.Second
)

// NewTestEnv creates a new TestEnv object for routing tests.
func NewTestEnv() *TestEnv {
	return &TestEnv{resetProfile: true, Pool: subnet.NewPool()}
}

// NewTestEnvWithoutResetProfile creates a new TestEnv object for routing tests,
// without resetting profile.
func NewTestEnvWithoutResetProfile() *TestEnv {
	return &TestEnv{resetProfile: false, Pool: subnet.NewPool()}
}

// SetUp configures shill and brings up the base network.
func (e *TestEnv) SetUp(ctx context.Context) error {
	// Reserve some time for cleanup on failures. This function will start some
	// processes which are supposed to be kept running so do not defer the
	// cancel() here.
	cleanupCtx := ctx
	ctx, _ = ctxutil.Shorten(ctx, 5*time.Second)

	success := false
	defer func(ctx context.Context) {
		if success {
			return
		}
		if err := e.TearDown(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to tear down routing test env: ", err)
		}
	}(cleanupCtx)

	var err error
	e.Manager, err = shill.NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create manager proxy")
	}

	if e.resetProfile {
		if err := e.Manager.PopAllUserProfiles(ctx); err != nil {
			return errors.Wrap(err, "failed to pop all user profile in shill")
		}

		// Push a test profile to guarantee that all changes related to shill
		// profile will be undone:
		// 1) after the test if the test ends normally;
		// 2) when restarting shill if a crash happened in the test.
		e.popTestProfile, err = e.Manager.PushTestProfile(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to push test profile in shill")
		}
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

	// Check the connectivity to the base network. Also make sure that routing is
	// setup properly for the base network.
	if errs := e.VerifyBaseNetwork(ctx, VerifyOptions{
		IPv4:      true,
		IPv6:      true,
		IsPrimary: true,
		Timeout:   30 * time.Second,
	}); len(errs) != 0 {
		for _, err := range errs {
			testing.ContextLog(ctx, "Failed to verify connectivity to the base network: ", err)
		}
		return errors.Wrap(errs[0], "failed to verify connectivity to the base network")
	}

	success = true
	return nil
}

// CreateNetworkEnvForTest creates the test network.
func (e *TestEnv) CreateNetworkEnvForTest(ctx context.Context, opts virtualnet.EnvOptions) error {
	var err error
	e.TestService, e.TestRouter, e.TestServer, err = virtualnet.CreateRouterServerEnv(ctx, e.Manager, e.Pool, opts)
	if err != nil {
		return errors.Wrap(err, "failed to create test virtualnet env")
	}
	return nil
}

// WaitForServiceOnline waits for a shill Service becoming Online.
func (e *TestEnv) WaitForServiceOnline(ctx context.Context, svc *shill.Service) error {
	return svc.WaitForProperty(ctx, shillconst.ServicePropertyState, shillconst.ServiceStateOnline, 10*time.Second)
}

// TearDown tears down the routing test environment.
func (e *TestEnv) TearDown(ctx context.Context) error {
	var lastErr error

	for _, netEnv := range []*env.Env{e.BaseRouter, e.BaseServer, e.TestRouter, e.TestServer} {
		if netEnv == nil {
			continue
		}
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

// expectPingSuccess pings |addr| as |user|, and returns nil if ping succeeded.
// Note that this function does not dump network info on failure. Use
// ExpectPingSuccessWithTimeout() instead when applicable.
func expectPingSuccess(ctx context.Context, addr, user string) error {
	if err := deletePingEntriesInConntrack(ctx); err != nil {
		return errors.Wrap(err, "failed to reset conntrack before pinging")
	}
	testing.ContextLogf(ctx, "Start to ping %s as user %s", addr, user)
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
// and returns nil if ping succeeds. |timeout|=0 means only ping once and
// return. Dumps the current network info on failure.
func ExpectPingSuccessWithTimeout(ctx context.Context, addr, user string, timeout time.Duration) (retErr error) {
	defer func() {
		if retErr != nil {
			dumpNetworkInfoAfterPingFailure(ctx)
		}
	}()

	if timeout == 0 {
		return expectPingSuccess(ctx, addr, user)
	}

	numRetries := 0
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		err := expectPingSuccess(ctx, addr, user)
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
// Dumps the current network info on failure.
func ExpectPingFailure(ctx context.Context, addr, user string) (retErr error) {
	defer func() {
		if retErr != nil {
			dumpNetworkInfoAfterPingFailure(ctx)
		}
	}()

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

func dumpNetworkInfoAfterPingFailure(ctx context.Context) {
	// Encode current time into filename to avoid confliction.
	filename := "network_dump_ping_" + time.Now().Format("030405000") + ".txt"
	if err := dumputil.DumpNetworkInfo(ctx, filename); err != nil {
		testing.ContextLog(ctx, "Failed to dump network info after a ping expectation failure: ", err)
	}
	testing.ContextLog(ctx, "Ping expectation failed. Dumped current network info into ", filename)
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

// VerifyOptions characterizes a network (a interface) on DUT. The routing
// semantics can be defined based on these options.
type VerifyOptions struct {
	// IPv4 indicates whether the network under test has IPv4.
	IPv4 bool
	// IPv6 indicates whether the network under test has IPv6.
	IPv6 bool
	// isPrimary indicates whether the network under test is a primary network in
	// shill or not.
	IsPrimary bool
	// IsHighestIPv6 indicates whether this network is the IPv6 network with the
	// highest priority, i.e., no network with higher priority than this one has
	// IPv6 connectivity. This field will be ignored when |IsPrimary| is set.
	// There is no |HighestIPv4| at the moment since the fallthrough behavior is
	// different currently.
	IsHighestIPv6 bool
	// Timeout means the network under test may be not fully connected now, but
	// that should happen in the given timeout. 0 means the network is already
	// connected.
	Timeout time.Duration
}

// VerifyBaseNetwork verifies the routing setup for the base network.
func (e *TestEnv) VerifyBaseNetwork(ctx context.Context, opts VerifyOptions) []error {
	return verifyNetworkConnectivity(ctx, e.BaseRouter, e.BaseServer, opts)
}

// VerifyTestNetwork verifies the routing setup for the test network.
func (e *TestEnv) VerifyTestNetwork(ctx context.Context, opts VerifyOptions) []error {
	return verifyNetworkConnectivity(ctx, e.TestRouter, e.TestServer, opts)
}

func verifyNetworkConnectivity(ctx context.Context, router, server *env.Env, opts VerifyOptions) []error {
	if !opts.IPv4 && !opts.IPv6 {
		return []error{errors.New("neither IPv4 nor IPv6 is set")}
	}
	if opts.IsPrimary {
		opts.IsHighestIPv6 = opts.IPv6
	}

	routerAddrs, err := router.WaitForVethInAddrs(ctx, opts.IPv4, opts.IPv6)
	if err != nil {
		return []error{errors.Wrapf(err, "failed to get inner addrs from router env %s", router.NetNSName)}
	}
	serverAddrs, err := server.WaitForVethInAddrs(ctx, opts.IPv4, opts.IPv6)
	if err != nil {
		return []error{errors.Wrapf(err, "failed to get inner addrs from server env %s", server.NetNSName)}
	}

	var errs []error

	// TODO(b/192436642): Add more verification items, e.g.:
	// - IP socket with bind interface;
	// - IP socket with bind src IP;
	// - Guest traffics;
	// - Other kinds of traffic which might be treated differently in routing
	// (tcp, udp, etc.).

	// Ping the router at first. This should work no matter whether the network is
	// primary or not. Also use the timeout in options to ping the router to
	// guarantee that the network is fully connected.
	var pingAddrs []net.IP
	if opts.IPv4 {
		pingAddrs = append(pingAddrs, routerAddrs.IPv4Addr)
	}
	// TODO(b/235050937): In the current implementation, IPv6 peer on local subnet
	// of the secondary network is not reachable if the primary network also has
	// IPv6. Change the expectation here when this bug is fixed.
	if opts.IPv6 && (opts.IsPrimary || opts.IsHighestIPv6) {
		pingAddrs = append(pingAddrs, routerAddrs.IPv6Addrs...)
	}
	for _, user := range []string{"root", "chronos"} {
		for _, ip := range pingAddrs {
			if err := ExpectPingSuccessWithTimeout(ctx, ip.String(), user, opts.Timeout); err != nil {
				errs = append(errs, errors.Wrapf(err, "local address %v is not reachable as user %s", ip, user))
			}
		}
	}

	// Check the connectivity to the remote server.
	var pingableAddrs, notPingableAddrs []net.IP
	if opts.IPv4 {
		if opts.IsPrimary {
			pingableAddrs = append(pingableAddrs, serverAddrs.IPv4Addr)
		} else {
			notPingableAddrs = append(notPingableAddrs, serverAddrs.IPv4Addr)
		}
	}
	if opts.IPv6 {
		// Currently we only have the fall-through case for IPv6 by default: the
		// connectivity to a remote server on a non-primary network does not rely on
		// the properties of the primary network (i.e., whether the primary network
		// provide connectivity only for IPv4 or not).
		if opts.IsPrimary || opts.IsHighestIPv6 {
			pingableAddrs = append(pingableAddrs, serverAddrs.IPv6Addrs...)
		} else {
			notPingableAddrs = append(notPingableAddrs, serverAddrs.IPv6Addrs...)
		}
	}

	for _, user := range []string{"root", "chronos"} {
		for _, ip := range pingableAddrs {
			// When local subnet is reachable, we suppose that the routing setup
			// should be finished, and thus ping should succeed directly here. But we
			// observed that in some cases for IPv6, this ping will use the link-local
			// address as its src ip, this seems like a transient issue during setup
			// and should not affect users in the real world, and thus we still use
			// opts.Timeout to retry ping here.
			if err := ExpectPingSuccessWithTimeout(ctx, ip.String(), user, opts.Timeout); err != nil {
				errs = append(errs, errors.Wrapf(err, "non-local address %v is not reachable as user %s", ip, user))
			}
		}
		for _, ip := range notPingableAddrs {

			if err := ExpectPingFailure(ctx, ip.String(), user); err != nil {
				errs = append(errs, errors.Wrapf(err, "non-local address %v on non-primary network is reachable as user %s", ip, user))
			}
		}
	}

	return errs
}
