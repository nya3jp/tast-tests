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
	// EnableDNS enables the DNS server in the Env. All hosts can be resolved
	// to the provided IP address.
	EnableDNS bool
	//AddressToForceIP is the address to force a specifice ip. When DNS feature is enabled,
	//all domain name will be resolved to AddressToForceIP.
	AddressToForceIP string
	// RAServer enables the RA server in the Env. IPv6 addresses can be obtained
	// on the interface by SLAAC.
	RAServer bool
}

type dnsMasqParams struct {
	ifName string
	ip     []byte
}

var params dnsMasqParams

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

	if opts.EnableDHCP || opts.EnableDNS {
		v4Subnet, err := pool.AllocNextIPv4Subnet()
		params.ip = v4Subnet.IP.To4()
		if err != nil {
			return nil, errors.Wrap(err, "failed to allocate v4 subnet for DHCP and DNS")
		}

		var dns = []string{}
		if opts.EnableDNS {
			//The address 0.0.0.0 is taken to mean the address of the machine running dnsmasq.
			dns = append(dns, "0.0.0.0")
		}

		dnsmasq := dnsmasq.New(v4Subnet, opts.EnableDHCP, opts.EnableDNS, opts.AddressToForceIP, dns)
		if err := router.StartServer(ctx, "dnsmasq", dnsmasq); err != nil {
			return nil, errors.Wrap(err, "failed to start dnsmasq")
		}
		params.ifName = router.VethOutName
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

	if err := svc.SetProperty(ctx, shillconst.ServicePropertyPriority, opts.Priority); err != nil {
		return nil, errors.Wrap(err, "failed to configure priority on interface")
	}

	testing.ContextLogf(ctx, "virtualnet env %s started", router.NetNSName)
	success = true
	return router, nil
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

// GetDNSMasqParams can get the interface and subnet IP address of DNSMasq service.
func GetDNSMasqParams() (string, []byte) {
	return params.ifName, params.ip
}
