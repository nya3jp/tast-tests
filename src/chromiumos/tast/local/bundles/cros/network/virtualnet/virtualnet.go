// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package virtualnet

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/dnsmasq"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/env"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/radvd"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/subnet"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

// EnvOptions contains the options that can be used to set up a virtualnet Env.
type EnvOptions struct {
	// Priority is the service priority in shill for the exposed interface. Please
	// refer to the EphemeralPriority property in service-api.txt for details.
	Priority int
	// NameSuffix is used to differentiate different virtualnet Env. Its length
	// cannot be longer than 3 due to IFNAMSIZ, and can be left empty if only one
	// Env is used.
	NameSuffix string
	// EnableDHCP enables the DHCP server in the Env. IPv4 address can be obtained
	// on the interface by DHCP.
	EnableDHCP bool
	// RAServer enables the RA server in the Env. IPv6 addresses can be obtained
	// on the interface by SLAAC.
	RAServer bool
}

// CreateRouterEnv creates a virtualnet Env with the given options. On success,
// returns the corresponding shill Service and Env object. It's caller's
// responsibility to call Cleanup() on the returned Env object.
func CreateRouterEnv(ctx context.Context, m *shill.Manager, pool *subnet.Pool, opts EnvOptions) (*shill.Service, *env.Env, error) {
	router, err := env.New("router" + opts.NameSuffix)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create router env")
	}

	if err := router.SetUp(ctx); err != nil {
		return nil, nil, errors.Wrap(err, "failed to set up the router env")
	}

	success := false
	defer func() {
		if success {
			return
		}
		if err := router.Cleanup(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to cleanup router env: ", err)
		}
	}()

	if opts.EnableDHCP {
		v4Subnet, err := pool.AllocNextIPv4Subnet()
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to allocate v4 subnet for DHCP")
		}
		dnsmasq := dnsmasq.New(v4Subnet, []string{})
		if err := router.StartServer(ctx, "dnsmasq", dnsmasq); err != nil {
			return nil, nil, errors.Wrap(err, "failed to start dnsmasq")
		}
	}

	if opts.RAServer {
		v6Prefix, err := pool.AllocNextIPv6Subnet()
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to allocate v4 prefix for DHCP")
		}

		// Note that in the current implementation, shill requires an IPv6
		// connection has both address and DNS servers, and thus we need to provide
		// it here even though it is not reachable.
		const googleIPv6DNSServer = "2001:4860:4860::8888"
		radvd := radvd.New(v6Prefix, []string{googleIPv6DNSServer})
		if err := router.StartServer(ctx, "radvd", radvd); err != nil {
			return nil, nil, errors.Wrap(err, "failed to start radvd")
		}
	}

	svc, err := findEthernetServiceByIfName(ctx, m, router.VethOutName)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to find shill service for %s", router.VethOutName)
	}

	if err := svc.SetProperty(ctx, shillconst.ServicePropertyEphemeralPriority, opts.Priority); err != nil {
		return nil, nil, errors.Wrap(err, "failed to configure priority on interface")
	}

	testing.ContextLogf(ctx, "virtualnet env %s started", router.NetNSName)
	success = true
	return svc, router, nil
}

// CreateRouterServerEnv creates two virtualnet Envs with the given options.
// This first one (router) simulates the first hop for Internet access which is
// connected to DUT directly, and the second one (server) simulates a server on
// the Internet. This setup is useful when we need to test something that cannot
// be done in local subnet, e.g., to test the default routes. On success, it's
// caller's responsibility to call Cleanup() on the returned Env objects.
func CreateRouterServerEnv(ctx context.Context, m *shill.Manager, pool *subnet.Pool, opts EnvOptions) (svc *shill.Service, routerEnv, serverEnv *env.Env, err error) {
	success := false

	svc, router, err := CreateRouterEnv(ctx, m, pool, opts)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to create router env")
	}
	defer func() {
		if success {
			return
		}
		if err := router.Cleanup(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to cleanup router env: ", err)
		}
	}()

	server, err := env.New("server" + opts.NameSuffix)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to create server env")
	}

	if err := server.SetUp(ctx); err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to set up server env")
	}
	defer func() {
		if success {
			return
		}
		if err := server.Cleanup(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to cleanup server env: ", err)
		}
	}()

	if err := server.ConnectToRouter(ctx, router, pool); err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to connect server to router")
	}

	success = true
	return svc, router, server, nil
}

func findEthernetServiceByIfName(ctx context.Context, m *shill.Manager, ifName string) (*shill.Service, error) {
	testing.ContextLogf(ctx, "Waiting for device %s showing up", ifName)
	device, err := m.WaitForDeviceByName(ctx, ifName, 5*time.Second)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find the device with interface name %s", ifName)
	}
	testing.ContextLog(ctx, "Waiting for service being selected on device: ", ifName)
	servicePath, err := device.WaitForSelectedService(ctx, 5*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the selected service path")
	}
	return shill.NewService(ctx, servicePath)
}
