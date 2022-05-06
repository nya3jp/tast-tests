// Copyright 2022 The Chromium OS Authors. All rights reserved.
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

// Server holds information about a started hostapd server.
type Server struct {
	// iface is the name of the network interface which hostapd should manage.
	iface string

	// outDir is the path to which output logs should be written.
	outDir string

	// conf is the server configuration.
	conf Conf

	// ctrlIface is the path to the domain socket for controlling hostapd (e.g., with hostapd_cli).
	ctrlIface string

	// tmpDir is the path where temporary files should be stashed. (Note: different than OutDir, where test
	// artifacts should be stashed.)
	tmpDir string

	cmd *testexec.Cmd
}

// NewServer creates a new server instance.
func NewServer(iface, outDir string, conf Conf) *Server {
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
	if s.conf == nil {
		return errors.New("failed to start hostapd: no configuration provided")
	}

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

	s.ctrlIface = filepath.Join(s.tmpDir, "ctrl_iface")

	confPath, err := s.conf.Prepare(ctx, s.tmpDir, s.ctrlIface)
	if err != nil {
		return err
	}

	// Bring up the hostapd link.
	ipr := ip.NewLocalRunner()
	if err := ipr.SetLinkUp(ctx, s.iface); err != nil {
		return errors.Wrapf(err, "could not bring up hostapd interface: %s", s.iface)
	}

	logPath := filepath.Join(s.outDir, "hostapd.log")
	s.cmd = testexec.CommandContext(ctx, "hostapd", "-dd", "-i", s.iface,
		"-t", "-f", logPath, confPath)
	s.cmd.Env = append(os.Environ(), "OPENSSL_CONF=/etc/ssl/openssl.cnf.compat")
	if err := s.cmd.Start(); err != nil {
		return errors.Wrapf(err, "failed to start hostapd(%s)", s.iface)
	}

	return nil
}

// Stop cleans up any resources and kills the hostapd server.
func (s *Server) Stop(ctx context.Context) error {
	if err := s.cmd.Signal(unix.SIGTERM); err != nil {
		return errors.Wrapf(err, "failed to kill hostapd(%s)", s.iface)
	}
	// Wait will always fail; ignore errors.
	s.cmd.Wait()
	if err := os.RemoveAll(s.tmpDir); err != nil {
		return errors.Wrapf(err, "failed to clean up tmp dir: %s", s.tmpDir)
	}
	return nil
}

// CliCmd runs a hostapd command via hostapd_cli. Returns stdout/stderr for success or error.
func (s *Server) CliCmd(ctx context.Context, args ...string) (stdout, stderr string, err error) {
	cliArgs := append([]string{"-p", s.ctrlIface, "-i", s.iface}, args...)

	// Don't intermix stdout/stderr for parsing, so we have to capture them separately.
	o, e, err := testexec.CommandContext(ctx, "hostapd_cli", cliArgs...).SeparatedOutput()
	if err != nil {
		err = errors.Wrapf(err, "hostapd_cli failed, args: %v", args)
	}
	return string(o), string(e), err
}

// ListSta lists the stations associated with the server.
func (s *Server) ListSta(ctx context.Context) ([]string, error) {
	o, e, err := s.CliCmd(ctx, "list_sta")
	if err != nil {
		return nil, errors.Wrapf(err, "hostapd_cli list_cla command failed: %s", e)
	}
	return strings.Split(o, "\n"), nil
}
