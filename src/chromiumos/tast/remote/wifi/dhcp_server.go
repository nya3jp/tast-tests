// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	"chromiumos/tast/testing"
)

// Port from Brian's PoC crrev.com/c/1733740

// DHCPConfig is the config to setup dhcp server.
type DHCPConfig struct {
	IPIndex int
}

// NewDHCPConfig creates config to spawn DHCPServer.
func NewDHCPConfig(ipIndex int) *DHCPConfig {
	return &DHCPConfig{
		IPIndex: ipIndex,
	}
}

// DHCPServer is the object to contorl DHCP server on AP router.
type DHCPServer struct {
	host     *dut.DUT // e.g., test AP
	iface    string
	cmd      *host.Cmd
	conf     *DHCPConfig
	confPath string
}

const (
	dhcpLeasePath = "/tmp/dhcpd.leases"
)

// NewDHCPServer creates and runs a DHCP server on a test AP.
func NewDHCPServer(ctx context.Context, r *Router, iface string, conf *DHCPConfig) (*DHCPServer, error) {
	server := &DHCPServer{
		host:     r.host,
		iface:    iface,
		conf:     conf,
		confPath: fmt.Sprintf("/tmp/dhcpd.%s.conf", iface),
	}
	if err := server.start(ctx); err != nil {
		return nil, err
	}
	return server, nil
}

func (d *DHCPServer) start(ctx context.Context) error {
	conf := fmt.Sprintf(strings.Join([]string{
		"port=0", // Disables DNS server.
		"bind-interfaces",
		"log-dhcp",
		"dhcp-range=192.168.%[1]d.1,192.168.%[1]d.128",
		"interface=%s",
		"dhcp-leasefile=%s",
	}, "\n"), d.conf.IPIndex, d.iface, dhcpLeasePath)

	if err := d.configureIP(ctx); err != nil {
		return errors.Wrap(err, "failed to configure IP")
	}

	if err := writeToDUT(ctx, d.host, d.confPath, []byte(conf)); err != nil {
		return errors.Wrap(err, "failed to write config")
	}
	testing.ContextLog(ctx, "Starting dnsmasq")
	cmd := d.host.Command("dnsmasq", fmt.Sprintf("--conf-file=%s", d.confPath), "--no-daemon")
	if err := cmd.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start dnsmasq")
	}
	d.cmd = cmd

	return nil
}

// Stop the dhcp server.
func (d *DHCPServer) Stop(ctx context.Context) error {
	if d.cmd == nil {
		return errors.New("server not started")
	}
	d.cmd.Abort()
	// TODO: This always has error as it is aborted. Is this really meaningful?
	err := d.cmd.Wait(ctx)
	d.cmd = nil
	return err
}

func (d *DHCPServer) configureIP(ctx context.Context) error {
	if out, err := d.host.Command("ip", "addr", "flush", d.iface).CombinedOutput(ctx); err != nil {
		return errors.Wrapf(err, "failed to flush addresses: %s", string(out))
	}
	cmd := d.host.Command("ip", "addr", "add", fmt.Sprintf("192.168.%d.254/24", d.conf.IPIndex),
		"broadcast", fmt.Sprintf("192.168.%d.255", d.conf.IPIndex), "dev", d.iface)
	if out, err := cmd.CombinedOutput(ctx); err != nil {
		return errors.Wrapf(err, "failed to add address: %s", string(out))
	}
	return nil
}
