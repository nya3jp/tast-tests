// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wiredhostapd

import (
	"bytes"
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
	"chromiumos/tast/local/network/ip"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// STA status definitions (i.e., expectation values for Server.ExpectSTAStatus).
const (
	STAStatusAuthSuccess = "dot1xAuthAuthSuccessesWhileAuthenticating"
	STAStatusAuthLogoff  = "dot1xAuthAuthEapLogoffWhileAuthenticated"
)

// EAPConf holds configuration information for creating hostapd's internal EAP server.
type EAPConf struct {
	OuterAuth string
	InnerAuth string
	Identity  string
	Password  string

	// Cert holds certificate information for hostapd's EAP server.
	Cert *certificate.CertStore
}

// Server holds information about a started hostapd server, primarily for the 'driver=wired' variant.
type Server struct {
	// Iface is the name of the network interface which hostapd should manage.
	Iface string

	// OutDir is the path to which output logs should be written.
	OutDir string

	EAP *EAPConf

	// ctrlIface is the path to the domain socket for controlling hostapd (e.g., with hostapd_cli).
	ctrlIface string

	// tmpDir is the path where temporary files should be stashed. (Note: different than OutDir, where test
	// artifacts should be stashed.)
	tmpDir string

	cmd *testexec.Cmd

	// logIndex is an internal counter, to ensure we write out unique status files.
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

// cliCmd runs a hostapd command via hostapd_cli. Returns stdout/stderr for success or error.
func (s *Server) cliCmd(ctx context.Context, args ...string) (stdout, stderr string, err error) {
	cliArgs := append([]string{"-p", s.ctrlIface, "-i", s.Iface}, args...)
	cmd := testexec.CommandContext(ctx, "hostapd_cli", cliArgs...)

	// Don't intermix stdout/stderr for parsing, so we have to capture them separately.
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	// Not all errors are fatal for all callers, so don't use DumpLogOnError.
	if err = cmd.Run(); err != nil {
		err = errors.Wrapf(err, "hostapd_cli failed, args: %v", args)
	}
	return outBuf.String(), errBuf.String(), err
}

// ExpectSTAStatus retrieves the status for the station at 'staAddr', logs it to a file in OutDir, and
// verifies if the expected status 'key=val' entry is found.
func (s *Server) ExpectSTAStatus(ctx context.Context, staAddr, key, val string) error {
	// Log hostapd status to file with increasing index.
	defer func() {
		s.logIndex++
	}()

	var err error
	var stdout, stderr string
	stdout, stderr, err = s.cliCmd(ctx, "sta", staAddr)
	out := stdout + "\nstderr:\n" + stderr
	if err != nil {
		testing.ContextLog(ctx, "hostapd_cli output: ", out)
		return errors.Wrapf(err, "failed to query STA %q", staAddr)
	}

	// Stash output for analysis.
	path := filepath.Join(s.OutDir, fmt.Sprintf("hostapd_auth_%d.txt", s.logIndex))
	if err := ioutil.WriteFile(path, []byte(out), 0644); err != nil {
		return errors.Wrapf(err, "failed to write file %q", path)
	}

	expect := key + "=" + val
	for _, line := range strings.Split(stdout, "\n") {
		if line == expect {
			return nil
		}
	}

	return errors.Errorf("hostapd auth status %q not found", expect)
}

// prepareConfigs is a helper to format and write out the config files, certificates, etc., needed by hostapd.
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
		{serverCertPath, s.EAP.Cert.ServerCred.Cert},
		{privateKeyPath, s.EAP.Cert.ServerCred.PrivateKey},
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

	confPath, err := s.prepareConfigs(ctx)
	if err != nil {
		return err
	}

	// Bring up the hostapd link.
	ipr := ip.NewLocalRunner()
	if err := ipr.SetLinkUp(ctx, s.Iface); err != nil {
		return errors.Wrap(err, "could not bring up hostapd veth")
	}

	logPath := filepath.Join(s.OutDir, "hostapd.log")
	s.cmd = testexec.CommandContext(ctx, "hostapd", "-dd", "-t", "-f", logPath, confPath)
	s.cmd.Env = append(os.Environ(), "OPENSSL_CONF=/etc/ssl/openssl.cnf.compat")
	if err := s.cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start hostapd")
	}

	return nil
}

// StartEAPOL tells the Server to initiate EAPOL with nearby links (multicast).
func (s *Server) StartEAPOL(ctx context.Context) error {
	// The default group MAC address to which EAP challenges are sent, absent any prior
	// knowledge of a specific client on the link -- part of the Link Layer Discovery Protocol
	// (LLDP), IEEE 802.1AB.
	const nearestMAC = "01:80:c2:00:00:03"

	var out string
	// Poll because we didn't guarantee the hostapd server has finished starting up (e.g.,
	// establishing the control socket).
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		stdout, stderr, err := s.cliCmd(ctx, "new_sta", nearestMAC)
		out = stdout + "\nstderr:\n" + stderr
		return err
	}, &testing.PollOptions{Timeout: 3 * time.Second}); err != nil {
		return errors.Wrapf(err, "new_sta failed, output %q", out)
	}

	return nil
}
