// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tpmstore

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
// Note that only one TPMStore can exist at a time.
// TODO: check if the previous statement is necessary.
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

	cryptohome *hwsec.UtilityCryptohomeBinary
	config     *config
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

// tpmStoreExist is the flag of if some TPMStore is created and not yet closed.
var tpmStoreExist bool

// tpmStoreExistLock is a mutex that guard the access of tpmStoreExist.
var tpmStoreExistLock sync.Mutex

// config contains the information for constructing a TPMStore.
// TODO: probably more parameters can be allowed here, e.g. username, password...
type config struct {
	system bool
}

type Option func(c *config)

// SystemSlot returns an Option to ask TPMStore to use system slot.
func SystemSlot() Option {
	return func(c *config) {
		c.system = true
	}
}

// NewTPMStore sets up a TPMStore for WiFi testing.
func NewTPMStore(ctx context.Context, runner hwsec.CmdRunner, ops ...Option) (result *TPMStore, retErr error) {
	tpmStoreExistLock.Lock()
	defer tpmStoreExistLock.Unlock()

	if tpmStoreExist {
		return nil, errors.New("another instance of TPMStore already exists")
	}

	conf := &config{}
	for _, op := range ops {
		op(conf)
	}

	cryptohome, err := hwsec.NewUtilityCryptohomeBinary(runner)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create cryptohome utility")
	}

	// Remove the vaults first before the test so we can be sure that the TPM Store returned is empty.
	if err := cleanupVault(ctx, cryptohome); err != nil {
		return nil, errors.Wrap(err, "failed to cleanup vault at the beginning of SetupTPMStore")
	}

	username := ""
	if !conf.system {
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
		username = testUsername
	}

	label, pin, slot, err := cryptohome.GetTokenInfoForUser(ctx, username)
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

	tpmStoreExist = true
	store := &TPMStore{
		runner:     runner,
		cryptohome: cryptohome,
		chaps:      chaps,
		slot:       slot,
		label:      label,
		pin:        pin,
		config:     conf,
	}
	return store, nil
}

func (s *TPMStore) Close(ctx context.Context) error {
	tpmStoreExistLock.Lock()
	defer tpmStoreExistLock.Unlock()

	tpmStoreExist = false

	// Cleanup scratchpad as well.
	if err := pkcs11test.CleanupScratchpad(ctx, s.runner, scratchpadPath); err != nil {
		return errors.Wrap(err, "failed to cleanup scratchpad")
	}
	if !s.config.system {
		if err := cleanupVault(ctx, s.cryptohome); err != nil {
			return err
		}
	}
	return nil
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
