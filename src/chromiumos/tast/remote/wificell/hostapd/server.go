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

// filename returns a filename for s to store different type of information.
// suffix can be the type of stored information. e.g. conf, stdout, stderr ...
func (s *Server) filename(suffix string) string {
	return fmt.Sprintf("hostapd-%s-%s.%s", s.name, s.iface, suffix)
}

// confPath returns the path of s's config file.
func (s *Server) confPath() string {
	return path.Join(s.workDir, s.filename("conf"))
}

// ctrlPath returns the path of s's control socket.
func (s *Server) ctrlPath() string {
	return path.Join(s.workDir, s.filename("ctrl"))
}

// stdoutFilename returns the filename under OutDir to store stdout of this hostapd.
func (s *Server) stdoutFilename() string {
	return s.filename("stdout")
}

// stderrFilename returns the filename under OutDir to store stderr of this hostapd.
func (s *Server) stderrFilename() string {
	return s.filename("stderr")
}

// Start spawns a hostapd daemon and waits until it is ready.
func (s *Server) Start(ctx context.Context) (err error) {
	// Cleanup on error.
	defer func() {
		if err != nil {
			s.Close(ctx)
		}
	}()

	conf, err := s.conf.Format(s.iface, s.ctrlPath())
	if err != nil {
		return err
	}
	if err := fileutil.WriteToHost(ctx, s.host, s.confPath(), []byte(conf)); err != nil {
		return errors.Wrap(err, "failed to write config")
	}

	testing.ContextLogf(ctx, "Starting hostapd %s on interface %s", s.name, s.iface)
	cmd := s.host.Command(hostapdCmd, "-dd", "-t", s.confPath())

	// Prepare stdout/stderr log files.
	s.stderrFile, err = fileutil.PrepareOutDirFile(ctx, s.stderrFilename())
	if err != nil {
		return errors.Wrap(err, "failed to open stderr log of hostapd")
	}
	cmd.Stderr = s.stderrFile

	s.stdoutFile, err = fileutil.PrepareOutDirFile(ctx, s.stdoutFilename())
	if err != nil {
		return errors.Wrap(err, "failed to open stdout log of hostapd")
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to obtain StdoutPipe of hostapd")
	}
	readyWriter := newReadyWriter()
	go func() {
		multiWriter := io.MultiWriter(s.stdoutFile, readyWriter)
		defer readyWriter.Close()
		io.Copy(multiWriter, stdoutPipe)
	}()

	if err := cmd.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start hostapd")
	}
	s.cmd = cmd

	// Wait for hostapd to get ready.
	if err := readyWriter.wait(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "hostapd started")
	return nil
}

// Close stops hostapd and cleans up related resources.
func (s *Server) Close(ctx context.Context) error {
	testing.ContextLog(ctx, "Stopping hostapd")
	if s.cmd != nil {
		s.cmd.Abort()
		// TODO(crbug.com/1030635): Abort might not work, use pkill to ensure the daemon is killed.
		s.host.Command("pkill", "-f", fmt.Sprintf("^%s.*%s", hostapdCmd, s.confPath()))

		// Skip the error in Wait as the process is aborted and always has error in wait.
		s.cmd.Wait(ctx)
		s.cmd = nil
	}
	if s.stdoutFile != nil {
		s.stdoutFile.Close()
	}
	if s.stderrFile != nil {
		s.stderrFile.Close()
	}
	if err := s.host.Command("rm", s.confPath()).Run(ctx); err != nil {
		return errors.Wrap(err, "failed to remove config")
	}
	return nil
}

// Interface returns the interface used by the hostapd.
func (s *Server) Interface() string {
	return s.iface
}

// Config returns the config used by the hostapd.
func (s *Server) Config() Config {
	return *s.conf
}
