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
	// Iface is the name of the network interface which hostapd should manage.
	Iface string

	// OutDir is the path to which output logs should be written.
	OutDir string

	// Server configuration
	Conf Conf

	// ctrlIface is the path to the domain socket for controlling hostapd (e.g., with hostapd_cli).
	ctrlIface string

	// tmpDir is the path where temporary files should be stashed. (Note: different than OutDir, where test
	// artifacts should be stashed.)
	tmpDir string

	cmd *testexec.Cmd
}

// Start starts up a hostapd instance. The caller should call Server.Stop() when
// finished.
func (s *Server) Start(ctx context.Context) (retErr error) {
	if s.Conf == nil {
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

	confPath, err := s.Conf.Prepare(ctx, s.tmpDir, s.ctrlIface)
	if err != nil {
		return err
	}

	// Bring up the hostapd link.
	ipr := ip.NewLocalRunner()
	if err := ipr.SetLinkUp(ctx, s.Iface); err != nil {
		return errors.Wrapf(err, "could not bring up hostapd interface: %s", s.Iface)
	}

	logPath := filepath.Join(s.OutDir, "hostapd.log")
	s.cmd = testexec.CommandContext(ctx, "hostapd", "-dd", "-i", s.Iface,
		"-t", "-f", logPath, confPath)
	s.cmd.Env = append(os.Environ(), "OPENSSL_CONF=/etc/ssl/openssl.cnf.compat")
	if err := s.cmd.Start(); err != nil {
		return errors.Wrapf(err, "failed to start hostapd(%s)", s.Iface)
	}

	return nil
}

// Stop cleans up any resources and kills the hostapd server.
func (s *Server) Stop(ctx context.Context) error {
	if err := s.cmd.Signal(unix.SIGTERM); err != nil {
		return errors.Wrapf(err, "failed to kill hostapd(%s)", s.Iface)
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
	cliArgs := append([]string{"-p", s.ctrlIface, "-i", s.Iface}, args...)

	// Don't intermix stdout/stderr for parsing, so we have to capture them separately.
	o, e, err := testexec.CommandContext(ctx, "hostapd_cli", cliArgs...).SeparatedOutput()
	if err != nil {
		err = errors.Wrapf(err, "hostapd_cli failed, args: %v", args)
	}
	return string(o), string(e), err
}

// ListSta lists the stations associated with the server.
func (s *Server) ListSta(ctx context.Context) ([]string, error) {
	var stas []string

	o, e, err := s.CliCmd(ctx, "list_sta")
	if err != nil {
		return stas, errors.Wrapf(err, "hostapd_cli list_cla command failed: %s", e)
	}
	return strings.Split(o, "\n"), nil
}
