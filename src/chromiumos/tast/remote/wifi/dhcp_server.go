// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"net"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	"chromiumos/tast/testing"
)

// Port from Brian's PoC crrev.com/c/1733740

// DHCPConfig is the config to setup dhcp server.
type DHCPConfig struct {
	IPIndex byte
}

// NewDHCPConfig creates config to spawn DHCPServer.
func NewDHCPConfig(ipIndex byte) *DHCPConfig {
	return &DHCPConfig{
		IPIndex: ipIndex,
	}
}

// DHCPServer is the object to contorl DHCP server on AP router.
type DHCPServer struct {
	host     *host.SSH // TODO(crbug.com/1019537): use a more suitable ssh object.
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
	if err := d.configureIP(ctx); err != nil {
		return errors.Wrap(err, "failed to configure IP")
	}

	dhcpIPStart, dhcpIPEnd := d.IPRange()
	conf := fmt.Sprintf(strings.Join([]string{
		"port=0", // Disables DNS server.
		"bind-interfaces",
		"log-dhcp",
		"dhcp-range=%s,%s",
		"interface=%s",
		"dhcp-leasefile=%s",
	}, "\n"), dhcpIPStart.String(), dhcpIPEnd.String(), d.iface, dhcpLeasePath)
	if err := writeToHost(ctx, d.host, d.confPath, []byte(conf)); err != nil {
		return errors.Wrap(err, "failed to write config")
	}

	testing.ContextLog(ctx, "Starting dnsmasq")
	cmd := d.host.Command("dnsmasq", fmt.Sprintf("--conf-file=%s", d.confPath), "--no-daemon")
	if err := cmd.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start dnsmasq")
	}
	d.cmd = cmd
	// TODO: maybe some wait until ready?

	return nil
}

// Stop the dhcp server.
func (d *DHCPServer) Stop(ctx context.Context) error {
	if d.cmd == nil {
		return errors.New("server not started")
	}

	d.cmd.Abort()
	// Skip the error in Wait as the process is aborted and always has error in wait.
	d.cmd.Wait(ctx)
	d.cmd = nil
	if err := d.host.Command("rm", d.confPath).Run(ctx); err != nil {
		return errors.Wrapf(err, "failed to remove config with err=%s", err.Error())
	}
	return nil
}

func (d *DHCPServer) getSubnetIP(suffix byte) net.IP {
	return net.IPv4(192, 168, d.conf.IPIndex, suffix)
}

// IPRange returns the starting and ending IP in DHCP range.
func (d *DHCPServer) IPRange() (net.IP, net.IP) {
	return d.getSubnetIP(1), d.getSubnetIP(128)
}

// ServerIP returns the IP used by DHCP server.
func (d *DHCPServer) ServerIP() net.IP {
	return d.getSubnetIP(254)
}

// Mask returns the mask of DHCP subnet.
func (d *DHCPServer) Mask() net.IPMask {
	return net.IPv4Mask(255, 255, 255, 0)
}

// BroadcastIP returns the broadcast IP of DHCP subnet.
func (d *DHCPServer) BroadcastIP() net.IP {
	return d.getSubnetIP(255)
}

func (d *DHCPServer) configureIP(ctx context.Context) error {
	if out, err := d.host.Command("ip", "addr", "flush", d.iface).CombinedOutput(ctx); err != nil {
		return errors.Wrapf(err, "failed to flush addresses: %s", string(out))
	}
	maskLen, _ := d.Mask().Size()
	cmd := d.host.Command("ip", "addr", "add", fmt.Sprintf("%s/%d", d.ServerIP().String(), maskLen),
		"broadcast", d.BroadcastIP().String(), "dev", d.iface)
	if out, err := cmd.CombinedOutput(ctx); err != nil {
		return errors.Wrapf(err, "failed to add address: %s", string(out))
	}
	return nil
}
