// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// TPMStore is a struct that contains the information related to using the TPM to store certificates/keys during the test. Strictly speaking, this struct holds the information required to access a chaps slot/token.
// Note that TPMStore is currently a singleton because users of this struct (WiFi tests) only need one TPMStore at a moment.
type TPMStore struct {
	// runner is a command runner interface for executing commands on the DUT.
	runner hwsec.CmdRunner

	// slot is the PKCS#11 slot in which this user slot is located.
	slot int

	// label is the label of the PKCS#11 slot.
	label string

	// pin is the pin to access the PKCS#11 slot.
	pin string
}

const (
	// TestUsername is the username of our test user whose vault will host the WiFi keys.
	TestUsername = "test@example.com"
	// TestPassword is the password to the user above.
	TestPassword = "not_a_real_password"
	// TestKeyLabel is the key label for the vault key that belongs to the user above.
	TestKeyLabel = "password"
	// Note that the credentials above are made up and isn't actual confidential information.

	// OpenSSLCommand is the openssl command.
	OpenSSLCommand = "openssl"
	// PKCS11ReplayCommand is the p11_replay command.
	PKCS11ReplayCommand = "p11_replay"
	// ConvertTypeRSA is the conversion command passed to the openssl command for RSA Private Key.
	ConvertTypeRSA = "rsa"
	// ConvertTypeX509 is the conversion command passed to the openssl command for X.509 certificate.
	ConvertTypeX509 = "x509"
	// OutputTypePrivateKey is the --type parameter that is passed to the openssl command for private key type.
	OutputTypePrivateKey = "privkey"
	// OutputTypeCertificate is the --type parameter that is passed to the openssl command for certificate.
	OutputTypeCertificate = "cert"
)

// cleanupVault removes the vault belonging to the test user.
func cleanupVault(ctx context.Context, cryptohomeUtil *hwsec.UtilityCryptohomeBinary) error {
	if _, err := cryptohomeUtil.Unmount(ctx, TestUsername); err != nil {
		return errors.Wrap(err, "failed to unmount")
	}
	if _, err := cryptohomeUtil.RemoveVault(ctx, TestUsername); err != nil {
		return errors.Wrap(err, "failed to remove vault")
	}
	return nil
}

// singletonTPMStore is the singleton instance of TPMStore in use.
var singletonTPMStore *TPMStore

// NewTPMStore sets up a TPMStore for WiFi testing.
func NewTPMStore(ctx context.Context, cryptohomeUtil *hwsec.UtilityCryptohomeBinary, runner hwsec.CmdRunner) (result *TPMStore, retErr error) {
	if singletonTPMStore != nil {
		return nil, errors.New("another instance of TPMStore already exists")
	}

	// Remove the vaults first before the test so we can be sure that the TPM Store returned is empty.
	if err := cleanupVault(ctx, cryptohomeUtil); err != nil {
		return nil, errors.Wrap(err, "failed to cleanup vault at the beginning of SetupTPMStore")
	}

	// Now create the vault.
	if err := cryptohomeUtil.MountVault(ctx, TestUsername, TestPassword, TestKeyLabel, true); err != nil {
		return nil, errors.Wrap(err, "failed to mount vault")
	}
	defer func() {
		// If this function failed, we'll need to cleanup the vault.
		if retErr != nil {
			cleanupVault(ctx, cryptohomeUtil)
		}
	}()

	// Wait for the slot to be available.
	if err := cryptohomeUtil.WaitForUserToken(ctx, TestUsername); err != nil {
		return nil, errors.Wrap(err, "failed to wait for user token")
	}

	// Get the slot.
	label, pin, slot, err := cryptohomeUtil.GetTokenInfoForUser(ctx, TestUsername)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user token")
	}

	singletonTPMStore = &TPMStore{runner, slot, label, pin}
	return singletonTPMStore, nil
}

// ResetTPMStore resets the environment (chaps keystore and cryptohome vault) back to the state before the TPMStore instance is created.
func ResetTPMStore(ctx context.Context, cryptohomeUtil *hwsec.UtilityCryptohomeBinary) error {
	if singletonTPMStore == nil {
		return errors.New("singleton TPMStore is nil")
	}
	singletonTPMStore.slot = -1
	singletonTPMStore.runner = nil
	singletonTPMStore = nil
	return cleanupVault(ctx, cryptohomeUtil)
}

// Slot return the slot number to access the PKCS#11 slot/token for testing.
func (s *TPMStore) Slot() int {
	return s.slot
}

// Pin return the pin to access the PKCS#11 slot/token for testing.
func (s *TPMStore) Pin() string {
	return s.pin
}

// Label return the label of the PKCS#11 slot/token for testing.
func (s *TPMStore) Label() string {
	return s.label
}

// installObject install a pem format object (cert or key) into the chaps keystore.
// The object will be identified by identifier in the chaps keystore.
// conversionType should be either ConvertTypeRSA or ConvertTypeX509 depending on if the object is an RSA Private Key or X.509 certificate.
// outputType should be either OutputTypePrivateKey or OutputTypeCertificate depending on if the object is a private key or certificate..
func (s *TPMStore) installObject(ctx context.Context, pem, identifier, conversionType, outputType string) (retErr error) {
	// Get a temp file name for PEM file and DER file.
	pemFilePath := filepath.Join("/tmp", "wifitest_"+uuid.New().String())
	derFilePath := filepath.Join("/tmp", "wifitest_"+uuid.New().String())

	// Write the PEM file to disk.
	if _, err := s.runner.Run(ctx, "sh", "-c", fmt.Sprintf("echo %q > %q", pem, pemFilePath)); err != nil {
		return errors.Wrap(err, "failed to write pem file")
	}

	// Convert the PEM file to DER file.
	if msg, err := s.runner.Run(ctx, OpenSSLCommand, conversionType, "-in", pemFilePath, "-out", derFilePath, "-inform", "PEM", "-outform", "DER"); err != nil {
		testing.ContextLogf(ctx, "OpenSSL failed to convert pem to der: %q", msg)
		return errors.Wrap(err, "failed to convert pem to der")
	}

	// Import the object with p11_replay
	if msg, err := s.runner.Run(ctx, PKCS11ReplayCommand, "--slot="+strconv.Itoa(s.slot), "--import", "--type="+outputType, "--path="+derFilePath, "--id="+identifier); err != nil {
		testing.ContextLogf(ctx, "p11_replay failed to import object: %q", msg)
		return errors.Wrap(err, "failed to import object with p11_replay")
	}

	return nil
}

// InstallKeyAndCertificate install a key and its certificate into the TPM.
// key is the private key in PEM format, certificate is the certificate in PEM format.
// identifier is the ID to the object when inserted into the user token.
func (s *TPMStore) InstallKeyAndCertificate(ctx context.Context, key, certificate, identifier string) error {
	if err := s.installObject(ctx, certificate, identifier, ConvertTypeX509, OutputTypeCertificate); err != nil {
		return errors.Wrap(err, "failed to install certificate")
	}
	if err := s.installObject(ctx, key, identifier, ConvertTypeRSA, OutputTypePrivateKey); err != nil {
		return errors.Wrap(err, "failed to install private key")
	}
	return nil
}
