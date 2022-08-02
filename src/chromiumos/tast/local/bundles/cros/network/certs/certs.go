// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package certs provides the utils for certificate installation
package certs

import (
	"context"
	"os"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	tmpCrtPath    = "/tmp/test_certs"
	serverKeyPath = tmpCrtPath + "/server_key.key"
	serverCrtPath = tmpCrtPath + "/server_cert.crt"
	caCrtPath     = tmpCrtPath + "/ca_cert.pem"
)

// InstallCerts installs and writes certs for HTTPS server.
func InstallCerts(ctx context.Context, certs certificate.CertStore, certDirectory string) error {
	testing.ContextLog(ctx, "Installing CA certs to tmp directory for TLS validation")
	if err := installSSLCerts(ctx, certs.CACred.Cert, certDirectory); err != nil {
		return errors.Wrap(err, "failed to set up temp CA certs")
	}

	testing.ContextLog(ctx, "Writing server certs to tmp directory")
	if err := writeServerCerts(ctx, certs.ServerCred); err != nil {
		return errors.Wrap(err, "failed to write temp server certs")
	}
	return nil
}

// writeServerCerts writes the server certificate and the server private key to a
// tmp directory for use in ServeTLS().
func writeServerCerts(ctx context.Context, serverCreds certificate.Credential) error {
	if err := os.WriteFile(serverCrtPath, []byte(serverCreds.Cert), 0644); err != nil {
		return errors.Wrap(err, "failed to write server cert")
	}
	if err := os.WriteFile(serverKeyPath, []byte(serverCreds.PrivateKey), 0644); err != nil {
		return errors.Wrap(err, "failed to write server private key")
	}
	return nil
}

// installSSLCerts writes the ca |cert| to a tmp directory, rehashes the directory,
// and then mounts the tmp directory to |certDirectory|.
func installSSLCerts(ctx context.Context, cert, certDirectory string) error {
	if err := testexec.CommandContext(ctx, "mkdir", tmpCrtPath).Run(); err != nil {
		return errors.Wrap(err, "failed to make tmp directory")
	}

	if err := os.WriteFile(caCrtPath, []byte(cert), 0644); err != nil {
		return errors.Wrap(err, "failed to write ca cert")
	}

	if err := testexec.CommandContext(ctx, "c_rehash", tmpCrtPath).Run(); err != nil {
		return errors.Wrap(err, "failed to rehash tmp cert directory")
	}

	if err := testexec.CommandContext(ctx, "mount", "-o", "bind", tmpCrtPath, certDirectory).Run(); err != nil {
		return errors.Wrap(err, "failed to bind mount")
	}
	return nil
}

// UninstallCerts unmounts |certDirectory| and deletes the tmp directory that contained
// the CA cert, server cert, and server private key.
func UninstallCerts(ctx context.Context, certDirectory string) {
	if err := testexec.CommandContext(ctx, "umount", certDirectory).Run(); err != nil {
		testing.ContextLog(ctx, "Failed to unmount bind: ", err)
	}
	if err := testexec.CommandContext(ctx, "rm", "-rf", tmpCrtPath).Run(); err != nil {
		testing.ContextLog(ctx, "Failed to delete tmp directory: ", err)
	}
}

// GetServerCertPath returns the path to the tmp server cert file
func GetServerCertPath() string {
	return serverCrtPath
}

// GetServerKeyPath returns the path to the tmp server private key file
func GetServerKeyPath() string {
	return serverKeyPath
}
