// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"fmt"
	"os"
	"path"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	// importedKeyID is the Key ID of the hardware backed key that is generated in software then imported.
	importedKeyID = "111111"
	// softwareKeyID is the Key ID of the software backed key.
	softwareKeyID = "222222"
	// generatedKeyID is the Key ID of the hardware backed key that is generated in the TPM.
	generatedKeyID = "333333"
)

// KeyType specify the type of key to create in CreateKeysForTesting. It accepts one of the values below.
type KeyType int

// Enums for KeyType.
const (
	RSAKey = iota
	ECKey
)

// allKeyIDs returns the list of Key IDs that should be covered by the test. noncopiedKeyIDs is the list of key IDs that are not copied with C_CopyObject(). copiedKeyIDs is the list of key IDs that is created through C_CopyObject(). The length of these two arrays should be equal.
func allKeyIDs() (noncopiedKeyIDs, copiedKeyIDs []string) {
	noncopiedKeyIDs = []string{importedKeyID, softwareKeyID, generatedKeyID}
	for i := range noncopiedKeyIDs {
		// Note: C0B1%02X format is just to avoid collision with other key ID. C0B1 => closest "hexspeak" for copy.
		copiedKeyIDs = append(copiedKeyIDs, fmt.Sprintf("C0B1%02X", i))
	}
	return noncopiedKeyIDs, copiedKeyIDs
}

// createKeysForTestingForUser creases all the possible keys that we should test that belong to the given username (reside in the slot that is associated with the user).
// Specify empty string for username to specify system token slot. Pass in RSAKey or ECKey for keyType.
func createKeysForTestingForUser(ctx context.Context, username string, pkcs11Util *pkcs11.Chaps, scratchpadPath string, keyType KeyType) (keys []*pkcs11.KeyInfo, retErr error) {
	defer func() {
		if retErr != nil {
			// Function failed, we need to cleanup all created keys.
			for _, k := range keys {
				if err := pkcs11Util.DestroyKey(ctx, k); err != nil {
					testing.ContextLogf(ctx, "Failed to destroy key %s during cleanup when createKeysForTestingForUser failed: %q", pkcs11Util.DumpKeyInfo(k), err)
				}
			}
			keys = nil
		}
	}()

	var importedKey, softwareKey, generatedKey *pkcs11.KeyInfo
	var err error

	// Create the software-generated, then imported key.
	if keyType == RSAKey {
		importedKey, err = pkcs11Util.CreateRSASoftwareKey(ctx, scratchpadPath, username, "testkey1", importedKeyID, false, true)
	} else if keyType == ECKey {
		importedKey, err = pkcs11Util.CreateECSoftwareKey(ctx, scratchpadPath, username, "testkey1", importedKeyID, false, true)
	} else {
		return keys, errors.Wrapf(err, "invalid keyType %d", keyType)
	}
	if err != nil {
		return keys, errors.Wrap(err, "failed to create software key")
	}
	keys = append(keys, importedKey)

	// Create the software-generated, then imported as software-backed key.
	if keyType == RSAKey {
		softwareKey, err = pkcs11Util.CreateRSASoftwareKey(ctx, scratchpadPath, username, "testkey2", softwareKeyID, true, true)
	} else if keyType == ECKey {
		softwareKey, err = pkcs11Util.CreateECSoftwareKey(ctx, scratchpadPath, username, "testkey2", softwareKeyID, true, true)
	}
	if err != nil {
		return keys, errors.Wrap(err, "failed to create software key")
	}
	keys = append(keys, softwareKey)

	// Create the TPM generated key.
	if keyType == RSAKey {
		generatedKey, err = pkcs11Util.CreateGeneratedKey(ctx, scratchpadPath, pkcs11.GenRSA2048, username, "testkey3", generatedKeyID)
	} else if keyType == ECKey {
		generatedKey, err = pkcs11Util.CreateGeneratedKey(ctx, scratchpadPath, pkcs11.GenECP256, username, "testkey3", generatedKeyID)
	}
	if err != nil {
		return keys, errors.Wrap(err, "failed to create generated key")
	}
	keys = append(keys, generatedKey)

	// Note: If anymore keys are added here, please add its ID to the list above in allKeyIDs() as well
	noncopiedKeyIDs, copiedKeyIDs := allKeyIDs()
	if len(copiedKeyIDs) != len(keys) || len(noncopiedKeyIDs) != len(keys) {
		panic("Key ID constants are out of sync.")
	}

	// Create a copy of software key for every key.
	for i, k := range keys {
		copiedKey, _, err := pkcs11Util.CreateKeyCopy(ctx, k, copiedKeyIDs[i], map[string]string{})
		if err != nil {
			return keys, errors.Wrap(err, "failed to copy key")
		}
		keys = append(keys, copiedKey)
	}

	return keys, nil
}

// CreateKeysForTesting creates the set of keys that we want to cover in our tests.
// scratchpadPath is a temporary location allocated by the test to place materials related to the keys.
// Note that a user may be created and its vault mounted in this method. Pass in RSAKey or ECKey for keyType.
func CreateKeysForTesting(ctx context.Context, r hwsec.CmdRunner, pkcs11Util *pkcs11.Chaps, cryptohome *hwsec.CryptohomeClient, scratchpadPath string, keyType KeyType) (keys []*pkcs11.KeyInfo, retErr error) {
	// Mount the vault of the user, so that we can test user keys as well.
	if err := cryptohome.MountVault(ctx, PasswordLabel, hwsec.NewPassAuthConfig(FirstUsername, FirstPassword), true, hwsec.NewVaultConfig()); err != nil {
		return keys, errors.Wrap(err, "failed to mount vault")
	}
	defer func() {
		// If this method failed, we'll need to cleanup the vault.
		if retErr != nil {
			if _, err := cryptohome.Unmount(ctx, FirstUsername); err != nil {
				testing.ContextLog(ctx, "Failed to unmount when CreateKeysForTesting failed: ", err)
			}
			if _, err := cryptohome.RemoveVault(ctx, FirstUsername); err != nil {
				testing.ContextLog(ctx, "Failed to remove vault when CreateKeysForTesting failed: ", err)
			}
		}
	}()
	if err := cryptohome.WaitForUserToken(ctx, FirstUsername); err != nil {
		return keys, errors.Wrap(err, "failed to wait for user token")
	}
	// Note that we only need to wait for the user token, not the vault, because we only use the token (which is backed by the vault) but not the actual vault itself.

	// Cleanup the keys if it failed halfway.
	defer func() {
		if retErr != nil {
			for _, k := range keys {
				if err := pkcs11Util.DestroyKey(ctx, k); err != nil {
					testing.ContextLogf(ctx, "Failed to destroy key %s during cleanup when CreateKeysForTesting failed: %q", pkcs11Util.DumpKeyInfo(k), err)
				}
			}
			keys = nil
		}
	}()

	// Create the keys for the user.
	userScratchpadPath := path.Join(scratchpadPath, "user")
	if err := os.MkdirAll(userScratchpadPath, 0755); err != nil {
		return keys, errors.Wrap(err, "failed to create scratchpad for user keys")
	}
	retKeys, err := createKeysForTestingForUser(ctx, FirstUsername, pkcs11Util, userScratchpadPath, keyType)
	if err != nil {
		return keys, errors.Wrap(err, "failed to create user key")
	}
	keys = append(keys, retKeys...)

	// Create the system keys.
	systemScratchpadPath := path.Join(scratchpadPath, "system")
	if err := os.MkdirAll(systemScratchpadPath, 0755); err != nil {
		return keys, errors.Wrap(err, "failed to create scratchpad for system keys")
	}
	retKeys, err = createKeysForTestingForUser(ctx, "", pkcs11Util, scratchpadPath, keyType)
	if err != nil {
		return keys, errors.Wrap(err, "failed to create system key")
	}
	keys = append(keys, retKeys...)

	return keys, nil
}

// CleanupTestingKeys is a helper method that remove the keys created by CreateKeysForTesting() after the test finishes.
// Usually this is called by defer in the test body.
func CleanupTestingKeys(ctx context.Context, keys []*pkcs11.KeyInfo, pkcs11Util *pkcs11.Chaps, cryptohome *hwsec.CryptohomeClient) (retErr error) {
	// Cleanup should remove all keys, only return the last error.
	for _, k := range keys {
		if err := pkcs11Util.DestroyKey(ctx, k); err != nil {
			testing.ContextLogf(ctx, "Failed to destroy key %s during CleanupTestingKeys: ", pkcs11Util.DumpKeyInfo(k))
			retErr = errors.Wrapf(err, "failed to destroy key %s during CleanupTestingKeys", pkcs11Util.DumpKeyInfo(k))
		}
	}

	if _, err := cryptohome.Unmount(ctx, FirstUsername); err != nil {
		testing.ContextLog(ctx, "Failed to unmount in CleanupTestingKeys: ", err)
		retErr = errors.Wrap(err, "failed to unmount in CleanupTestingKeys")
	}
	if _, err := cryptohome.RemoveVault(ctx, FirstUsername); err != nil {
		testing.ContextLog(ctx, "Failed to remove vault in CleanupTestingKeys: ", err)
		retErr = errors.Wrap(err, "failed to remove vault in CleanupTestingKeys")
	}

	return retErr
}

// CleanupKeysBeforeTest is a helper method that resets the system back to a state that is consistent for the test. This ensures that no stray remnants of key is left on the system.
// Note that this doesn't return anything because there's no guarantee if there's anything to remove/cleanup before the test runs.
// Usually this is called at the start of the test.
func CleanupKeysBeforeTest(ctx context.Context, pkcs11Util *pkcs11.Chaps, cryptohome *hwsec.CryptohomeClient) {
	// We simply remove the user vault to ensure user token is clean.
	if _, err := cryptohome.Unmount(ctx, FirstUsername); err != nil {
		testing.ContextLog(ctx, "Failed to unmount in CleanupKeysBeforeTest: ", err)
	}
	if _, err := cryptohome.RemoveVault(ctx, FirstUsername); err != nil {
		testing.ContextLog(ctx, "Failed to remove vault in CleanupKeysBeforeTest: ", err)
	}

	// For system token, we'll remove them one by one.
	noncopiedKeyIDs, copiedKeyIDs := allKeyIDs()
	keyIDs := append(noncopiedKeyIDs, copiedKeyIDs...)
	for _, keyID := range keyIDs {
		if err := pkcs11Util.ClearObjectsOfAllType(ctx, 0, keyID); err != nil {
			testing.ContextLogf(ctx, "Failed to remove key ID %q: %q", keyID, err)
		}
	}
}
