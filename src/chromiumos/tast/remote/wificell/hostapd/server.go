// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hostapd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"

	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	"chromiumos/tast/remote/wificell/fileutil"
	"chromiumos/tast/testing"
)

const (
	hostapdCmd = "hostapd"
)

// KillAll kills all running hostapd on host, useful for environment setup/cleanup.
func KillAll(ctx context.Context, host *host.SSH) error {
	return host.Command("killall", hostapdCmd).Run(ctx)
}

// Server controls a hostapd on router.
type Server struct {
	host    *host.SSH // TODO(crbug.com/1019537): use a more suitable ssh object.
	name    string
	iface   string
	workDir string
	conf    *Config

	cmd        *host.Cmd
	stdoutFile *os.File
	stderrFile *os.File
}

// NewServer creates a new Server object and runs hostapd on iface of the given host with settings
// specified in config. workDir is the dir on host for the server to put temporary files.
// name is the identifier used for log filenames in OutDir.
func NewServer(host *host.SSH, name, iface, workDir string, config *Config) *Server {
	return &Server{
		host:    host,
		name:    name,
		iface:   iface,
		workDir: workDir,
		conf:    config,
	}
}

// filename is the filename for this instance to store different type of information.
// suffix can be the type of stored information. e.g. conf, stdout, stderr ...
func (ap *Server) filename(suffix string) string {
	return fmt.Sprintf("hostapd-%s-%s.%s", ap.name, ap.iface, suffix)
}

// confPath is the location on host of hostapd.conf for this instance.
func (ap *Server) confPath() string {
	return path.Join(ap.workDir, ap.filename("conf"))
}

// ctrlPath is the location on host of control socket of this hostapd.
func (ap *Server) ctrlPath() string {
	return path.Join(ap.workDir, ap.filename("ctrl"))
}

// stdoutFilename is the filename under OutDir to store stdout of this hostapd.
func (ap *Server) stdoutFilename() string {
	return ap.filename("stdout")
}

// stderrFilename is the filename under OutDir to store stderr of this hostapd.
func (ap *Server) stderrFilename() string {
	return ap.filename("stderr")
}

// Start spawns hostapd daemon and waits until it is ready.
func (ap *Server) Start(ctx context.Context) (err error) {
	// Cleanup on error.
	defer func() {
		if err != nil {
			ap.Close(ctx)
		}
	}()

	conf, err := ap.conf.Format(ap.iface, ap.ctrlPath())
	if err != nil {
		return err
	}
	if err := fileutil.WriteToHost(ctx, ap.host, ap.confPath(), []byte(conf)); err != nil {
		return errors.Wrap(err, "failed to write config")
	}

	testing.ContextLogf(ctx, "Starting hostapd %s on interface %s", ap.name, ap.iface)
	cmd := ap.host.Command(hostapdCmd, "-dd", "-t", ap.confPath())

	// Prepare stdout/stderr log files.
	ap.stderrFile, err = fileutil.PrepareOutDirFile(ctx, ap.stderrFilename())
	if err != nil {
		return errors.Wrap(err, "failed to open stderr log of hostapd")
	}
	cmd.Stderr = ap.stderrFile

	ap.stdoutFile, err = fileutil.PrepareOutDirFile(ctx, ap.stdoutFilename())
	if err != nil {
		return errors.Wrap(err, "failed to open stdout log of hostapd")
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to obtain StdoutPipe of hostapd")
	}
	readyWriter := newReadyWriter()
	go func() {
		multiWriter := io.MultiWriter(ap.stdoutFile, readyWriter)
		defer readyWriter.Close()
		io.Copy(multiWriter, stdoutPipe)
	}()

	if err := cmd.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start hostapd")
	}
	ap.cmd = cmd

	// Wait for hostapd to get ready.
	if err := readyWriter.wait(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "hostapd started")
	return nil
}

// Close stops hostapd and cleans up related resources.
func (ap *Server) Close(ctx context.Context) error {
	testing.ContextLog(ctx, "Stopping hostapd")
	if ap.cmd != nil {
		ap.cmd.Abort()
		// TODO(crbug.com/1030635): Abort might not work, use pkill to ensure the daemon is killed.
		ap.host.Command("pkill", "-f", fmt.Sprintf("^%s.*%s", hostapdCmd, ap.confPath()))

		// Skip the error in Wait as the process is aborted and always has error in wait.
		ap.cmd.Wait(ctx)
		ap.cmd = nil
	}
	if ap.stdoutFile != nil {
		ap.stdoutFile.Close()
	}
	if ap.stderrFile != nil {
		ap.stderrFile.Close()
	}
	if err := ap.host.Command("rm", ap.confPath()).Run(ctx); err != nil {
		return errors.Wrap(err, "failed to remove config")
	}
	return nil
}

// Interface returns the interface used by the hostapd.
func (ap *Server) Interface() string {
	return ap.iface
}

// Config returns the config used by the hostapd.
func (ap *Server) Config() Config {
	return *ap.conf
}
