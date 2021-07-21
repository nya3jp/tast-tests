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
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/fileutil"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	dnsmasqCmd    = "dnsmasq"
	dhcpLeaseFile = "dhcpd.leases"
)

// KillAll kills all running dhcp server on host, useful for environment setup/cleanup.
func KillAll(ctx context.Context, host *ssh.Conn) error {
	return host.CommandContext(ctx, "killall", dnsmasqCmd).Run()
}

// Server controls a DHCP server on AP router.
type Server struct {
	host    *ssh.Conn
	name    string
	iface   string
	workDir string
	ipStart net.IP
	ipEnd   net.IP

	cmd        *ssh.CmdCtx
	stdoutFile *os.File
	stderrFile *os.File
}

// StartServer creates and runs a DHCP server on iface of the given host with settings specified in conf.
// workDir is the dir on host for the server to put temporary files.
// name is the identifier used for log filenames in OutDir.
// ipStart, ipEnd specifies the leasable range for this dhcp server to offer.
// After getting a Server instance, d, the caller should call d.Close() at the end, and use the
// shortened ctx (provided by d.ReserveForClose()) before d.Close() to reserve time for it to run.
func StartServer(ctx context.Context, host *ssh.Conn, name, iface, workDir string, ipStart, ipEnd net.IP) (*Server, error) {
	ctx, st := timing.Start(ctx, "dhcp.StartServer")
	defer st.End()

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
func (d *Server) start(fullCtx context.Context) (err error) {
	defer func() {
		if err != nil {
			d.Close(fullCtx)
		}
	}()

	ctx, cancel := d.ReserveForClose(fullCtx)
	defer cancel()

	conf := fmt.Sprintf(strings.Join([]string{
		"port=0", // Disables DNS server.
		"bind-interfaces",
		"log-dhcp",
		"dhcp-range=%s,%s",
		"interface=%s",
		"dhcp-leasefile=%s",
	}, "\n"), d.ipStart.String(), d.ipEnd.String(), d.iface, d.leasePath())
	if err := linuxssh.WriteFile(ctx, d.host, d.confPath(), []byte(conf), 0644); err != nil {
		return errors.Wrap(err, "failed to write config")
	}

	testing.ContextLogf(ctx, "Starting dnsmasq %s on interface %s", d.name, d.iface)
	// TODO(crbug.com/1030635): though it is better to use --conf-file=- so that it
	// can write conf to stdin without file i/o. However, we need the conf filename
	// as the command's signature so that pkill works. Switch to write conf file to
	// stdin once we don't need pkill to kill the process.
	cmd := d.host.CommandContext(ctx, dnsmasqCmd, fmt.Sprintf("--conf-file=%s", d.confPath()), "--no-daemon")

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

	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start dnsmasq")
	}
	d.cmd = cmd

	return nil
}

// ReserveForClose returns a shortened ctx with cancel function.
// The shortened ctx is used for running things before d.Close() to reserve time for it to run.
func (d *Server) ReserveForClose(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, 2*time.Second)
}

// Close stops the dhcp server and cleans up related resources.
func (d *Server) Close(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "dhcp.Close")
	defer st.End()

	testing.ContextLog(ctx, "Stopping dnsmasq")
	if d.cmd != nil {
		d.cmd.Abort()
		// TODO(crbug.com/1030635): Abort might not work, use pkill to ensure the daemon is killed.
		d.host.CommandContext(ctx, "pkill", "-f", fmt.Sprintf("^%s.*%s", dnsmasqCmd, d.confPath())).Run()

		// Skip the error in Wait as the process is aborted and always has error in wait.
		d.cmd.Wait()
		d.cmd = nil
	}
	if d.stdoutFile != nil {
		d.stdoutFile.Close()
	}
	if d.stderrFile != nil {
		d.stderrFile.Close()
	}
	if err := d.host.CommandContext(ctx, "rm", d.confPath()).Run(); err != nil {
		return errors.Wrap(err, "failed to remove config")
	}
	return nil
}

// Interface returns the interface where DHCP server is running on.
func (d *Server) Interface() string {
	return d.iface
}
