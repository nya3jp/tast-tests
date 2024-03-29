// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package certs provides the utils for installation of trusted certification authorities
// for tests, and for using server certificates issued by such a trusted test authority.
// Specifically, this package allows installation of a certificate authority from
// |certs| in a |certDirectory|. This allows clients on DUTs to successfully verify
// test certs used by test servers set up in a local tast test or test fixture.
// This package provides no utils for client certs.
//
// WARNING: After calling InstallTestCerts(), the default set of trusted CAs is not
// active, so HTTPS communication to publicly-trusted web hosts will not be possible.
//
// InstallTestCerts() will do the following:
// 1. Remount the tmp directory to allow symlinks to be followed.
// 2. Write CA cert (|cert|) to a tmp directory, rehash, and mount it to |certDirectory|.
// 3. Write server cert and private key to a tmp directory.
//
// Cleanup func returned from InstallTestCerts() will remove the mount on |certDirectory|
// and delete the certs in the tmp directory.
package certs

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

var (
	tmpPath            = filepath.Join("/tmp")
	tmpCertsPath       = filepath.Join(tmpPath, "test_certs")
	serverPath         = filepath.Join(tmpCertsPath, "server")
	caPath             = filepath.Join(tmpCertsPath, "ca")
	serverKeyFilePath  = filepath.Join(serverPath, "server_key.key")
	serverCertFilePath = filepath.Join(serverPath, "server_cert.crt")
	caCertFilePath     = filepath.Join(caPath, "ca_cert.pem")
	caCertsFilePath    = filepath.Join(caPath, "ca-certificates.crt")
)

// Default path in root to bind mount to for cert validation
const (
	SSLCrtPath = "/etc/ssl/certs"
)

// Certs holds the default cert directory and a cert store.
type Certs struct {
	// certDirectory is the directory of the default certs (contains CA certificates
	// of authorities trusted by default). This will be hidden when the test certs
	// are mounted on top of it.
	certDirectory string
	// certStore contains the server cert, server private key, and ca cert for install.
	certStore certificate.CertStore
}

// New creates a new certs object that contains the |certDirectory| where the test
// certs will be mounted to and the |certStore| which contains the certs that will
// be installed.
func New(certDirectory string, certstore certificate.CertStore) *Certs {
	return &Certs{
		certDirectory: certDirectory,
		certStore:     certstore,
	}
}

// InstallTestCerts installs and writes certs that can be used by a HTTPS server.
// In order to install certs in tmp, tmp must be remounted to allow symlinks.
// Cleanup func returned will restore tmp directory to previous state.
//
// WARNING: After calling InstallTestCerts(), the default set of trusted CAs is not
// active, so HTTPS communication to publicly-trusted web hosts will not be possible.
func (c *Certs) InstallTestCerts(ctx context.Context) (func(context.Context), error) {
	success := false
	cleanup := func(ctx context.Context) {
		if err := testexec.CommandContext(ctx, "umount", c.certDirectory).Run(); err != nil {
			testing.ContextLog(ctx, "Failed to unmount bind: ", err)
		}

		if err := testexec.CommandContext(ctx, "rm", "-rf", tmpCertsPath).Run(); err != nil {
			testing.ContextLog(ctx, "Failed to delete tmp directory: ", err)
		}

		if err := testexec.CommandContext(ctx, "mount", "-o", "remount,nosymfollow", tmpPath).Run(); err != nil {
			testing.ContextLogf(ctx, "Failed to change mount options on %s back to default: %s", tmpPath, err)
		}
	}

	defer func() {
		if !success {
			cleanup(ctx)
		}
	}()

	if err := testexec.CommandContext(ctx, "mount", "-o", "remount,symfollow", tmpPath).Run(); err != nil {
		return nil, errors.Wrapf(err, "failed change mount options on: %s", tmpPath)
	}

	if err := testexec.CommandContext(ctx, "mkdir", tmpCertsPath).Run(); err != nil {
		return nil, errors.Wrapf(err, "failed to make tmp directory: %s", tmpCertsPath)
	}

	testing.ContextLog(ctx, "Installing CA certs to tmp directory for TLS validation")
	if err := c.installTestCertificateAuthorityCert(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to set up temp CA certs in certDirectory: %s", c.certDirectory)
	}

	testing.ContextLog(ctx, "Writing server certs to tmp directory")
	if err := c.writeTestServerCertAndPrivateKey(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to write temp server certs")
	}
	success = true
	return cleanup, nil
}

// writeTestServerCertAndPrivateKey writes the server certificate and the server private key to a tmp directory for use in ServeTLS().
func (c *Certs) writeTestServerCertAndPrivateKey(ctx context.Context) error {
	if err := testexec.CommandContext(ctx, "mkdir", serverPath).Run(); err != nil {
		return errors.Wrap(err, "failed to make server directory")
	}

	if err := os.WriteFile(c.GetTestServerCertFilePath(), []byte(c.certStore.ServerCred.Cert), 0644); err != nil {
		return errors.Wrap(err, "failed to write server cert")
	}

	if err := os.WriteFile(c.GetTestServerKeyFilePath(), []byte(c.certStore.ServerCred.PrivateKey), 0644); err != nil {
		return errors.Wrap(err, "failed to write server private key")
	}
	return nil
}

// installTestCertificateAuthorityCert writes the ca |cert| to a tmp directory,
// rehashes the directory, and then mounts the tmp directory to |certDirectory|.
// After this is called, the default certs in |certDirectory| are hidden. Cleanup
// func from InstallTestCerts() will remove the mount and make the default certs
// visible again.
func (c *Certs) installTestCertificateAuthorityCert(ctx context.Context) error {
	if err := testexec.CommandContext(ctx, "mkdir", caPath).Run(); err != nil {
		return errors.Wrap(err, "failed to make ca directory")
	}

	if err := os.WriteFile(caCertFilePath, []byte(c.certStore.CACred.Cert), 0644); err != nil {
		return errors.Wrap(err, "failed to write ca cert")
	}

	if err := testexec.CommandContext(ctx, "c_rehash", caPath).Run(); err != nil {
		return errors.Wrap(err, "failed to rehash tmp cert directory")
	}

	// CURL by default expects a file containing all available CA certificates for HTTPS requests.
	if err := os.WriteFile(caCertsFilePath, []byte(c.certStore.CACred.Cert), 0644); err != nil {
		return errors.Wrap(err, "failed to write ca cert")
	}

	if err := testexec.CommandContext(ctx, "mount", "-o", "bind", caPath, c.certDirectory).Run(); err != nil {
		return errors.Wrapf(err, "failed to bind mount, caPath: %s, target: %s", caPath, c.certDirectory)
	}
	return nil
}

// GetTestServerCertFilePath returns the path to the tmp server cert file.
func (c *Certs) GetTestServerCertFilePath() string {
	return serverCertFilePath
}

// GetTestServerKeyFilePath returns the path to the tmp server private key file.
func (c *Certs) GetTestServerKeyFilePath() string {
	return serverKeyFilePath
}
