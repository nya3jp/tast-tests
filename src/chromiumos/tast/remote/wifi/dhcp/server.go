// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dhcp provides utilities for controlling DHCP server.
package dhcp

import (
	"context"
	"fmt"
	"net"
	"path"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	"chromiumos/tast/remote/wifi/utils"
	"chromiumos/tast/testing"
)

// Ported from Brian's draft crrev.com/c/1733740.

const (
	dnsmasqCmd    = "dnsmasq"
	dhcpLeaseFile = "dhcpd.leases"
)

// Config is used to set up a DHCP server.
type Config struct {
	// IPIndex is used as part of the subnet 192.169.{IPIndex}.0/24 managed by the DHCP server.
	IPIndex byte
}

// NewConfig creates a Config to spawn DHCPServer.
func NewConfig(ipIndex byte) *Config {
	return &Config{
		IPIndex: ipIndex,
	}
}

// Server contorls a DHCP server on AP router.
type Server struct {
	host    *host.SSH // TODO(crbug.com/1019537): use a more suitable ssh object.
	iface   string
	cmd     *host.Cmd
	conf    *Config
	workDir string
}

// NewServer creates and runs a DHCP server on iface of the given host with settings specified in conf.
// workDir is the dir on host for the server to put temporary files.
func NewServer(ctx context.Context, host *host.SSH, iface string, workDir string, conf *Config) (*Server, error) {
	server := &Server{
		host:    host,
		iface:   iface,
		conf:    conf,
		workDir: workDir,
	}
	if err := server.start(ctx); err != nil {
		return nil, err
	}
	return server, nil
}

func (d *Server) confPath() string {
	return path.Join(d.workDir, fmt.Sprintf("dnsmasq-%s.conf", d.iface))
}

func (d *Server) leasePath() string {
	return path.Join(d.workDir, dhcpLeaseFile)
}

func (d *Server) stdoutFile() string {
	return fmt.Sprintf("dnsmasq-%s.stdout", d.iface)
}

func (d *Server) stderrFile() string {
	return fmt.Sprintf("dnsmasq-%s.stderr", d.iface)
}

func (d *Server) start(ctx context.Context) (err error) {
	// Cleanup on error.
	defer func() {
		if err != nil {
			d.Stop(ctx)
		}
	}()

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
	}, "\n"), dhcpIPStart.String(), dhcpIPEnd.String(), d.iface, d.leasePath())
	if err := utils.WriteToHost(ctx, d.host, d.confPath(), []byte(conf)); err != nil {
		return errors.Wrap(err, "failed to write config")
	}

	testing.ContextLog(ctx, "Starting dnsmasq")
	cmd := d.host.Command(dnsmasqCmd, fmt.Sprintf("--conf-file=%s", d.confPath()), "--no-daemon")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to obtain StdoutPipe of dnsmasq")
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return errors.Wrap(err, "failed to obtain StderrPipe of dnsmasq")
	}
	if err := cmd.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start dnsmasq")
	}
	d.cmd = cmd

	// Collect stdout and stderr. Start reader routines after cmd.Start to ensure that
	// they won't read pipes which may never be closed.
	if err := utils.ReadToOutDir(ctx, d.stdoutFile(), stdout, nil); err != nil {
		return errors.Wrap(err, "failed to spawn reader for stdout")
	}
	if err := utils.ReadToOutDir(ctx, d.stderrFile(), stderr, nil); err != nil {
		return errors.Wrap(err, "failed to spawn reader for stderr")
	}

	return nil
}

// Stop the dhcp server and cleanup related resources.
func (d *Server) Stop(ctx context.Context) error {
	testing.ContextLog(ctx, "Stoping dnsmasq")
	var err error
	if d.cmd == nil {
		err = errors.New("server not started")
	} else {
		d.cmd.Abort()
		// TODO(crbug.com/1030635): Abort might not work, use pkill to ensure the daemon is killed.
		d.host.Command("pkill", "-f", fmt.Sprintf("^%s.*%s", dnsmasqCmd, d.confPath())).Run(ctx)

		// Skip the error in Wait as the process is aborted and always has error in wait.
		d.cmd.Wait(ctx)
		d.cmd = nil
	}
	if err2 := d.host.Command("rm", d.confPath()).Run(ctx); err2 != nil {
		err = errors.Wrapf(err, "failed to remove config with err=%s", err2.Error())
	}
	if err2 := d.flushIP(ctx); err2 != nil {
		err = errors.Wrapf(err, "failed to flush ip setting, err=%s", err2.Error())
	}
	return err
}

func (d *Server) getSubnetIP(suffix byte) net.IP {
	return net.IPv4(192, 168, d.conf.IPIndex, suffix)
}

// IPRange returns the starting and ending IP in DHCP range.
func (d *Server) IPRange() (net.IP, net.IP) {
	return d.getSubnetIP(1), d.getSubnetIP(128)
}

// ServerIP returns the IP used by DHCP server.
func (d *Server) ServerIP() net.IP {
	return d.getSubnetIP(254)
}

// Mask returns the mask of DHCP subnet.
func (d *Server) Mask() net.IPMask {
	return net.IPv4Mask(255, 255, 255, 0)
}

// BroadcastIP returns the broadcast IP of DHCP subnet.
func (d *Server) BroadcastIP() net.IP {
	return d.getSubnetIP(255)
}

// Config returns the config used to spawn this DHCP server.
func (d *Server) Config() Config {
	return *d.conf
}

func (d *Server) flushIP(ctx context.Context) error {
	if out, err := d.host.Command("ip", "addr", "flush", d.iface).CombinedOutput(ctx); err != nil {
		return errors.Wrapf(err, "failed to flush addresses: %s", string(out))
	}
	return nil
}

func (d *Server) configureIP(ctx context.Context) error {
	if err := d.flushIP(ctx); err != nil {
		return err
	}
	maskLen, _ := d.Mask().Size()
	cmd := d.host.Command("ip", "addr", "add", fmt.Sprintf("%s/%d", d.ServerIP().String(), maskLen),
		"broadcast", d.BroadcastIP().String(), "dev", d.iface)
	if out, err := cmd.CombinedOutput(ctx); err != nil {
		return errors.Wrapf(err, "failed to add address: %s", string(out))
	}
	return nil
}
