// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package http provides utilities for controlling HTTP server.
package http

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
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
	pythonCmd = "/usr/local/bin/python3"
	script    = "httpserver.py"
)

// Server controls a DHCP server on AP router.
type Server struct {
	host             *ssh.Conn
	name             string
	iface            string
	workDir          string
	remoteScriptPath string
	port             string
	statusCode       string
	redirectAddr     string

	cmd        *ssh.Cmd
	stdoutFile *os.File
	stderrFile *os.File
}

// StartServer creates and runs a HTTP server by executing a python script.
func StartServer(ctx context.Context, host *ssh.Conn, name, iface, workDir, port, statusCode, redirectAddr string) (*Server, error) {
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
// suffix can be the type of stored information. e.g. conf, stdout, stderr ...
func (d *Server) filename(suffix string) string {
	return fmt.Sprintf("dnsmasq-%s-%s.%s", d.name, d.iface, suffix)
}

// confPath returns the location on host of dnsmasq.conf for this instance.
func (d *Server) confPath() string {
	return path.Join(d.workDir, d.filename("conf"))
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

	localScriptPath, err := d.localServerPath(script)
	if err != nil {
		return errors.Wrap(err, "failed to get local path of HTTP server script")
	}
	d.remoteScriptPath = d.remoteServerPath(script)

	// Copy the local HTTP server script to the remote router.
	if _, err := linuxssh.PutFiles(ctx, d.host, map[string]string{
		localScriptPath: d.remoteScriptPath,
	},
		linuxssh.DereferenceSymlinks); err != nil {
		return errors.Wrap(err, "failed to send script to remote host")
	}

	cmd := d.host.CommandContext(ctx, pythonCmd, d.remoteScriptPath, d.port, d.statusCode, d.redirectAddr)

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
		// TODO(crbug.com/1030635): Abort might not work, use pkill to ensure the daemon is killed.
		d.host.CommandContext(ctx, "pkill", "-f", fmt.Sprintf("^%s.*%s", pythonCmd, d.remoteScriptPath)).Run()
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
	if err := d.host.CommandContext(ctx, "rm", d.remoteScriptPath).Run(); err != nil {
		return errors.Wrap(err, "failed to remove HTTP server script")
	}
	return nil
}

func (d *Server) localServerPath(fileName string) (string, error) {
	localPath := "/"
	thisDir, err := os.Getwd()
	if err != nil {
		return "", errors.Wrap(err, "failed to get working directory")
	}
	thisDir, err = filepath.Abs(thisDir)
	if err != nil {
		return "", errors.Wrap(err, "failed to get absolute path of working directory")
	}
	thisDir = filepath.ToSlash(thisDir)
	for _, dir := range strings.Split(thisDir, "/") {
		localPath = filepath.Join(localPath, dir)
		if dir == "src" {
			localPath = filepath.Join(localPath, "platform", "tast-tests", "src", "chromiumos", "tast", "remote", "wificell", "http", fileName)
			return localPath, nil
		}
	}
	return "", errors.New("failed to find local path for fileName")
}

// remoteServerPath returns the location on host of HTTP server script for this instance.
func (d *Server) remoteServerPath(fileName string) string {
	return path.Join(d.workDir, fileName)
}
