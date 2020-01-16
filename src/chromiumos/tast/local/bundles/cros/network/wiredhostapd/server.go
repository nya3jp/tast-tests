// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wiredhostapd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// STA status definitions (i.e., expectation values for Server.ExpectSTAStatus).
const (
	STAStatusAuthSucess = "dot1xAuthAuthSuccessesWhileAuthenticating"
	STAStatusAuthLogoff = "dot1xAuthAuthEapLogoffWhileAuthenticated"
)

// EAPConf holds configuration information for creating hostapd's internal EAP server.
type EAPConf struct {
	OuterAuth string
	InnerAuth string
	Identity  string
	Password  string

	// cert holds certificate information for hostapd's EAP server.
	Cert certificate.Certificate
}

// Server holds information about a started hostapd server, primarily for the 'driver=wired' variant.
type Server struct {
	// iface is the name of the network interface which hostapd should manage.
	Iface string

	// Path to which output logs should be written.
	OutDir string

	EAP *EAPConf

	// ctrlIface is the path to the domain socket for controlling hostapd (e.g., with hostapd_cli).
	ctrlIface string

	// Temporary files should be stashed in this path. (Note: different than OutDir, where test artifacts
	// should be stashed.)
	tmpDir string

	cmd *testexec.Cmd

	// internal counter, to ensure we write out unique status files
	logIndex int
}

// Stop cleans up any resources and kills the hostapd server.
func (s *Server) Stop(ctx context.Context) error {
	if err := s.cmd.Signal(unix.SIGTERM); err != nil {
		return errors.Wrap(err, "failed to kill hostapd")
	}
	// Wait will always fail; ignore errors.
	s.cmd.Wait()
	if err := os.RemoveAll(s.tmpDir); err != nil {
		return errors.Wrapf(err, "failed to clean up tmp dir: %s", s.tmpDir)
	}
	return nil
}

// cliCmd runs a hostapd command via hostapd_cli. Returns combined stdout/stderr for success or error.
func (s *Server) cliCmd(ctx context.Context, args ...string) (string, error) {
	cliArgs := append([]string{"-p", s.ctrlIface, "-i", s.Iface}, args...)
	out, err := testexec.CommandContext(ctx, "hostapd_cli", cliArgs...).CombinedOutput()
	if err != nil {
		return string(out), errors.Wrapf(err, "hostapd_cli failed, args: %v", args)
	}

	return string(out), nil
}

// ExpectSTAStatus retrieves the status for the station at 'staAddr', logs it to a file in OutDir, and
// verifies if the expected status 'key=val' entry is found.
func (s *Server) ExpectSTAStatus(ctx context.Context, staAddr string, key string, val string) error {
	// Log hostapd status to file with increasing index.
	defer func() {
		s.logIndex++
	}()

	var err error
	var out string
	if out, err = s.cliCmd(ctx, "sta", staAddr); err != nil {
		testing.ContextLog(ctx, "hostapd_cli output: ", out)
		return errors.Wrapf(err, "failed to query STA %q", staAddr)
	}

	// Stash output for analysis.
	path := filepath.Join(s.OutDir, fmt.Sprintf("hostapd_auth_%d.txt", s.logIndex))
	if err := ioutil.WriteFile(path, []byte(out), 0644); err != nil {
		return errors.Wrapf(err, "failed to write file %q", path)
	}

	expect := key + "=" + val
	for _, line := range strings.Split(out, "\n") {
		if line == expect {
			return nil
		}
	}

	return errors.Errorf("hostapd auth status %q not found", expect)
}

// prepareConfigs is a helper to format and write out the config files, certificats, etc., needed by hostapd.
// Returns the path to which hostapd.conf was written.
func (s *Server) prepareConfigs(ctx context.Context) (string, error) {
	// NB: 'eap_user' format is not well documented. The '[2]' indicates
	// phase 2 (i.e., inner).
	eapUser := fmt.Sprintf(`* %s
"%s" %s "%s" [2]
`, s.EAP.OuterAuth, s.EAP.Identity, s.EAP.InnerAuth, s.EAP.Password)

	serverCertPath := filepath.Join(s.tmpDir, "cert")
	privateKeyPath := filepath.Join(s.tmpDir, "private_key")
	eapUserFilePath := filepath.Join(s.tmpDir, "eap_user")
	caCertPath := filepath.Join(s.tmpDir, "ca_cert")
	confPath := filepath.Join(s.tmpDir, "hostapd.conf")
	s.ctrlIface = filepath.Join(s.tmpDir, "hostapd.ctrl")

	confContents := fmt.Sprintf(`driver=wired
interface=%s
ctrl_interface=%s
server_cert=%s
private_key=%s
eap_user_file=%s
ca_cert=%s
eap_server=1
ieee8021x=1
eapol_version=2
`, s.Iface, s.ctrlIface, serverCertPath, privateKeyPath, eapUserFilePath, caCertPath)

	for _, p := range []struct {
		path     string
		contents string
	}{
		{confPath, confContents},
		{serverCertPath, s.EAP.Cert.Cert},
		{privateKeyPath, s.EAP.Cert.PrivateKey},
		{eapUserFilePath, eapUser},
		{caCertPath, s.EAP.Cert.CACert},
	} {
		if err := ioutil.WriteFile(p.path, []byte(p.contents), 0644); err != nil {
			return "", errors.Wrapf(err, "failed to write file %q", p.path)
		}
	}

	return confPath, nil
}

// Start starts up a hostapd instance, for wired authentication. The caller should call
// Server.Stop() when finished.
func (s *Server) Start(ctx context.Context) error {
	succeeded := false

	var err error
	if s.tmpDir, err = ioutil.TempDir("", ""); err != nil {
		return errors.Wrap(err, "failed to create temp dir")
	}
	defer func() {
		if !succeeded {
			if err := os.RemoveAll(s.tmpDir); err != nil {
				testing.ContextLogf(ctx, "Failed to clean up dir %s, %v", s.tmpDir, err)
			}
		}
	}()

	confPath, err := s.prepareConfigs(ctx)
	if err != nil {
		return err
	}

	// Bring up the hostapd link.
	if err := testexec.CommandContext(ctx, "ip", "link", "set", s.Iface, "up").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "could not bring up hostapd veth")
	}

	logPath := filepath.Join(s.OutDir, "hostapd.log")
	s.cmd = testexec.CommandContext(ctx, "hostapd", "-dd", "-t", "-f", logPath, confPath)
	if err := s.cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start hostapd")
	}

	succeeded = true
	return nil
}

// StartEAPOL tells the Server to initiate EAPOL with nearby links (multicast).
func (s *Server) StartEAPOL(ctx context.Context) error {
	// The default group MAC address to which EAP challenges are sent, absent any prior
	// knowledge of a specific client on the link -- part of the Link Layer Discovery Protocol
	// (LLDP), IEEE 802.1AB.
	const nearestMAC = "01:80:c2:00:00:03"

	var out string
	var err error
	// Poll because we didn't guarantee the hostapd server has finished starting up (e.g.,
	// establishing the control socket).
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err = s.cliCmd(ctx, "new_sta", nearestMAC)
		return err
	}, &testing.PollOptions{Timeout: 3 * time.Second}); err != nil {
		return errors.Wrapf(err, "new_sta failed, output %q", out)
	}

	return nil
}
