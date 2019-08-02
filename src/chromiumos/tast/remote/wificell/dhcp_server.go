// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Runs a DHCP server on a test AP.
type DHCPServer struct {
	host     *dut.DUT // e.g., test AP
	iface    string
	confPath string
	pid      int
	ipIndex  int
}

const (
	dhcpLeasePath = "/tmp/dhcpd.leases"
	// TODO: parameterize for multiple subnets.
	ipIndex = 0
)

func NewDHCPServer(r *Router, iface string) *DHCPServer {
	return &DHCPServer{
		host:     r.host,
		iface:    iface,
		confPath: fmt.Sprintf("/tmp/dhcpd.%s.conf", iface),
		ipIndex:  ipIndex,
	}
}

func (d *DHCPServer) Start(ctx context.Context) error {
	conf := fmt.Sprintf(strings.Join([]string{
		"port=0", // Disables DNS server.
		"bind-interfaces",
		"log-dhcp",
		"dhcp-range=192.168.%[1]d.1,192.168.%[1]d.128",
		"interface=%s",
		"dhcp-leasefile=%s",
	}, "\n"), d.ipIndex, d.iface, dhcpLeasePath)

	if err := d.configureIP(ctx); err != nil {
		return errors.Wrap(err, "failed to configure IP")
	}

	testing.ContextLog(ctx, "Starting dnsmasq")
	cmd := fmt.Sprintf("cat > %s <<EOF\n%s\nEOF\n", d.confPath, conf)
	cmd = cmd + fmt.Sprintf("dnsmasq --conf-file=%s --no-daemon >/dev/null 2>&1 & echo $!", d.confPath)
	out, err := d.host.Run(ctx, cmd)
	if err != nil {
		testing.ContextLog(ctx, string(out))
		return errors.Wrap(err, "hostapd failure")
	}
	d.pid, err = strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		testing.ContextLog(ctx, string(out))
		return errors.Wrap(err, "parsing hostapd PID")
	}

	return nil
}

func (d *DHCPServer) Stop(ctx context.Context) error {
	if !d.IsRunning(ctx) {
		return nil
	}
	if _, err := d.host.Run(ctx, fmt.Sprintf("kill %d\n", d.pid)); err != nil {
		return errors.Wrapf(err, "killing PID %d failed", d.pid)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if !d.IsRunning(ctx) {
			return nil
		}
		return errors.New(fmt.Sprintf("hostapd process %d still running", d.pid))
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrapf(err, "hostapd PID %d didn't stop", d.pid)
	}

	d.pid = 0
	return nil
}

func (d *DHCPServer) IsRunning(ctx context.Context) bool {
	if d.pid == 0 {
		return false
	}
	if _, err := d.host.Run(ctx, fmt.Sprintf("kill -0 %d", d.pid)); err != nil {
		return false
	}
	return true
}

func (d *DHCPServer) configureIP(ctx context.Context) error {
	if out, err := d.host.Run(ctx, fmt.Sprintf("ip addr flush %s", d.iface)); err != nil {
		return errors.Wrapf(err, "failed to flush addresses: %s", string(out))
	}
	if out, err := d.host.Run(ctx, fmt.Sprintf("ip addr add 192.168.%d.254/24 broadcast 192.168.%d.255 dev %s", d.ipIndex, d.ipIndex, d.iface)); err != nil {
		return errors.Wrapf(err, "failed to add address: %s", string(out))
	}
	return nil
}
