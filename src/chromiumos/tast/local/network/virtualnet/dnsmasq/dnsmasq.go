// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dnsmasq provides the utils to run the dnsmasq server inside a
// virtualnet.Env, which will be used to provide the functionality of a DHCP
// server.
package dnsmasq

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net"
	"os"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/network/virtualnet/env"
)

const confTemplate = `
port={{.port}}
interface={{.ifname}}
{{if .subnet}}
dhcp-range={{.pool_start}},{{.pool_end}},{{.netmask}},12h
dhcp-option=option:netmask,{{.netmask}}
dhcp-option=option:router,{{.gateway}}
{{end}}
{{if .address}}
address={{.address}}
{{end}}
{{if .dns}}
dhcp-option=option:dns-server,{{.dns}}
{{end}}
{{if .classless_static_routes}}
dhcp-option=121,{{.classless_static_routes}}
{{end}}
{{if .wpad}}
dhcp-option=252,{{.wpad}}
{{end}}
`

// Paths in chroot.
const (
	dnsmasqPath   = "/usr/sbin/dnsmasq"
	confPath      = "/tmp/dnsmasq.conf"
	logPath       = "/tmp/dnsmasq.log"
	leaseFilePath = "/tmp/dnsmasq.leases"
	dnsPort       = "53"
)

// Route represents a classless static route.
type Route struct {
	Prefix  *net.IPNet
	Gateway net.IP
}

type dnsmasq struct {
	env *env.Env

	subnet                *net.IPNet
	classlessStaticRoutes []Route
	resolvedHost          string
	resolveHostToIP       net.IP
	dns                   []string
	enableDNS             bool
	ifname                string
	wpad                  string

	cmd *testexec.Cmd
}

// Option is a type of function to configure dnsmasq.
type Option = func(*dnsmasq)

// WithDHCPServer enables DHCPv4 server function in dnsmasq. subnet specifies
// the DHCP range, and the first address in subnet will be used as the gateway
// address. This option will be mapped to dhcp-range option in dnsmasq.
func WithDHCPServer(subnet *net.IPNet) Option {
	return func(d *dnsmasq) {
		d.subnet = subnet
	}
}

// WithDHCPNameServers configures the external DNS server lists which will be
// broadcast as a DHCP option.
func WithDHCPNameServers(dns []string) Option {
	return func(d *dnsmasq) {
		d.dns = dns
	}
}

// WithDHCPClasslessStaticRoutes configures the classless static routes in DHCP
// (option 121).
func WithDHCPClasslessStaticRoutes(routes []Route) Option {
	return func(d *dnsmasq) {
		d.classlessStaticRoutes = routes
	}
}

// WithDHCPWPAD configures the Web Proxy Auto-Discovery (WPAD) field in DHCP
// (option 252).
func WithDHCPWPAD(wpad string) Option {
	return func(d *dnsmasq) {
		d.wpad = wpad
	}
}

// WithInterface specifies the interface which dnsmasq should be running on. By
// default, the in-interface of the associated Env with be used.
func WithInterface(ifname string) Option {
	return func(d *dnsmasq) {
		d.ifname = ifname
	}
}

// WithResolveHost will enables the DNS server function in dnsmasq, which will
// response the DNS request to resolve host to ip. If host is empty, all hosts
// will be resolved to ip. If ip is nil, host will be resolved to the gateway
// address is DHCP server function is enabled, or loopback address (127.0.0.1)
// otherwise.
func WithResolveHost(host string, ip net.IP) Option {
	return func(d *dnsmasq) {
		d.enableDNS = true
		d.resolvedHost = host
		d.resolveHostToIP = ip
	}
}

// New creates a new dnsmasq object. The returned object can be passed to
// Env.StartServer(), its lifetime will be managed by the Env object.
func New(opts ...Option) *dnsmasq {
	d := &dnsmasq{}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Start starts the dnsmasq process.
func (d *dnsmasq) Start(ctx context.Context, env *env.Env) error {
	d.env = env

	if d.ifname == "" {
		d.ifname = d.env.VethInName
	}

	// Prepare config file.
	confVals := map[string]string{
		"ifname": d.ifname,
		"port":   "0", // disable DNS
	}

	var gateway net.IP
	if d.subnet != nil {
		ip := d.subnet.IP.To4()
		if ip == nil {
			return errors.Errorf("given subnet %s is not invalid", d.subnet.String())
		}

		gateway = net.IPv4(ip[0], ip[1], ip[2], 1)
		poolStart := net.IPv4(ip[0], ip[1], ip[2], 50)
		poolEnd := net.IPv4(ip[0], ip[1], ip[2], 150)

		// d.subnet.Mask is of type Mask and thus cannot be stringified as an IP.
		mask := net.IPv4(255, 255, 255, 255).Mask(d.subnet.Mask)

		confVals["subnet"] = d.subnet.String()
		confVals["pool_start"] = poolStart.String()
		confVals["pool_end"] = poolEnd.String()
		confVals["netmask"] = mask.String()
		confVals["gateway"] = gateway.String()

		// Install gateway address and routes.
		if err := d.env.ConfigureInterface(ctx, d.ifname, gateway, d.subnet); err != nil {
			return errors.Wrap(err, "failed to configure IPv4 in netns")
		}
	}

	if len(d.classlessStaticRoutes) > 0 {
		if d.subnet == nil {
			return errors.New("classless static route option is set but DHCP is not enabled")
		}
		var routes []string
		for _, r := range d.classlessStaticRoutes {
			prefix := r.Prefix.String()
			gateway := r.Gateway.String()
			routes = append(routes, prefix+","+gateway)
		}
		confVals["classless_static_routes"] = strings.Join(routes, ",")
	}

	if len(d.dns) > 0 {
		confVals["dns"] = strings.Join(d.dns, ",")
	}

	if len(d.wpad) > 0 {
		if d.subnet == nil {
			return errors.New("WPAD option is set but DHCP is not enabled")
		}
		confVals["wpad"] = d.wpad
	}

	var resolvedIP, resolvedHost string

	if d.resolveHostToIP != nil {
		resolvedIP = d.resolveHostToIP.String()
	} else if gateway != nil {
		resolvedIP = gateway.String()
	} else {
		// Defaults to localhost address.
		resolvedIP = "127.0.0.1"
	}

	if d.resolvedHost == "" {
		resolvedHost = "#" // '#' matches any domain in dnsmasq configuration.
	} else {
		resolvedHost = d.resolvedHost
	}

	if d.enableDNS {
		confVals["address"] = fmt.Sprintf("/%v/%v", resolvedHost, resolvedIP)
		confVals["port"] = dnsPort // enable DNS if needed for address forwarding
	}
	b := &bytes.Buffer{}
	template.Must(template.New("").Parse(confTemplate)).Execute(b, confVals)
	if err := os.WriteFile(d.env.ChrootPath(confPath), []byte(b.String()), 0644); err != nil {
		return errors.Wrap(err, "failed to write config file")
	}

	// Start the command.
	cmd := []string{
		dnsmasqPath,
		"--keep-in-foreground",
		"-C", confPath,
		"--log-facility=" + logPath,
		"--no-resolv",
		"--no-hosts",
		"--dhcp-leasefile=" + leaseFilePath,
	}
	d.cmd = d.env.CreateCommand(ctx, cmd...)

	if err := d.cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start dnsmasq daemon")
	}

	return nil
}

// Stop stops the dnsmasq process.
func (d *dnsmasq) Stop(ctx context.Context) error {
	if d.cmd == nil || d.cmd.Process == nil {
		return nil
	}
	if err := d.cmd.Kill(); err != nil {
		return errors.Wrap(err, "failed to kill dnsmasq processs")
	}
	d.cmd.Wait()
	d.cmd = nil
	return nil
}

// WriteLogs writes logs into |f|.
func (d *dnsmasq) WriteLogs(ctx context.Context, f *os.File) error {
	return d.env.ReadAndWriteLogIfExists(d.env.ChrootPath(logPath), f)
}

type lease struct {
	Hostname string // hostname claimed by the client
}

// GetLeases returns the leases issued by dnsmasq.
func (d *dnsmasq) GetLeases(ctx context.Context) ([]lease, error) {
	s, err := os.ReadFile(d.env.ChrootPath(leaseFilePath))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read leases file")
	}

	var leases []lease
	lines := strings.Split(string(s), "\n")
	for _, l := range lines {
		if len(l) == 0 {
			continue
		}
		// Example output:
		// `1666720187 72:f1:b4:0c:1f:7a 192.168.100.61 test-hostname 01:72:f1:b4:0c:1f:7a`
		items := strings.Split(l, " ")
		if len(items) != 5 {
			return nil, errors.Errorf("unexpected lease line: %s", l)
		}
		leases = append(leases, lease{Hostname: items[3]})
	}

	return leases, nil
}
