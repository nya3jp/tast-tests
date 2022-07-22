// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dhcp provides utilities for controlling DHCP server.
package dhcp

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net"
	"os"
	"path"
	"strconv"
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
	confTemplate  = `
port={{.port}}
bind-interfaces
log-dhcp
dhcp-range={{.ipStart}},{{.ipEnd}}
interface={{.iface}}
dhcp-leasefile={{.leasefile}}
no-resolv
no-hosts
{{if .address}}
address={{.address}}
{{end}}
{{if .nameServers}}
dhcp-option=option:dns-server,{{.nameServers}}
{{end}}
`
)

// KillAll kills all running dhcp server on host, useful for environment setup/cleanup.
func KillAll(ctx context.Context, host *ssh.Conn) error {
	cmd := dnsmasqCmd
	killallOutput, killAllErr := host.CommandContext(ctx, "killall", cmd).Output()
	pgrepOutput, err := host.CommandContext(ctx, "pgrep", cmd).Output()
	if err != nil {
		if err.Error() == "Process exited with status 1" {
			return nil // no processes found, kill successful
		}
		return errors.Wrapf(err, "failed to verify that all %s processes have been killed: %s", cmd, string(pgrepOutput))
	}
	if killAllErr != nil {
		return errors.Wrapf(killAllErr, "found processes matching %q still running after failed killall (output=%q), pgrep output: %s", cmd, string(killallOutput), string(pgrepOutput))
	}
	return errors.Errorf("found processes matching %q still running after successful killall (output=%q), pgrep output: %s", cmd, string(killallOutput), string(pgrepOutput))
}

// Server controls a DHCP server on AP router.
type Server struct {
	host    *ssh.Conn
	name    string
	iface   string
	workDir string
	ipStart net.IP
	ipEnd   net.IP
	dnsOpt  *DNSOption

	cmd        *ssh.Cmd
	stdoutFile *os.File
	stderrFile *os.File
}

// DNSOption handles parameters to enable the DNS server.
type DNSOption struct {
	Port int
	// NameServers contains the DNS server list which will be broadcasted by DHCP.
	NameServers []string
	// ResolvedHost is the hostname to force a specific IPv4 or IPv6 address. When
	// ResolvedHost is queried from dnsmasq, dnsmasq will respond with ResolveHostToIP.
	// If resolvedHost is not set, it matches any domain in dnsmasq configuration.
	ResolvedHost string
	// ResolveHostToIP is the IP address returned when doing a DNS query for ResolvedHost.
	ResolveHostToIP net.IP
}

// StartServer creates and runs a DHCP server on iface of the given host with settings specified in conf.
// workDir is the dir on host for the server to put temporary files.
// name is the identifier used for log filenames in OutDir.
// ipStart, ipEnd specifies the leasable range for this dhcp server to offer.
// dnsOpt contains the configuration of the DNS server.
// After getting a Server instance, d, the caller should call d.Close() at the end, and use the
// shortened ctx (provided by d.ReserveForClose()) before d.Close() to reserve time for it to run.
func StartServer(ctx context.Context, host *ssh.Conn, name, iface, workDir string, ipStart, ipEnd net.IP, dnsOpt *DNSOption) (*Server, error) {
	ctx, st := timing.Start(ctx, "dhcp.StartServer")
	defer st.End()

	s := &Server{
		host:    host,
		name:    name,
		iface:   iface,
		workDir: workDir,
		ipStart: ipStart,
		ipEnd:   ipEnd,
		dnsOpt:  dnsOpt,
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

	// Prepare config file.
	confVals := map[string]string{
		"port":      "0", // disable DNS
		"iface":     d.iface,
		"ipStart":   d.ipStart.String(),
		"ipEnd":     d.ipEnd.String(),
		"leasefile": d.leasePath(),
	}

	// Need DNS functionality.
	if d.dnsOpt != nil {
		var resolvedIP, resolvedHost string
		confVals["port"] = strconv.Itoa(d.dnsOpt.Port)

		if d.dnsOpt.ResolveHostToIP == nil {
			resolvedIP = ""
		} else {
			resolvedIP = d.dnsOpt.ResolveHostToIP.String()
		}

		if d.dnsOpt.ResolvedHost == "" {
			resolvedHost = "#"
		}

		confVals["address"] = fmt.Sprintf("/%s/%s", resolvedHost, resolvedIP)

		if len(d.dnsOpt.NameServers) > 0 {
			confVals["nameServers"] = strings.Join(d.dnsOpt.NameServers, ",")
		}
	}

	b := &bytes.Buffer{}
	template.Must(template.New("").Parse(confTemplate)).Execute(b, confVals)
	if err := linuxssh.WriteFile(ctx, d.host, d.confPath(), []byte(b.String()), 0644); err != nil {
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
	configPath := d.confPath()
	if d.cmd != nil {
		d.cmd.Abort()
		// TODO(crbug.com/1030635): Abort might not work, use pkill to ensure the daemon is killed.
		_ = d.host.CommandContext(ctx, "pkill", "-f", fmt.Sprintf("^%s.*%s", dnsmasqCmd, configPath)).Run()

		// Skip the error in Wait as the process is aborted and always has error in wait.
		_ = d.cmd.Wait()
		d.cmd = nil
	}
	if d.stdoutFile != nil {
		_ = d.stdoutFile.Close()
	}
	if d.stderrFile != nil {
		_ = d.stderrFile.Close()
	}
	if err := d.host.CommandContext(ctx, "rm", configPath).Run(); err != nil {
		return errors.Wrapf(err, "failed to remove dhcp config at %q", configPath)
	}
	return nil
}

// Interface returns the interface where DHCP server is running on.
func (d *Server) Interface() string {
	return d.iface
}
