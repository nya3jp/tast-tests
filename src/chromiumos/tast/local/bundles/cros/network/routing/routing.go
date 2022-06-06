// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package routing contains the common utils shared by routing tests.
package routing

import (
	"context"
	"time"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/virtualnet"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/env"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/subnet"
	localping "chromiumos/tast/local/network/ping"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

type testEnv struct {
	popTestProfile func()

	Manager     *shill.Manager
	Pool        *subnet.Pool
	BaseService *shill.Service
	BaseRouter  *env.Env
	BaseServer  *env.Env
	TestService *shill.Service
	TestRouter  *env.Env
	TestServer  *env.Env
}

const (
	HighPriority = 3
	BasePriority = 2
	LowPriority  = 1
)

const (
	BasePrefix = "b"
	TestPrefix = "t"
)

func NewTestEnv() *testEnv {
	return &testEnv{Pool: subnet.NewPool()}
}

func (e *testEnv) SetUp(ctx context.Context) error {
	success := false
	defer func() {
		if success {
			return
		}
		if err := e.TearDown(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to tear down routing test env", err)
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
		NameSuffix: BasePrefix,
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

func (e *testEnv) CreateNetworkEnvForTest(ctx context.Context, opts virtualnet.EnvOptions) error {
	var err error
	e.TestService, e.TestRouter, e.TestServer, err = virtualnet.CreateRouterServerEnv(ctx, e.Manager, e.Pool, opts)
	if err != nil {
		return errors.Wrap(err, "failed to create test virtualnet env")
	}
	return nil
}

func (e *testEnv) WaitForServiceOnline(ctx context.Context, svc *shill.Service) error {
	// pw, err := svc.CreateWatcher(ctx)
	// if err != nil {
	// 	return errors.Wrap(err, "failed to create property watcher")
	// }
	// defer pw.Close(ctx)

	// props, err := svc.GetProperties(ctx)
	// if err != nil {
	// 	return errors.Wrap(err, "failed to read the current service properties")
	// }
	// state, err := props.GetString(shillconst.ServicePropertyState)
	// if err != nil {
	// 	return errors.Wrap(err, "failed to read the current state value")
	// }
	// if state == shillconst.ServiceStateOnline {
	// 	return nil
	// }

	// timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	// defer cancel()
	// if err := pw.Expect(timeoutCtx, shillconst.ServicePropertyState, shillconst.ServiceStateOnline); err != nil {
	// 	return errors.Wrap(err, "failed to wait for service state becoming Online")
	// }

	if err := svc.WaitForConnectedOrError(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for service connected")
	}

	props, err := svc.GetProperties(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to read the current service properties")
	}
	state, err := props.GetString(shillconst.ServicePropertyState)
	if err != nil {
		return errors.Wrap(err, "failed to read the current state value")
	}
	if state != shillconst.ServiceStateOnline {
		return errors.Wrap(err, "the current state is not online")
	}

	return nil
}

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
	// if err := testexec.CommandContext(ctx, "conntrack", "-D", "-f", "ipv4", "-p", "icmp").Run(); err != nil {
	// 	return errors.Wrap(err, "failed to delete IPv4 ICMP entries in conntrack table")
	// }
	// if err := testexec.CommandContext(ctx, "conntrack", "-D", "-f", "ipv6", "-p", "icmpv6").Run(); err != nil {
	// 	return errors.Wrap(err, "failed to delete IPv6 ICMPv6 entries in conntrack table")
	// }
	return nil
}
