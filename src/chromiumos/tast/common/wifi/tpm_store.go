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

// cleanupVault remove the vault belonging to the test user.
func cleanupVault(ctx context.Context, cryptohomeUtil *hwsec.UtilityCryptohomeBinary) error {
	if _, err := cryptohomeUtil.Unmount(ctx, TestUsername); err != nil {
		return errors.Wrap(err, "failed to unmount")
	}
	if _, err := cryptohomeUtil.RemoveVault(ctx, TestUsername); err != nil {
		return errors.Wrap(err, "failed to remove vault")
	}
	return nil
}

// SetupTPMStore setup a TPMStore for WiFi testing. Only one SetupTPMStore can exist at a time.
func SetupTPMStore(ctx context.Context, cryptohomeUtil *hwsec.UtilityCryptohomeBinary, runner hwsec.CmdRunner) (result *TPMStore, retErr error) {
	// Remove the vaults first before the test so we can be sure that the TPM Store returned is empty.
	if err := cleanupVault(ctx, cryptohomeUtil); err != nil {
		return nil, errors.Wrap(err, "failed to cleanup vault before SetupTPMStore")
	}

	// Now create the vault.
	if err := cryptohomeUtil.MountVault(ctx, TestUsername, TestPassword, TestKeyLabel, true); err != nil {
		return nil, errors.Wrap(err, "failed to mount vault")
	}
	defer func() {
		// If this method failed, we'll need to cleanup the vault.
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

	return &TPMStore{runner: runner, slot: slot, label: label, pin: pin}, nil
}

// ResetTPMStore resets the TPMStore back to before SetupTPMStore() is called.
func ResetTPMStore(ctx context.Context, s *TPMStore, cryptohomeUtil *hwsec.UtilityCryptohomeBinary) error {
	s.slot = -1
	s.runner = nil
	return cleanupVault(ctx, cryptohomeUtil)
}

// GetSlot return the slot number to access the PKCS#11 slot/token for testing.
func (s *TPMStore) GetSlot() int {
	return s.slot
}

// GetPin return the pin to access the PKCS#11 slot/token for testing.
func (s *TPMStore) GetPin() string {
	return s.pin
}

// GetLabel return the label of the PKCS#11 slot/token for testing.
func (s *TPMStore) GetLabel() string {
	return s.label
}

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
		testing.ContextLogf(ctx, "OpenSSL failed to convert pem to der: %q", string(msg))
		return errors.Wrap(err, "failed to convert pem to der")
	}

	// Import the object with p11_replay
	if msg, err := s.runner.Run(ctx, PKCS11ReplayCommand, "--slot="+strconv.Itoa(s.slot), "--import", "--type="+outputType, "--path="+derFilePath, "--id="+identifier); err != nil {
		testing.ContextLogf(ctx, "p11_replay failed to import object: %q", string(msg))
		return errors.Wrap(err, "failed to import object with p11_replay")
	}

	return nil
}

// InstallCertificate install a certificate into the TPM.
// certificate is the certificate in PEM format. identifier is the ID to the object when inserted into the user token.
func (s *TPMStore) InstallCertificate(ctx context.Context, certificate, identifier string) error {
	return s.installObject(ctx, certificate, identifier, ConvertTypeX509, OutputTypeCertificate)
}

// InstallPrivateKey install a private key into the TPM.
// key is the private key in PEM format. identifier is the ID to the object when inserted into the user token.
func (s *TPMStore) InstallPrivateKey(ctx context.Context, key, identifier string) error {
	return s.installObject(ctx, key, identifier, ConvertTypeRSA, OutputTypePrivateKey)
}
