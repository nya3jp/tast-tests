// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dhcp provides utilities for controlling DHCP server.
package dhcp

import (
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	"chromiumos/tast/remote/wificell/fileutil"
	"chromiumos/tast/testing"
)

const (
	dnsmasqCmd    = "dnsmasq"
	dhcpLeaseFile = "dhcpd.leases"
)

// KillAll kills all running dhcp server on host, useful for environment setup/cleanup.
func KillAll(ctx context.Context, host *host.SSH) error {
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

	cmd        *host.Cmd
	stdoutFile *os.File
	stderrFile *os.File
}

// StartServer creates and runs a DHCP server on iface of the given host with settings specified in conf.
// workDir is the dir on host for the server to put temporary files.
// name is the identifier used for log filenames in OutDir.
// ipStart, ipEnd specifies the leasable range for this dhcp server to offer.
func StartServer(ctx context.Context, host *host.SSH, name, iface, workDir string, ipStart, ipEnd net.IP) (*Server, error) {
	s := &Server{
		host:    host,
		name:    name,
		iface:   iface,
		workDir: workDir,
		ipStart: ipStart,
		ipEnd:   ipEnd,
	}
	if err := s.start(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// filename returns the filename for this instance to store different type of information.
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

// stdoutFilename returns the filename under OutDir to store stdout of this daemon.
func (d *Server) stdoutFilename() string {
	return d.filename("stdout")
}

// stderrFilename returns the filename under OutDir to store stderr of this daemon.
func (d *Server) stderrFilename() string {
	return d.filename("stderr")
}

// start spawns dnsmasq daemon.
func (d *Server) start(ctx context.Context) (err error) {
	// Clean up on error.
	defer func() {
		if err != nil {
			d.Close(ctx)
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
	if err := fileutil.WriteToHost(ctx, d.host, d.confPath(), []byte(conf)); err != nil {
		return errors.Wrap(err, "failed to write config")
	}

	testing.ContextLogf(ctx, "Starting dnsmasq %s on interface %s", d.name, d.iface)
	// TODO(crbug.com/1030635): though it is better to use --conf-file=- so that it
	// can write conf to stdin without file i/o. However, we need the conf filename
	// as the command's signature so that pkill works. Switch to write conf file to
	// stdin once we don't need pkill to kill the process.
	cmd := d.host.Command(dnsmasqCmd, fmt.Sprintf("--conf-file=%s", d.confPath()), "--no-daemon")

	// Prepare stdout/stderr log files.
	d.stdoutFile, err = fileutil.PrepareOutDirFile(ctx, d.stdoutFilename())
	if err != nil {
		return errors.Wrap(err, "failed to open stdout log of dnsmasq")
	}
	cmd.Stdout = d.stdoutFile
	d.stderrFile, err = fileutil.PrepareOutDirFile(ctx, d.stderrFilename())
	if err != nil {
		return errors.Wrap(err, "failed to open stdout log of dnsmasq")
	}
	cmd.Stderr = d.stderrFile

	if err := cmd.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start dnsmasq")
	}
	d.cmd = cmd

	return nil
}

// Close stops the dhcp server and cleans up related resources.
func (d *Server) Close(ctx context.Context) error {
	testing.ContextLog(ctx, "Stopping dnsmasq")
	if d.cmd != nil {
		d.cmd.Abort()
		// TODO(crbug.com/1030635): Abort might not work, use pkill to ensure the daemon is killed.
		d.host.Command("pkill", "-f", fmt.Sprintf("^%s.*%s", dnsmasqCmd, d.confPath())).Run(ctx)

		// Skip the error in Wait as the process is aborted and always has error in wait.
		d.cmd.Wait(ctx)
		d.cmd = nil
	}
	if d.stdoutFile != nil {
		d.stdoutFile.Close()
	}
	if d.stderrFile != nil {
		d.stderrFile.Close()
	}
	if err := d.host.Command("rm", d.confPath()).Run(ctx); err != nil {
		return errors.Wrap(err, "failed to remove config")
	}
	return nil
}
