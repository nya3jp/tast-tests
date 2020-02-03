// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/remote/wificell/fileutils"
	"chromiumos/tast/testing"
)

const (
	dnsmasqCmd    = "dnsmasq"
	dhcpLeaseFile = "dhcpd.leases"
)

// Killall kills all running dhcp server on host, useful for environment setup/cleanup.
func Killall(ctx context.Context, host *host.SSH) error {
	return host.Command("killall", dnsmasqCmd).Run(ctx)
}

// Server controls a DHCP server on AP router.
type Server struct {
	host    *host.SSH // TODO(crbug.com/1019537): use a more suitable ssh object.
	name    string
	iface   string
	workDir string
	ipStart net.IP
	ipEnd   net.IP
	cmd     *host.Cmd
}

// NewServer creates and runs a DHCP server on iface of the given host with settings specified in conf.
// workDir is the dir on host for the server to put temporary files.
// name is the identifier used for log filenames in OutDir.
func NewServer(host *host.SSH, name, iface, workDir string, ipStart, ipEnd net.IP) *Server {
	return &Server{
		host:    host,
		name:    name,
		iface:   iface,
		workDir: workDir,
		ipStart: ipStart,
		ipEnd:   ipEnd,
	}
}

// filename for this instance to store different type of information.
// suffix can be the type of stored information. e.g. conf, stdout, stderr ...
func (d *Server) filename(suffix string) string {
	return fmt.Sprintf("dnsmasq-%s-%s.%s", d.name, d.iface, suffix)
}

// confPath returns the location on host of dnsmasq.conf for this instance.
func (d *Server) confPath() string {
	return path.Join(d.workDir, d.filename("conf"))
}

// leasePath returns the location on host of dhcp lease file.
func (d *Server) leasePath() string {
	return path.Join(d.workDir, dhcpLeaseFile)
}

// stdoutFile returns the filename under OutDir to store stdout of this daemon.
func (d *Server) stdoutFile() string {
	return d.filename("stdout")
}

// stderrFile returns the filename under OutDir to store stderr of this daemon.
func (d *Server) stderrFile() string {
	return d.filename("stderr")
}

// Start dnsmasq daemon.
func (d *Server) Start(ctx context.Context) (err error) {
	// Clean up on error.
	defer func() {
		if err != nil {
			d.Stop(ctx)
		}
	}()

	conf := fmt.Sprintf(strings.Join([]string{
		"port=0", // Disables DNS server.
		"bind-interfaces",
		"log-dhcp",
		"dhcp-range=%s,%s",
		"interface=%s",
		"dhcp-leasefile=%s",
	}, "\n"), d.ipStart.String(), d.ipEnd.String(), d.iface, d.leasePath())
	if err := fileutils.WriteToHost(ctx, d.host, d.confPath(), []byte(conf)); err != nil {
		return errors.Wrap(err, "failed to write config")
	}

	testing.ContextLogf(ctx, "Starting dnsmasq %s on interface %s", d.name, d.iface)
	// TODO(crbug.com/1030635): might be better to use --conf-file=- and write conf
	// to its stdin. However, we might need to remove the pkill first or else we have
	// no hint to pgrep the specific process
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
	if err := fileutils.ReadToOutDir(ctx, d.stdoutFile(), stdout); err != nil {
		return errors.Wrap(err, "failed to spawn reader for stdout")
	}
	if err := fileutils.ReadToOutDir(ctx, d.stderrFile(), stderr); err != nil {
		return errors.Wrap(err, "failed to spawn reader for stderr")
	}

	return nil
}

// Stop the dhcp server and cleanup related resources.
func (d *Server) Stop(ctx context.Context) error {
	testing.ContextLog(ctx, "Stopping dnsmasq")
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
	return err
}
