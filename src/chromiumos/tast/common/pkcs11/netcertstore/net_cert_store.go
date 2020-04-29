// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package netcertstore

import (
	"context"
	"fmt"
	"sync"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/common/pkcs11/pkcs11test"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// NetCertStore contains the information to use TPM to store certificates/keys during the test.
// Strictly speaking, this struct holds the information required to access a chaps slot/token.
// Note that NetCertStore is currently a singleton because users of this struct (network tests) only need one
// NetCertStore at a moment and handling multiple NetCertStore at a time adds unnecessary complexity.
type NetCertStore struct {
	// runner is a command runner interface for executing commands on the DUT.
	runner hwsec.CmdRunner

	// chaps is an interface to the chaps API in common/pkcs11 package.
	chaps *pkcs11.Chaps

	// cryptohome is an interface to the cryptohome API in common/hwsec package.
	cryptohome *hwsec.UtilityCryptohomeBinary

	// nextID is the next available object ID.
	nextID int

	// slot is the PKCS#11 slot in which this user slot is located.
	slot int

	// label is the label of the PKCS#11 slot.
	label string

	// pin is the pin to access the PKCS#11 slot.
	pin string
}

const (
	// testUsername is the username of our test user whose vault will host the cert/keys.
	testUsername = "test@example.com"
	// testPassword is the password to the user above.
	testPassword = "not_a_real_password"
	// testKeyLabel is the key label for the vault key that belongs to the user above.
	testKeyLabel = "password"
	// Note that the credentials above are made up and isn't actual confidential information.

	// scratchpadPath is just a location for us to store tmp files.
	scratchpadPath = "/tmp/net_cert_store"

	// startingID is the starting object ID that this store will use.
	startingID = 0x54321000
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

// singletonNetCertStore is the singleton instance of NetCertStore in use.
var singletonNetCertStore *NetCertStore

// singletonNetCertStoreLock is a mutex that guard the creation and destruction of singletonNetCertStore.
var singletonNetCertStoreLock sync.Mutex

// NewNetCertStore sets up a NetCertStore for network testing.
func NewNetCertStore(ctx context.Context, runner hwsec.CmdRunner) (result *NetCertStore, retErr error) {
	singletonNetCertStoreLock.Lock()
	defer singletonNetCertStoreLock.Unlock()

	if singletonNetCertStore != nil {
		testing.ContextLog(ctx, "Duplicate call to NewNetCertStore, returning singleton")
		return singletonNetCertStore, nil
	}

	cryptohome, err := hwsec.NewUtilityCryptohomeBinary(runner)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create cryptohome utility")
	}

	// Remove the vaults first before the test so we can be sure that the TPM Store returned is empty.
	if err := cleanupVault(ctx, cryptohome); err != nil {
		return nil, errors.Wrap(err, "failed to cleanup vault at the beginning of SetupNetCertStore")
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

	singletonNetCertStore = &NetCertStore{runner, chaps, cryptohome, startingID, slot, label, pin}
	return singletonNetCertStore, nil
}

// CleanupNetCertStore resets the environment (chaps keystore and cryptohome vault) back to the state
// before the NetCertStore instance is created.
func CleanupNetCertStore(ctx context.Context) error {
	singletonNetCertStoreLock.Lock()
	defer singletonNetCertStoreLock.Unlock()

	if singletonNetCertStore == nil {
		return errors.New("singleton NetCertStore is nil")
	}
	// Cleanup scratchpad as well.
	if err := pkcs11test.CleanupScratchpad(ctx, singletonNetCertStore.runner, scratchpadPath); err != nil {
		return errors.Wrap(err, "failed to cleanup scratchpad")
	}
	cryptohome := singletonNetCertStore.cryptohome
	singletonNetCertStore.slot = -1
	singletonNetCertStore.runner = nil
	singletonNetCertStore.chaps = nil
	singletonNetCertStore.cryptohome = nil
	singletonNetCertStore = nil
	// Note that we are not removing all created objects because we remove the vault that holds them directly.
	return cleanupVault(ctx, cryptohome)
}

// Slot returns the slot number to access the PKCS#11 slot/token for testing.
func (s *NetCertStore) Slot() int {
	return s.slot
}

// Pin returns the pin to access the PKCS#11 slot/token for testing.
func (s *NetCertStore) Pin() string {
	return s.pin
}

// Label returns the label of the PKCS#11 slot/token for testing.
func (s *NetCertStore) Label() string {
	return s.label
}

// NextID returns the next object ID that's available for use.
func (s *NetCertStore) NextID() string {
	result := fmt.Sprintf("%08X", s.nextID)
	s.nextID++
	return result
}

// InstallCertKeyPair installs a key and its certificate into the TPM.
// key is the private key in PEM format; certificate is the certificate in PEM format.
// The returned identifier is the ID to the object when inserted into the user token.
func (s *NetCertStore) InstallCertKeyPair(ctx context.Context, key, certificate string) (identifier string, retErr error) {
	// Generate the identifier.
	identifier = s.NextID()

	// Call chaps to import the key and cert.
	if _, err := s.chaps.ImportPEMKeyAndCertBySlot(ctx, scratchpadPath, key, certificate, identifier, s.slot); err != nil {
		return "", errors.Wrap(err, "failed to import")
	}
	return identifier, nil
}
