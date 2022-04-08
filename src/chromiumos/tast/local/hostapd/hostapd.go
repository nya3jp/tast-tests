// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hostapd

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/network/ip"
	"chromiumos/tast/testing"
)

// configGenerator is an interface for a specific hostapd configuration.
type configGenerator interface {
	// Generate creates all the hostapd files required by the current
	// configuration in dir. Returns the path to the main configuration
	// file or an error.
	Generate(ctx context.Context, dir, ctrlPath string) (string, error)
}

// Server holds information about a started hostapd server.
type Server struct {
	// iface is the name of the network interface which hostapd should manage.
	iface string

	// outDir is the path to which output logs should be written.
	outDir string

	// conf is the generator of the server configuration.
	conf configGenerator

	// ctrlSocketPath is the path to the domain socket for controlling hostapd
	// (e.g., with hostapd_cli).
	ctrlSocketPath string

	// tmpDir is the path where temporary files should be stashed. (Note:
	// different than OutDir, where test artifacts should be stashed.)
	tmpDir string

	cmd *testexec.Cmd
}

// NewServer creates a new server instance.
func NewServer(iface, outDir string, conf configGenerator) *Server {
	if conf == nil {
		panic("a server requires a configuration")
	}
	return &Server{
		iface:  iface,
		outDir: outDir,
		conf:   conf,
	}
}

// OutDir returns the path to the directory that will receive the output logs.
func (s *Server) OutDir() string {
	return s.outDir
}

// Start starts up a hostapd instance. The caller should call Server.Stop() when
// finished.
func (s *Server) Start(ctx context.Context) (retErr error) {
	var err error
	if s.tmpDir, err = ioutil.TempDir("", ""); err != nil {
		return errors.Wrap(err, "failed to create a temporary directory")
	}
	defer func() {
		if retErr != nil {
			if err := os.RemoveAll(s.tmpDir); err != nil {
				testing.ContextLogf(ctx, "Failed to clean up dir %s, %v", s.tmpDir, err)
			}
		}
	}()

	s.ctrlSocketPath = filepath.Join(s.tmpDir, "ctrl_iface")

	confPath, err := s.conf.Generate(ctx, s.tmpDir, s.ctrlSocketPath)
	if err != nil {
		return err
	}

	// Bring up the hostapd link.
	ipr := ip.NewLocalRunner()
	if err := ipr.SetLinkUp(ctx, s.iface); err != nil {
		return errors.Wrapf(err, "could not bring up hostapd interface: %s", s.iface)
	}

	logPath := filepath.Join(s.outDir, "hostapd.log")
	s.cmd = testexec.CommandContext(ctx, "hostapd",
		"-dd",         // enable debug logs.
		"-i", s.iface, // specify the Wi-Fi interface to use.
		"-t",          // include timestamps in log messages.
		"-f", logPath, // output logs to a file.
		confPath)
	s.cmd.Env = append(os.Environ(), "OPENSSL_CONF=/etc/ssl/openssl.cnf.compat")
	if err := s.cmd.Start(); err != nil {
		return errors.Wrapf(err, "failed to start hostapd(%s)", s.iface)
	}

	return nil
}

// Stop cleans up any resources and kills the hostapd server.
func (s *Server) Stop() error {
	if err := s.cmd.Signal(unix.SIGTERM); err != nil {
		return errors.Wrapf(err, "failed to kill hostapd(%s)", s.iface)
	}
	// Wait will always fail; ignore errors.
	if err := s.cmd.Wait(); err != nil {
		return errors.Wrap(err, "failed to wait for hostapd process")
	}
	if err := os.RemoveAll(s.tmpDir); err != nil {
		return errors.Wrapf(err, "failed to clean up tmp dir: %s", s.tmpDir)
	}
	return nil
}

// CliCmd runs a hostapd command via hostapd_cli. Returns stdout/stderr for success or error.
func (s *Server) CliCmd(ctx context.Context, args ...string) (stdout, stderr string, err error) {
	cliArgs := append([]string{"-p", s.ctrlSocketPath, "-i", s.iface}, args...)

	// Don't intermix stdout/stderr for parsing, so we have to capture them separately.
	o, e, err := testexec.CommandContext(ctx, "hostapd_cli", cliArgs...).SeparatedOutput()
	if err != nil {
		err = errors.Wrapf(err, "hostapd_cli failed, args: %v", args)
	}
	return string(o), string(e), err
}

// ListStations lists the stations associated with the server.
func (s *Server) ListStations(ctx context.Context) ([]string, error) {
	o, e, err := s.CliCmd(ctx, "list_sta")
	if err != nil {
		return nil, errors.Wrapf(err, "hostapd_cli list_sta command failed: %s", e)
	}
	return strings.Split(o, "\n"), nil
}
