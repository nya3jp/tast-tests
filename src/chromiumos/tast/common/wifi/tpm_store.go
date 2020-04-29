// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/google/uuid"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/common/pkcs11/pkcs11test"
	"chromiumos/tast/errors"
)

// TPMStore is a struct that contains the information related to using the TPM to store certificates/keys during the test. Strictly speaking, this struct holds the information required to access a chaps slot/token.
// Note that TPMStore is currently a singleton because users of this struct (WiFi tests) only need one TPMStore at a moment.
type TPMStore struct {
	// runner is a command runner interface for executing commands on the DUT.
	runner hwsec.CmdRunner

	// chaps is an interface to the chaps API in common/pkcs11 package
	chaps *pkcs11.Chaps

	// slot is the PKCS#11 slot in which this user slot is located.
	slot int

	// label is the label of the PKCS#11 slot.
	label string

	// pin is the pin to access the PKCS#11 slot.
	pin string
}

const (
	// testUsername is the username of our test user whose vault will host the WiFi keys.
	testUsername = "test@example.com"
	// testPassword is the password to the user above.
	testPassword = "not_a_real_password"
	// testKeyLabel is the key label for the vault key that belongs to the user above.
	testKeyLabel = "password"
	// Note that the credentials above are made up and isn't actual confidential information.

	// scratchpadPath is just a location for us to store tmp files
	scratchpadPath = "/tmp/wifi_tpm_store"
)

// cleanupVault removes the vault belonging to the test user.
func cleanupVault(ctx context.Context, cryptohome *hwsec.UtilityCryptohomeBinary) error {
	if _, err := cryptohome.Unmount(ctx, testUsername); err != nil {
		return errors.Wrap(err, "failed to unmount")
	}
	if _, err := cryptohome.RemoveVault(ctx, testUsername); err != nil {
		return errors.Wrap(err, "failed to remove vault")
	}
	return nil
}

// singletonTPMStore is the singleton instance of TPMStore in use.
var singletonTPMStore *TPMStore

// singletonTPMStoreLock is a mutex that guard the creation and destruction of singletonTPMStore.
var singletonTPMStoreLock sync.Mutex

// NewTPMStore sets up a TPMStore for WiFi testing.
func NewTPMStore(ctx context.Context, cryptohome *hwsec.UtilityCryptohomeBinary, runner hwsec.CmdRunner) (result *TPMStore, retErr error) {
	singletonTPMStoreLock.Lock()
	defer singletonTPMStoreLock.Unlock()

	if singletonTPMStore != nil {
		return nil, errors.New("another instance of TPMStore already exists")
	}

	// Remove the vaults first before the test so we can be sure that the TPM Store returned is empty.
	if err := cleanupVault(ctx, cryptohome); err != nil {
		return nil, errors.Wrap(err, "failed to cleanup vault at the beginning of SetupTPMStore")
	}

	// Now create the vault.
	if err := cryptohome.MountVault(ctx, testUsername, testPassword, testKeyLabel, true, hwsec.NewVaultConfig()); err != nil {
		return nil, errors.Wrap(err, "failed to mount vault")
	}
	defer func() {
		// If this function failed, we'll need to cleanup the vault.
		if retErr != nil {
			cleanupVault(ctx, cryptohome)
		}
	}()

	// Wait for the slot to be available.
	if err := cryptohome.WaitForUserToken(ctx, testUsername); err != nil {
		return nil, errors.Wrap(err, "failed to wait for user token")
	}

	// Get the slot.
	label, pin, slot, err := cryptohome.GetTokenInfoForUser(ctx, testUsername)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user token")
	}

	// Create the chaps object that is needed to import keys.
	chaps, err := pkcs11.NewChaps(ctx, runner, cryptohome)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create chaps")
	}

	// Prepare the scratchpad so we can put our temp files there.
	if _, _, err := pkcs11test.PrepareScratchpadAndTestFiles(ctx, runner, scratchpadPath); err != nil {
		return nil, errors.Wrap(err, "failed to prepare scratchpad")
	}

	singletonTPMStore = &TPMStore{runner, chaps, slot, label, pin}
	return singletonTPMStore, nil
}

// ResetTPMStore resets the environment (chaps keystore and cryptohome vault) back to the state before the TPMStore instance is created.
func ResetTPMStore(ctx context.Context, cryptohome *hwsec.UtilityCryptohomeBinary) error {
	singletonTPMStoreLock.Lock()
	defer singletonTPMStoreLock.Unlock()

	if singletonTPMStore == nil {
		return errors.New("singleton TPMStore is nil")
	}
	// Cleanup scratchpad as well.
	if err := pkcs11test.CleanupScratchpad(ctx, singletonTPMStore.runner, scratchpadPath); err != nil {
		return errors.Wrap(err, "failed to cleanup scratchpad")
	}
	singletonTPMStore.slot = -1
	singletonTPMStore.runner = nil
	singletonTPMStore.chaps = nil
	singletonTPMStore = nil
	return cleanupVault(ctx, cryptohome)
}

// Slot returns the slot number to access the PKCS#11 slot/token for testing.
func (s *TPMStore) Slot() int {
	return s.slot
}

// Pin returns the pin to access the PKCS#11 slot/token for testing.
func (s *TPMStore) Pin() string {
	return s.pin
}

// Label returns the label of the PKCS#11 slot/token for testing.
func (s *TPMStore) Label() string {
	return s.label
}

// InstallKeyAndCertificate installs a key and its certificate into the TPM.
// key is the private key in PEM format; certificate is the certificate in PEM format.
// identifier is the ID to the object when inserted into the user token.
func (s *TPMStore) InstallKeyAndCertificate(ctx context.Context, key, certificate, identifier string) error {
	privKeyPath := filepath.Join(scratchpadPath, uuid.New().String()+".pem")
	certPath := filepath.Join(scratchpadPath, uuid.New().String()+".pem")

	// Write the Key and Cert to disk.
	if _, err := s.runner.Run(ctx, "sh", "-c", fmt.Sprintf("echo %q > %q", key, privKeyPath)); err != nil {
		return errors.Wrap(err, "failed to write key pem file")
	}
	if _, err := s.runner.Run(ctx, "sh", "-c", fmt.Sprintf("echo %q > %q", certificate, certPath)); err != nil {
		return errors.Wrap(err, "failed to write cert pem file")
	}

	// Call chaps to import the key and cert.
	if _, err := s.chaps.ImportPEMPrivateKeyAndCertBySlot(ctx, scratchpadPath, privKeyPath, certPath, identifier, s.slot, false /* hw-backed */); err != nil {
		return errors.Wrap(err, "failed to import")
	}
	return nil
}
