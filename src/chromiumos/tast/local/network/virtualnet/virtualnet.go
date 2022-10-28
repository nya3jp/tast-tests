// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package virtualnet

import (
	"context"
	"encoding/hex"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/network/virtualnet/certs"
	"chromiumos/tast/local/network/virtualnet/dnsmasq"
	"chromiumos/tast/local/network/virtualnet/env"
	"chromiumos/tast/local/network/virtualnet/httpserver"
	"chromiumos/tast/local/network/virtualnet/radvd"
	"chromiumos/tast/local/network/virtualnet/subnet"
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
	// EnableDNS enables the DNS functionality. It only support resolving ResolvedHost
	// to a single IP address ResolveHostToIP provided during configuration.
	EnableDNS bool
	// IPv4DNSServers specifies the IPv4 DNS servers to be advertised by the router (dnsmasq).
	IPv4DNSServers []string
	// RAServer enables the RA server in the Env. IPv6 addresses can be obtained
	// on the interface by SLAAC.
	RAServer bool
	// HTTPServerResponseHandler is the handler function for the HTTP server
	// to customize how the server should respond to requests. If the handler is
	// set, then this enables the HTTP server in the Env.
	HTTPServerResponseHandler func(rw http.ResponseWriter, req *http.Request)
	// HTTPSServerResponseHandler is the handler function for the HTTPS server
	// to customize how the server should respond to requests. If the handler is
	// set, then this enables the HTTPS server in the Env.
	HTTPSServerResponseHandler func(rw http.ResponseWriter, req *http.Request)
	// HTTPS certs is the cert directory and a cert store to be used by HTTPS server.
	HTTPSCerts *certs.Certs
	// ResolvedHost is the hostname to force a specific IPv4 or IPv6 address.
	// When ResolvedHost is queried from dnsmasq, dnsmasq will respond with ResolveHostToIP.
	// If resolvedHost is not set, it matches any domain in dnsmasq configuration.
	ResolvedHost string
	// ResolveHostToIP is the IP address returned when doing a DNS query for ResolvedHost.
	// If resolveHostToIP is not set, resolvedHost is resolved to an IPv4 or IPv6 address
	// to the gateway of the virtualnet Env created with these EnvOptions.
	ResolveHostToIP net.IP
}

// CreateRouterEnv creates a virtualnet Env with the given options. On success,
// returns the corresponding shill Service and Env object. It's caller's
// responsibility to call Cleanup() on the returned Env object.
func CreateRouterEnv(ctx context.Context, m *shill.Manager, pool *subnet.Pool, opts EnvOptions) (*shill.Service, *env.Env, error) {
	router := env.New("router" + opts.NameSuffix)
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

	if err := startServersInRouter(ctx, router, pool, opts); err != nil {
		return nil, nil, err
	}

	svc, err := findEthernetServiceByIfName(ctx, m, router.VethOutName)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to find shill service for %s", router.VethOutName)
	}

	if err := svc.SetProperty(ctx, shillconst.ServicePropertyEphemeralPriority, opts.Priority); err != nil {
		return nil, nil, errors.Wrap(err, "failed to configure priority on interface")
	}

	// Do a reconnect to speed up the DHCP process, since when the veth device is
	// created, DHCP server may not be ready, and thus the first DHCP DISCOVER
	// packet will be lost, and result a long timeout to send the second one. Note
	// that this speed-up is in a best effort way since it cannot be guaranteed
	// that the DHCP server is ready this time.
	if opts.EnableDHCP {
		if err := svc.Disconnect(ctx); err != nil {
			return nil, nil, errors.Wrap(err, "failed to disconnect the veth service")
		}
		if err := svc.Connect(ctx); err != nil {
			return nil, nil, errors.Wrap(err, "failed to reconnect the veth service")
		}
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

	server := env.New("server" + opts.NameSuffix)
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

	if err := server.ConnectToRouterWithPool(ctx, router, pool); err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to connect server to router")
	}

	success = true
	return svc, router, server, nil
}

func startServersInRouter(ctx context.Context, router *env.Env, pool *subnet.Pool, opts EnvOptions) error {
	if opts.EnableDHCP {
		v4Subnet, err := pool.AllocNextIPv4Subnet()
		if err != nil {
			return errors.Wrap(err, "failed to allocate v4 subnet for DHCP")
		}
		dnsmasq := dnsmasq.New(
			dnsmasq.WithDHCPServer(v4Subnet),
			dnsmasq.WithDHCPNameServers(opts.IPv4DNSServers),
			dnsmasq.WithResolveHost(opts.ResolvedHost, opts.ResolveHostToIP),
		)
		if err := router.StartServer(ctx, "dnsmasq", dnsmasq); err != nil {
			return errors.Wrap(err, "failed to start dnsmasq")
		}
	}

	if opts.RAServer {
		v6Prefix, err := pool.AllocNextIPv6Subnet()
		if err != nil {
			return errors.Wrap(err, "failed to allocate v4 prefix for DHCP")
		}

		// Note that in the current implementation, shill requires an IPv6
		// connection has both address and DNS servers, and thus we need to provide
		// it here even though it is not reachable.
		const googleIPv6DNSServer = "2001:4860:4860::8888"
		radvd := radvd.New(v6Prefix, []string{googleIPv6DNSServer})
		if err := router.StartServer(ctx, "radvd", radvd); err != nil {
			return errors.Wrap(err, "failed to start radvd")
		}
	}

	if opts.HTTPServerResponseHandler != nil {
		httpserver := httpserver.New("80", opts.HTTPServerResponseHandler, nil)
		if err := router.StartServer(ctx, "httpserver", httpserver); err != nil {
			return errors.Wrap(err, "failed to start http server")
		}
	}

	if opts.HTTPSServerResponseHandler != nil {
		if opts.HTTPSCerts == nil {
			return errors.New("failed to create https server: empty certificate option")
		}
		httpsserver := httpserver.New("443", opts.HTTPSServerResponseHandler, opts.HTTPSCerts)
		if err := router.StartServer(ctx, "httpsserver", httpsserver); err != nil {
			return errors.Wrap(err, "failed to start https server")
		}
	}

	return nil
}

func findEthernetServiceByIfName(ctx context.Context, m *shill.Manager, ifName string) (*shill.Service, error) {
	testing.ContextLogf(ctx, "Waiting for device %s showing up", ifName)
	device, err := m.WaitForDeviceByName(ctx, ifName, 5*time.Second)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find the device with interface name %s", ifName)
	}
	testing.ContextLog(ctx, "Waiting for service being selected on device: ", ifName)
	service, err := device.WaitForSelectedService(ctx, 5*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the selected service")
	}
	return service, nil
}

type wifiEnv struct {
	// Service is the shill service corresponding to this AP.
	Service *shill.Service
	// Router is the Env which simulates the WiFi router. Servers can be ran on it.
	Router *env.Env

	hostapd *testexec.Cmd
}

// Cleanup shuts down all the servers.
func (e *wifiEnv) Cleanup(ctx context.Context) error {
	if e.Router != nil {
		if err := e.Router.Cleanup(ctx); err != nil {
			return err
		}
	}
	if e.hostapd != nil && e.hostapd.Process != nil {
		if err := e.hostapd.Kill(); err != nil {
			return errors.Wrap(err, "failed to kill hostapd process")
		}
		e.hostapd.Wait()
	}
	return nil
}

// CreateWifiRouterEnv create a virtualnet Env with the given options. Different
// from CreateRouterEnv, the created Env will be shown as a WiFi service in
// shill. apIf should be an interface created by the mac80211_hwsim kernel
// module, and this interface will be used to simulate the access point.
// shillSimulatedWiFi fixture can be used to create such interfaces. Note that
// the returned shill service will not be connected automatically.
func CreateWifiRouterEnv(ctx context.Context, apIf string, m *shill.Manager, pool *subnet.Pool, opts EnvOptions) (*wifiEnv, error) {
	wifi := &wifiEnv{}
	wifi.Router = env.NewHidden("wifi" + opts.NameSuffix)
	if err := wifi.Router.SetUp(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to set up the router env")
	}

	success := false
	cleanupCtx := ctx
	ctx, _ = ctxutil.Shorten(ctx, 10*time.Second)
	defer func(ctx context.Context) {
		if success {
			return
		}
		if err := wifi.Cleanup(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to clean up WiFi Env")
		}
	}(cleanupCtx)

	if err := startServersInRouter(ctx, wifi.Router, pool, opts); err != nil {
		return nil, err
	}

	ssid := "test-ap" + opts.NameSuffix
	svcProps := map[string]interface{}{
		shillconst.ServicePropertyType:        shillconst.TypeWifi,
		shillconst.ServicePropertyWiFiHexSSID: strings.ToUpper(hex.EncodeToString([]byte(ssid))),
	}

	// Check if shill knows this service at first. It can be leftover from the
	// previous tests.
	svc, err := m.FindMatchingService(ctx, svcProps)
	if err != nil && err.Error() != shillconst.ErrorMatchingServiceNotFound {
		return nil, errors.Wrap(err, "failed to check service existence before starting hostapd")
	}
	if svc != nil {
		if err := svc.Remove(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to remove the WiFi service")
		}
	}

	// Bridge interface name for connecting apIf and out interface of wifi.Router.
	// Name it with prefix "veth" to avoid shill managing it. The lifetime of this
	// interface will be managed by hostapd.
	const br = "veth-br0"

	// Start hostapd. We don't care about how the WiFi layer is working here so
	// just use the simplest setup.
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return nil, errors.New("failed to get ContextOutDir")
	}
	hostapdConf := strings.Join([]string{
		"interface=" + apIf,
		"ssid=" + ssid,
		"bridge=" + br,
	}, "\n")
	hostapdConfFile, err := os.CreateTemp(outDir, "hostapd*.conf")
	defer os.Remove(hostapdConfFile.Name())
	if _, err := hostapdConfFile.WriteString(hostapdConf); err != nil {
		return nil, errors.Wrap(err, "failed to create hostapd config file")
	}
	wifi.hostapd = testexec.CommandContext(ctx, "hostapd", hostapdConfFile.Name())
	if err := wifi.hostapd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start hostapd")
	}

	testing.ContextLogf(ctx, "Waiting for %s interface up", br)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := net.InterfaceByName(br); err != nil {
			return errors.Errorf("failed to get %s interface", br)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return nil, errors.Wrap(err, "failed to wait for bridge interface to appear")
	}

	// Bridge interfaces.
	for _, cmd := range [][]string{
		{"ip", "link", "set", br, "up"},
		{"ip", "link", "set", wifi.Router.VethOutName, "up"},
		{"brctl", "addif", br, wifi.Router.VethOutName},
	} {
		if err := testexec.CommandContext(ctx, cmd[0], cmd[1:]...).Run(testexec.DumpLogOnError); err != nil {
			return nil, errors.Wrap(err, "failed to bridge the interfaces")
		}
	}

	// This may take a long time.
	testing.ContextLogf(ctx, "Waiting for WiFi service for %s in shill", ssid)
	wifi.Service, err = m.WaitForServiceProperties(ctx, svcProps, 30*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "failed to wait for WiFi service to appear")
	}

	success = true
	testing.ContextLogf(ctx, "Virtual WiFi router env with SSID %s has been set up", ssid)
	return wifi, nil
}
