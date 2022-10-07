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
`

// Paths in chroot.
const (
	dnsmasqPath   = "/usr/sbin/dnsmasq"
	confPath      = "/tmp/dnsmasq.conf"
	logPath       = "/tmp/dnsmasq.log"
	leaseFilePath = "/tmp/dnsmasq.leases"
	dnsPort       = "53"
)

type dnsmasq struct {
	env *env.Env
	// subnet is the subnet that is used to define the gateway address and the range of DHCP addresses.
	subnet          *net.IPNet
	resolvedHost    string
	resolveHostToIP net.IP
	// dns defines the addresses for external DNS servers.
	dns        []string
	cmd        *testexec.Cmd
	enableDNS  bool
	enableDHCP bool
}

// New creates a new dnsmasq object. enableDNS enables the DNS functionality. It supports
// resolving resolvedHost to resolveHostToIP. If resolvedHost is not set, it matches any domain
// in dnsmasq configuration. If resolveHostToIP is not set, resolvedHost is resolved to the
// gateway. dns contains the DNS server list which will be broadcasted by DHCP. The returned
// object can be passed to Env.StartServer(), its lifetime will be managed by the Env object.
func New(enableDHCP, enableDNS bool, subnet *net.IPNet, dns []string, resolvedHost string, resolveHostToIP net.IP) *dnsmasq {
	return &dnsmasq{enableDHCP: enableDHCP, enableDNS: enableDNS, subnet: subnet, dns: dns, resolvedHost: resolvedHost, resolveHostToIP: resolveHostToIP}
}

// Start starts the dnsmasq process.
func (d *dnsmasq) Start(ctx context.Context, env *env.Env) error {
	d.env = env

	// Prepare config file.
	confVals := map[string]string{
		"ifname": d.env.VethInName,
		"port":   "0", // disable DNS
	}

	var gateway net.IP
	if d.enableDHCP {
		if d.subnet == nil {
			return errors.New("failed to start dnsmasq with DHCP: empty subnet")
		}

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
		if err := d.env.ConfigureInterface(ctx, d.env.VethInName, gateway, d.subnet); err != nil {
			return errors.Wrap(err, "failed to configure IPv4 in netns")
		}
	}

	if len(d.dns) > 0 {
		confVals["dns"] = strings.Join(d.dns, ",")
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
