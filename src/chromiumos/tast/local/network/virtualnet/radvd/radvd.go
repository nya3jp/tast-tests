// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package radvd provides the utils to run the radvd server inside a
// virtualnet.Env. This simulate an IPv6 router advertising the IPv6
// configuration of the network with RA packets.
package radvd

import (
	"bytes"
	"context"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"text/template"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/network/virtualnet/env"
)

const confTemplate = `
interface {{.ifname}} {
	MinRtrAdvInterval 3;
	MaxRtrAdvInterval 4;
	AdvSendAdvert on;
	AdvManagedFlag on;
	prefix {{.prefix}} {};
	{{if .dns}}
	RDNSS {{.dns}} {};
	{{end}}
};`

// Paths in chroot.
const (
	radvdPath = "/usr/local/radvd"
	confPath  = "/tmp/radvd.conf"
	pidPath   = "/tmp/radvd.pid"
	logPath   = "/tmp/radvd.log"
)

type radvd struct {
	env    *env.Env
	prefix *net.IPNet
	dns    []string
	cmd    *testexec.Cmd
}

// New creates a new radvd object. The returned object can be passed to
// Env.StartServer(), its lifetime will be managed by the Env object.
func New(prefix *net.IPNet, dns []string) *radvd {
	return &radvd{prefix: prefix, dns: dns}
}

// Start starts the radvd process.
func (r *radvd) Start(ctx context.Context, env *env.Env) error {
	r.env = env

	// Prepare config file.
	confVals := map[string]string{
		"ifname": r.env.VethInName,
		"prefix": r.prefix.String(),
	}
	if len(r.dns) > 0 {
		confVals["dns"] = strings.Join(r.dns, ",")
	}
	b := &bytes.Buffer{}
	template.Must(template.New("").Parse(confTemplate)).Execute(b, confVals)
	if err := ioutil.WriteFile(r.env.ChrootPath(confPath), []byte(b.String()), 0644); err != nil {
		return errors.Wrap(err, "failed to write config file")
	}

	// Configure IPv6 environment in netns.
	if err := r.env.RunWithoutChroot(ctx, "sysctl", "-w", "net.ipv6.conf."+r.env.VethInName+".accept_ra=2"); err != nil {
		return errors.Wrapf(err, "failed to configure accept_ra for %s", r.env.VethInName)
	}

	// Start the command.
	cmd := []string{
		radvdPath,
		"--nodaemon",
		"-d", "4",
		"-C", confPath,
		"-p", pidPath,
		"-m", "logfile", "-l", logPath,
	}
	r.cmd = r.env.CreateCommand(ctx, cmd...)
	if err := r.cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start radvd daemon")
	}

	return nil
}

// Stop stops the radvd process.
func (r *radvd) Stop(ctx context.Context) error {
	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}
	if err := r.cmd.Kill(); err != nil {
		return errors.Wrap(err, "failed to kill radvd processs")
	}
	r.cmd.Wait()
	r.cmd = nil
	return nil
}

// WriteLogs writes logs into |f|.
func (r *radvd) WriteLogs(ctx context.Context, f *os.File) error {
	return r.env.ReadAndWriteLogIfExists(r.env.ChrootPath(logPath), f)
}
