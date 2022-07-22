// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package http provides utilities for controlling HTTP server.
package http

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"
	"path"
	"strconv"
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
	pythonCmd    = "/usr/local/bin/python3"
	serverScript = `
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer
	
class RequestHandler(BaseHTTPRequestHandler):
	def do_GET(self):
		message = "HTTP server is running"
		self.protocol_version = "HTTP/1.1"
		self.send_response(int(sys.argv[2]))
		self.send_header("Location", sys.argv[3])
		self.end_headers()
		self.wfile.write(bytes(message, "utf8"))
		return
	
	def do_HEAD(self):
		self.protocol_version = "HTTP/1.1"
		self.send_response(int(sys.argv[2]))
		self.send_header("Location", sys.argv[3])
		self.end_headers()
		return
	
def run():
	server = ('', int(sys.argv[1]))
	httpd = HTTPServer(server, RequestHandler)
	httpd.serve_forever()
	
run()
`
)

// Server controls a HTTP server on AP router.
type Server struct {
	host         *ssh.Conn
	name         string
	iface        string
	workDir      string
	redirectAddr string
	port         int
	statusCode   int

	cmd        *ssh.Cmd
	stdoutFile *os.File
	stderrFile *os.File
}

// StartServer creates and runs a HTTP server by executing a python script.
func StartServer(ctx context.Context, host *ssh.Conn, name, iface, workDir, redirectAddr string, port, statusCode int) (*Server, error) {
	ctx, st := timing.Start(ctx, "http.StartServer")
	defer st.End()

	s := &Server{
		host:         host,
		name:         name,
		iface:        iface,
		workDir:      workDir,
		port:         port,
		statusCode:   statusCode,
		redirectAddr: redirectAddr,
	}
	if err := s.start(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// filename returns the filename for this instance to store different type of information.
// suffix can be the type of stored information.
func (d *Server) filename(suffix string) string {
	return fmt.Sprintf("httpserver-%s-%s.%s", d.name, d.iface, suffix)
}

// pyPath returns the python file location on host for this instance.
func (d *Server) pyPath() string {
	return path.Join(d.workDir, d.filename("py"))
}

// stdoutFilename returns the filename under OutDir to store stdout of this daemon.
func (d *Server) stdoutFilename() string {
	return d.filename("stdout")
}

// stderrFilename returns the filename under OutDir to store stderr of this daemon.
func (d *Server) stderrFilename() string {
	return d.filename("stderr")
}

// start spawns HTTP daemon.
func (d *Server) start(fullCtx context.Context) (err error) {
	defer func() {
		if err != nil {
			d.Close(fullCtx)
		}
	}()

	ctx, cancel := d.ReserveForClose(fullCtx)
	defer cancel()

	b := &bytes.Buffer{}
	template.Must(template.New("").Parse(serverScript)).Execute(b, nil)
	if err := linuxssh.WriteFile(ctx, d.host, d.pyPath(), []byte(b.String()), 0644); err != nil {
		return errors.Wrap(err, "failed to write python script")
	}
	cmd := d.host.CommandContext(ctx, pythonCmd, d.pyPath(), strconv.Itoa(d.port), strconv.Itoa(d.statusCode), d.redirectAddr)

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
		return errors.Wrap(err, "failed to start HTTP server")
	}
	d.cmd = cmd
	testing.ContextLogf(ctx, "Starting HTTP server %s on interface %s", d.name, d.iface)
	return nil
}

// ReserveForClose returns a shortened ctx with cancel function.
// The shortened ctx is used for running things before d.Close() to reserve time for it to run.
func (d *Server) ReserveForClose(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, 2*time.Second)
}

// Close stops the HTTP server and cleans up related resources.
func (d *Server) Close(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "http.Close")
	defer st.End()

	testing.ContextLog(ctx, "Stopping HTTP")
	if d.cmd != nil {
		d.cmd.Abort()
		// TODO(b/187790213): Abort might not work, use pkill to ensure the daemon is killed.
		d.host.CommandContext(ctx, "pkill", "-f", fmt.Sprintf("^%s.*%s", pythonCmd, d.pyPath())).Run()
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
	if err := d.host.CommandContext(ctx, "rm", d.pyPath()).Run(); err != nil {
		return errors.Wrap(err, "failed to remove HTTP server script")
	}
	return nil
}
