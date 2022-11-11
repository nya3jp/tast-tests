// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package routing

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/network/virtualnet"
	"chromiumos/tast/local/network/virtualnet/dnsmasq"
	"chromiumos/tast/local/network/virtualnet/env"
	"chromiumos/tast/local/network/virtualnet/radvd"
	"chromiumos/tast/local/network/virtualnet/subnet"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

// SimpleNetworkEnv setup a (DUT)-router-server virtualnet topology for testing.
// A DHCP server and an RA server will be started on router, and a DNS server will be
// started on server (all possible to disable through options). The DNS server provides
// resolution to two special domain name "v4.foo.bar" and "v6.foo.bar".
type SimpleNetworkEnv struct {
	hasIPv4    bool
	hasIPv6    bool
	hasIPv4DNS bool
	hasIPv6DNS bool

	popTestProfile func()
	// Manager wraps the Manager D-Bus object in shill.
	Manager *shill.Manager
	// Pool is the subnet pool used in this test.
	Pool *subnet.Pool
	// ShillService wraps the Service D-Bus object for the base network.
	ShillService *shill.Service
	// BaseRouter is the router env (the local subnet) for the base network.
	Router *env.Env
	// BaseServer is the server env (beyond local subnet) for the base network.
	Server *env.Env

	ServerAddress *env.IfaceAddrs
	RouterAddress *env.IfaceAddrs
}

// NewSimpleNetworkEnv creates a simple network test environment object.
func NewSimpleNetworkEnv(ipv4, ipv6, dnsv4, dnsv6 bool) *SimpleNetworkEnv {
	return &SimpleNetworkEnv{
		hasIPv4: ipv4, hasIPv6: ipv6, hasIPv4DNS: dnsv4, hasIPv6DNS: dnsv6,
		Pool: subnet.NewPool(),
	}
}

// SetUp configures shill and brings up the network.
func (e *SimpleNetworkEnv) SetUp(ctx context.Context) error {
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

	if err := e.Manager.PopAllUserProfiles(ctx); err != nil {
		return errors.Wrap(err, "failed to pop all user profile in shill")
	}
	e.popTestProfile, err = e.Manager.PushTestProfile(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to push test profile in shill")
	}

	testing.ContextLog(ctx, "Disabling portal detection on ethernet")
	if err := e.Manager.SetProperty(ctx, shillconst.ProfilePropertyCheckPortalList, "wifi,cellular"); err != nil {
		return errors.Wrap(err, "failed to disable portal detection on ethernet")
	}

	// Don't start dnsmasq and radvd here as we need to start them later knowing the server address.
	opts := virtualnet.EnvOptions{
		Priority:   BasePriority,
		NameSuffix: BaseSuffix,
		EnableDHCP: false,
		RAServer:   false,
		EnableDNS:  false,
	}
	e.ShillService, e.Router, e.Server, err = virtualnet.CreateRouterServerEnv(ctx, e.Manager, e.Pool, opts)
	if err != nil {
		return errors.Wrap(err, "failed to create virtualnet env")
	}
	e.ServerAddress, err = e.Server.WaitForVethInAddrs(ctx, false, true)
	if err != nil {
		return errors.Wrap(err, "failed to get inner addrs from server env")
	}
	if err := e.startServers(ctx); err != nil {
		return err
	}

	success = true
	return nil
}

func (e *SimpleNetworkEnv) startServers(ctx context.Context) error {
	// DHCP server on router
	if e.hasIPv4 {
		v4Subnet, err := e.Pool.AllocNextIPv4Subnet()
		if err != nil {
			return errors.Wrap(err, "failed to allocate v4 subnet for DHCP")
		}
		var dnsmasqOpts []dnsmasq.Option
		dnsmasqOpts = append(dnsmasqOpts, dnsmasq.WithDHCPServer(v4Subnet))
		if e.hasIPv4DNS {
			dnsmasqOpts = append(dnsmasqOpts, dnsmasq.WithDHCPNameServers([]string{e.ServerAddress.IPv4Addr.String()}))
		}
		dnsmasq := dnsmasq.New(dnsmasqOpts...)
		if err := e.Router.StartServer(ctx, "dnsmasq", dnsmasq); err != nil {
			return errors.Wrap(err, "failed to start dnsmasq on router")
		}
	}

	// RA server on router
	if e.hasIPv6 {
		v6Prefix, err := e.Pool.AllocNextIPv6Subnet()
		if err != nil {
			return errors.Wrap(err, "failed to allocate v6 prefix for RA server")
		}
		var rdnssServers []string
		if e.hasIPv6DNS {
			rdnssServers = append(rdnssServers, e.ServerAddress.IPv6Addrs[0].String())
		}
		radvd := radvd.New(v6Prefix, rdnssServers)
		if err := e.Router.StartServer(ctx, "radvd", radvd); err != nil {
			return errors.Wrap(err, "failed to start radvd on router")
		}
	}

	// DNS server on server
	dnsmasqOnServerOpts := []dnsmasq.Option{
		dnsmasq.WithResolveHost("v4.foo.bar", e.ServerAddress.IPv4Addr),
		dnsmasq.WithResolveHost("v6.foo.bar", e.ServerAddress.IPv6Addrs[0]),
	}
	dnsmasqOnServer := dnsmasq.New(dnsmasqOnServerOpts...)
	if err := e.Server.StartServer(ctx, "dnsmasq", dnsmasqOnServer); err != nil {
		return errors.Wrap(err, "failed to start dnsmasq on server")
	}

	return nil
}

// TearDown tears down the simple network test environment.
func (e *SimpleNetworkEnv) TearDown(ctx context.Context) error {
	var lastErr error

	for _, netEnv := range []*env.Env{e.Router, e.Server} {
		if netEnv == nil {
			continue
		}
		if err := netEnv.Cleanup(ctx); err != nil {
			lastErr = errors.Wrapf(err, "failed to cleanup %s", netEnv.NetNSName)
			testing.ContextLog(ctx, "Failed to cleanup TestEnv: ", lastErr)
		}
	}

	e.popTestProfile()
	return lastErr
}
