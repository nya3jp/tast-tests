// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hostapd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"sync"

	"chromiumos/tast/common/network/daemonutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/fileutil"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	hostapdCmd = "hostapd"
)

// KillAll kills all running hostapd on host, useful for environment setup/cleanup.
func KillAll(ctx context.Context, host *ssh.Conn) error {
	return host.Command("killall", hostapdCmd).Run(ctx)
}

// Server controls a hostapd on router.
type Server struct {
	host    *ssh.Conn // TODO(crbug.com/1019537): use a more suitable ssh object.
	name    string
	iface   string
	workDir string
	conf    *Config

	cmd        *ssh.Cmd
	wg         sync.WaitGroup
	stdoutFile *os.File
	stderrFile *os.File
}

// StartServer creates a new Server object and runs hostapd on iface of the given host with settings
// specified in config. workDir is the dir on host for the server to put temporary files.
// name is the identifier used for log filenames in OutDir.
func StartServer(ctx context.Context, host *ssh.Conn, name, iface, workDir string, config *Config) (server *Server, err error) {
	ctx, st := timing.Start(ctx, "hostapd.StartServer")
	defer st.End()

	s := &Server{
		host:    host,
		name:    name,
		iface:   iface,
		workDir: workDir,
		conf:    config,
	}
	// Clean up on error.
	defer func() {
		if err != nil {
			// Close the Server instance created above, not the returned one as it might be nil.
			s.Close(ctx)
		}
	}()

	if err := s.initConfig(ctx); err != nil {
		return nil, err
	}
	if err := s.start(ctx); err != nil {
		return nil, err
	}
	return s, nil
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

// initConfig writes a hostapd config file.
func (s *Server) initConfig(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "initConfig")
	defer st.End()

	conf, err := s.conf.Format(s.iface, s.ctrlPath())
	if err != nil {
		return err
	}
	if err := fileutil.WriteToHost(ctx, s.host, s.confPath(), []byte(conf)); err != nil {
		return errors.Wrap(err, "failed to write config")
	}
	return nil
}

// start spawns a hostapd daemon and waits until it is ready.
func (s *Server) start(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "start")
	defer st.End()

	testing.ContextLogf(ctx, "Starting hostapd %s on interface %s", s.name, s.iface)
	cmd := s.host.Command(hostapdCmd, "-dd", "-t", s.confPath())
	// Prepare stdout/stderr log files.
	var err error
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
	readyFunc := func(buf []byte) (bool, error) {
		if bytes.Contains(buf, []byte("Interface initialization failed")) {
			return false, errors.New("hostapd failed to initialize AP interface")
		} else if bytes.Contains(buf, []byte("Setup of interface done")) {
			return true, nil
		}
		return false, nil
	}

	// Wait for hostapd to get ready.
	readyWriter := daemonutil.NewReadyWriter(readyFunc)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer stdoutPipe.Close()
		defer readyWriter.Close()
		multiWriter := io.MultiWriter(s.stdoutFile, readyWriter)
		io.Copy(multiWriter, stdoutPipe)
	}()

	if err := cmd.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start hostapd")
	}
	s.cmd = cmd

	// Wait for hostapd to get ready.
	if err := readyWriter.Wait(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "hostapd started")
	return nil
}

// Close stops hostapd and cleans up related resources.
func (s *Server) Close(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "hostapd.Close")
	defer st.End()

	testing.ContextLog(ctx, "Stopping hostapd")
	if s.cmd != nil {
		s.cmd.Abort()
		// TODO(crbug.com/1030635): Abort might not work, use pkill to ensure the daemon is killed.
		s.host.Command("pkill", "-f", fmt.Sprintf("^%s.*%s", hostapdCmd, s.confPath())).Run(ctx)

		// Skip the error in Wait as the process is aborted and always has error in wait.
		s.cmd.Wait(ctx)
		s.cmd = nil
	}
	// Wait the bg routine to end before closing files.
	s.wg.Wait()
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
