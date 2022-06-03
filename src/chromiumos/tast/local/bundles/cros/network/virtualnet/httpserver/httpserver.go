// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package httpserver provides the utils to run an httpserver inside a
// virtualnet.Env.
package httpserver

import (
	"context"
	"os"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/env"
)

// Paths in chroot.
const (
	logPath    = "/tmp/httpServer.log"
	pythonPath = "/usr/local/bin/python3"
)

var (
	httpServerResponseCode = 302
)

type httpserver struct {
	ip   string
	port string
	env  *env.Env
	cmd  *testexec.Cmd
}

// New creates a new dnsmasq object. Currently dnsmasq will only be used as a
// DHCP server daemon. The returned object can be passed to Env.StartServer(),
// its lifetime will be managed by the Env object.
func New(ip, port string) *httpserver {
	return &httpserver{ip: ip, port: port}
}

// Start starts the http server process.
func (h *httpserver) Start(ctx context.Context, env *env.Env) error {
	h.env = env

	// Start the command.
	cmd := []string{
		pythonPath,
		"-m",
		"http.server",
		"--bind", h.ip,
		h.port,
	}

	// Bring up loopback interface since packets go through local route table.
	if err := h.env.RunWithoutChroot(ctx, "ip", "link", "set", "lo", "up"); err != nil {
		return errors.Wrapf(err, "failed to bring lo up in %s", h.env.NetNSName)
	}

	h.cmd = h.env.CreateCommand(ctx, cmd...)
	if err := h.cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start python http server")
	}

	return nil
}

// Start starts the http server process.
func (h *httpserver) Stop(ctx context.Context) error {
	if h.cmd == nil || h.cmd.Process == nil {
		return nil
	}
	if err := h.cmd.Kill(); err != nil {
		return errors.Wrap(err, "failed to kill python http server processs")
	}
	h.cmd.Wait()
	h.cmd = nil
	return nil
}

// WriteLogs writes logs into |f|.
func (h *httpserver) WriteLogs(ctx context.Context, f *os.File) error {
	return h.env.ReadAndWriteLogIfExists(h.env.ChrootPath(logPath), f)
}
