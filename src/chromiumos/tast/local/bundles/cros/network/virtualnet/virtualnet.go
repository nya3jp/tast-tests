// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package virtualnet

import (
	"context"
	"fmt"
	"net"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/dnsmasq"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/env"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/httpserver"
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

// CaptivePortalOptions contains the options that can be used to set up captive
// portal Env.
type CaptivePortalOptions struct {
	// Priority is the service priority in shill for the exposed interface. Please
	// refer to the EphemeralPriority property in service-api.txt for details.
	Priority int
	// NameSuffix is used to differentiate different virtualnet Env. Its length
	// cannot be longer than 3 due to IFNAMSIZ, and can be left empty if only one
	// Env is used.
	NameSuffix string
	// AddressToForceIP is the address to force a specifice ip, which in the
	// captive portal case will be the ip address of the http server.
	AddressToForceIP string
}

// CreateRouterEnv creates a virtualnet Env with the given options. On success,
// it's caller's responsibility to call Cleanup() on the returned Env object.
func CreateRouterEnv(ctx context.Context, m *shill.Manager, pool *subnet.Pool, opts EnvOptions) (*env.Env, error) {
	router, err := env.New("router" + opts.NameSuffix)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create router env")
	}

	if err := router.SetUp(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to set up the router env")
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
			return nil, errors.Wrap(err, "failed to allocate v4 subnet for DHCP")
		}
		dnsmasq := dnsmasq.New(v4Subnet, []string{}, "")
		if err := router.StartServer(ctx, "dnsmasq", dnsmasq); err != nil {
			return nil, errors.Wrap(err, "failed to start dnsmasq")
		}
	}

	if opts.RAServer {
		v6Prefix, err := pool.AllocNextIPv6Subnet()
		if err != nil {
			return nil, errors.Wrap(err, "failed to allocate v4 prefix for DHCP")
		}

		// Note that in the current implementation, shill requires an IPv6
		// connection has both address and DNS servers, and thus we need to provide
		// it here even though it is not reachable.
		const googleIPv6DNSServer = "2001:4860:4860::8888"
		radvd := radvd.New(v6Prefix, []string{googleIPv6DNSServer})
		if err := router.StartServer(ctx, "radvd", radvd); err != nil {
			return nil, errors.Wrap(err, "failed to start radvd")
		}
	}

	svc, err := findEthernetServiceByIfName(ctx, m, router.VethOutName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find shill service for %s", router.VethOutName)
	}

	if err := svc.SetProperty(ctx, shillconst.ServicePropertyEphemeralPriority, opts.Priority); err != nil {
		return nil, errors.Wrap(err, "failed to configure priority on interface")
	}

	testing.ContextLogf(ctx, "virtualnet env %s started", router.NetNSName)
	success = true
	return router, nil
}

// CreateRouterServerEnv creates two virtualnet Envs with the given options.
// This first one (router) simulates the first hop for Internet access which is
// connected to DUT directly, and the second one (server) simulates a server on
// the Internet. This setup is useful when we need to test something that cannot
// be done in local subnet, e.g., to test the default routes. On success, it's
// caller's responsibility to call Cleanup() on the returned Env objects.
func CreateRouterServerEnv(ctx context.Context, m *shill.Manager, pool *subnet.Pool, opts EnvOptions) (routerEnv, serverEnv *env.Env, err error) {
	success := false

	router, err := CreateRouterEnv(ctx, m, pool, opts)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create router env")
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
		return nil, nil, errors.Wrap(err, "failed to create server env")
	}

	if err := server.SetUp(ctx); err != nil {
		return nil, nil, errors.Wrap(err, "failed to set up server env")
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
		return nil, nil, errors.Wrap(err, "failed to connect server to router")
	}

	success = true
	return router, server, nil
}

// CreateCaptivePortalEnv creates a captive portal Env with the given options. On success,
// it's caller's responsibility to call Cleanup() on the returned Env object.
func CreateCaptivePortalEnv(ctx context.Context, m *shill.Manager, pool *subnet.Pool, opts CaptivePortalOptions) (*env.Env, error) {
	portal, err := env.New("portal" + opts.NameSuffix)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create captive portal env")
	}

	if err := portal.SetUp(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to set up the captive portal env")
	}

	success := false
	defer func() {
		if success {
			return
		}
		if err := portal.Cleanup(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to cleanup captive portal env: ", err)
		}
	}()

	// Setup DNS subnet
	v4Subnet, err := pool.AllocNextIPv4Subnet()
	if err != nil {
		return nil, errors.Wrap(err, "failed to allocate v4 subnet for DHCP")
	}

	// Setup HTTP server subnet, IPv4 addresses, and routes
	ipv4Subnet, err := pool.AllocNextIPv4Subnet()
	if err != nil {
		return nil, errors.Wrap(err, "failed to allocate IPv4 subnet for connecting Envs")
	}
	ipv4Addr := ipv4Subnet.IP.To4()
	serverIPv4Addr := net.IPv4(ipv4Addr[0], ipv4Addr[1], ipv4Addr[2], 1)

	// Start DNS server
	dnsmasq := dnsmasq.New(v4Subnet, []string{}, fmt.Sprintf("/%v/%v", opts.AddressToForceIP, serverIPv4Addr.String()))
	if err := portal.StartServer(ctx, "dnsmasq", dnsmasq); err != nil {
		return nil, errors.Wrap(err, "failed to start dnsmasq")
	}

	// Setup and start HTTP server
	if err := portal.ConfigureInterface(ctx, portal.VethInName, serverIPv4Addr, ipv4Subnet); err != nil {
		return nil, errors.Wrapf(err, "failed to configure IPv4 on %s", portal.VethInName)
	}
	httpserver := httpserver.New(serverIPv4Addr.String(), "80")
	if err := portal.StartServer(ctx, "httpserver", httpserver); err != nil {
		return nil, errors.Wrap(err, "failed to start http server")
	}

	svc, err := findEthernetServiceByIfName(ctx, m, portal.VethOutName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find shill service for %s", portal.VethOutName)
	}

	if err := svc.SetProperty(ctx, shillconst.ServicePropertyEphemeralPriority, opts.Priority); err != nil {
		return nil, errors.Wrap(err, "failed to configure priority on interface")
	}

	testing.ContextLogf(ctx, "virtualnet env %s started", portal.NetNSName)
	success = true
	return portal, nil
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
