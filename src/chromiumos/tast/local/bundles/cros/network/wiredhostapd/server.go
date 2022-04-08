// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wiredhostapd

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/hostapd"
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

// Generate is a helper to format and write out the config files, certificates,
// etc. needed by hostapd. Returns the path to which hostapd.conf was written.
func (c EAPConf) Generate(ctx context.Context, dir, ctrlPath string) (string, error) {
	// NB: 'eap_user' format is not well documented. The '[2]' indicates
	// phase 2 (i.e., inner).
	eapUser := fmt.Sprintf(`* %s
"%s" %s "%s" [2]
`, c.OuterAuth, c.Identity, c.InnerAuth, c.Password)

	serverCertPath := filepath.Join(dir, "cert")
	privateKeyPath := filepath.Join(dir, "private_key")
	eapUserFilePath := filepath.Join(dir, "eap_user")
	caCertPath := filepath.Join(dir, "ca_cert")
	confPath := filepath.Join(dir, "hostapd.conf")

	confContents := fmt.Sprintf(`driver=wired
ctrl_interface=%s
server_cert=%s
private_key=%s
eap_user_file=%s
ca_cert=%s
eap_server=1
ieee8021x=1
eapol_version=2
`, ctrlPath, serverCertPath, privateKeyPath, eapUserFilePath, caCertPath)

	for _, p := range []struct {
		path     string
		contents string
	}{
		{confPath, confContents},
		{serverCertPath, c.Cert.ServerCred.Cert},
		{privateKeyPath, c.Cert.ServerCred.PrivateKey},
		{eapUserFilePath, eapUser},
		{caCertPath, c.Cert.CACred.Cert},
	} {
		if err := ioutil.WriteFile(p.path, []byte(p.contents), 0644); err != nil {
			return "", errors.Wrapf(err, "failed to write file %q", p.path)
		}
	}

	return confPath, nil
}

// Server holds information about a started hostapd server for the 'driver=wired' variant.
type Server struct {
	hostapd.Server

	// logIndex is an internal counter, to ensure we write out unique status files.
	logIndex int
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
	stdout, stderr, err = s.CliCmd(ctx, "sta", staAddr)
	out := stdout + "\nstderr:\n" + stderr
	if err != nil {
		testing.ContextLog(ctx, "hostapd_cli output: ", out)
		return errors.Wrapf(err, "failed to query STA %q", staAddr)
	}

	// Stash output for analysis.
	path := filepath.Join(s.OutDir(), fmt.Sprintf("hostapd_auth_%d.txt", s.logIndex))
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
		stdout, stderr, err := s.CliCmd(ctx, "new_sta", nearestMAC)
		out = stdout + "\nstderr:\n" + stderr
		return err
	}, &testing.PollOptions{Timeout: 3 * time.Second}); err != nil {
		return errors.Wrapf(err, "new_sta failed, output %q", out)
	}

	return nil
}
