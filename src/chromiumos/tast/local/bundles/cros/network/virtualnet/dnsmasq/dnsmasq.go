// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dnsmasq provides the utils to run the dnsmasq server inside a
// virtualnet.Env, which will be used to provide the functionality of a DHCP
// server.
package dnsmasq

import (
	"bytes"
	"context"
	"html/template"
	"net"
	"os"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/env"
)

const confTemplate = `
port=0 # disable dns
interface={{.ifname}}
dhcp-range={{.pool_start}},{{.pool_end}},{{.netmask}},12h
dhcp-option=option:netmask,{{.netmask}}
dhcp-option=option:router,{{.gateway}}
{{if .dns}}
dhcp-option=option:dns-server,{{.dns}}
{{end}}
`

// Paths in chroot.
const (
	dnsmasqPath   = "/usr/local/sbin/dnsmasq"
	confPath      = "/tmp/dnsmasq.conf"
	logPath       = "/tmp/dnsmasq.log"
	leaseFilePath = "/tmp/dnsmasq.leases"
)

type dnsmasq struct {
	env    *env.Env
	subnet *net.IPNet
	dns    []string
	cmd    *testexec.Cmd
}

// New creates a new dnsmasq object. Currently dnsmasq will only be used as a
// DHCP server daemon. The returned object can be passed to Env.StartServer(),
// its lifetime will be managed by the Env object.
func New(subnet *net.IPNet, dns []string) *dnsmasq {
	return &dnsmasq{subnet: subnet, dns: dns}
}

// Start starts the dnsmasq process.
func (d *dnsmasq) Start(ctx context.Context, env *env.Env) error {
	d.env = env

	ip := d.subnet.IP.To4()
	if ip == nil {
		return errors.Errorf("given subnet %s is not invalid", d.subnet.String())
	}

	gateway := net.IPv4(ip[0], ip[1], ip[2], 1)
	poolStart := net.IPv4(ip[0], ip[1], ip[2], 50)
	poolEnd := net.IPv4(ip[0], ip[1], ip[2], 150)

	// d.subnet.Mask is of type Mask and thus cannot be stringified as an IP.
	mask := net.IPv4(255, 255, 255, 255).Mask(d.subnet.Mask)

	// Prepare config file.
	confVals := map[string]string{
		"ifname":     d.env.VethInName,
		"pool_start": poolStart.String(),
		"pool_end":   poolEnd.String(),
		"netmask":    mask.String(),
		"gateway":    gateway.String(),
	}
	if len(d.dns) > 0 {
		confVals["dns"] = strings.Join(d.dns, ",")
	}
	b := &bytes.Buffer{}
	template.Must(template.New("").Parse(confTemplate)).Execute(b, confVals)
	if err := os.WriteFile(d.env.ChrootPath(confPath), []byte(b.String()), 0644); err != nil {
		return errors.Wrap(err, "failed to write config file")
	}

	// Install gateway address and routes.
	if err := d.env.ConfigureInterface(ctx, d.env.VethInName, gateway, d.subnet); err != nil {
		return errors.Wrap(err, "failed to configure IPv4 in netns")
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
