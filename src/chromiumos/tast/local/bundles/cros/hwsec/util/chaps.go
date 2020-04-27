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

// createKeysForTestingForUser creases all the possible keys that we should test that belong to the given username (reside in the slot that is associated with the user).
// Specify empty string for username to specify system token slot.
func createKeysForTestingForUser(ctx context.Context, username string, pkcs11Util *pkcs11.Chaps, scratchpadPath string) (keys []*pkcs11.KeyInfo, retErr error) {
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

	// Create the software-generated, then imported key.
	importedKey, err := pkcs11Util.CreateRSASoftwareKey(ctx, scratchpadPath, username, "testkey1", "111111", false, true)
	if err != nil {
		return keys, errors.Wrap(err, "failed to create software key")
	}
	keys = append(keys, importedKey)

	// Create the software-generated, then imported as software-backed key.
	softwareKey, err := pkcs11Util.CreateRSASoftwareKey(ctx, scratchpadPath, username, "testkey2", "222222", true, true)
	if err != nil {
		return keys, errors.Wrap(err, "failed to create software key")
	}
	keys = append(keys, softwareKey)

	// Create the TPM generated key.
	generatedKey, err := pkcs11Util.CreateRsaGeneratedKey(ctx, scratchpadPath, username, "testkey3", "333333")
	if err != nil {
		return keys, errors.Wrap(err, "failed to create generated key")
	}
	keys = append(keys, generatedKey)

	// Create a copy of software key for every key.
	for i, k := range keys {
		// Note: C0B1%02X format is just to avoid collision with other key ID. C0B1 => closest "hexspeak" for copy.
		copiedKey, _, err := pkcs11Util.CreateKeyCopy(ctx, k, fmt.Sprintf("C0B1%02X", i), map[string]string{})
		if err != nil {
			return keys, errors.Wrap(err, "failed to copy key")
		}
		keys = append(keys, copiedKey)
	}

	return keys, nil
}

// CreateKeysForTesting creates the set of keys that we want to cover in our tests.
// scratchpadPath is a temporary location allocated by the test to place materials related to the keys.
func CreateKeysForTesting(ctx context.Context, r hwsec.CmdRunner, pkcs11Util *pkcs11.Chaps, cryptohomeUtil *hwsec.UtilityCryptohomeBinary, scratchpadPath string) (keys []*pkcs11.KeyInfo, retErr error) {
	// Create the system keys.
	systemScratchpadPath := path.Join(scratchpadPath, "system")
	if err := os.MkdirAll(systemScratchpadPath, 0755); err != nil {
		return keys, errors.Wrap(err, "failed to create scratchpad for system keys")
	}
	retKeys, err := createKeysForTestingForUser(ctx, "", pkcs11Util, scratchpadPath)
	if err != nil {
		return keys, errors.Wrap(err, "failed to create system key")
	}
	keys = append(keys, retKeys...)

	return keys, nil
}

// CleanupTestingKeys is a helper method that remove the keys created by CreateKeysForTesting() after the test finishes.
// Usually this is called by defer in the test body.
func CleanupTestingKeys(ctx context.Context, keys []*pkcs11.KeyInfo, pkcs11Util *pkcs11.Chaps, cryptohomeUtil *hwsec.UtilityCryptohomeBinary) (retErr error) {
	// Cleanup should remove all keys, only return the last error.
	for _, k := range keys {
		if err := pkcs11Util.DestroyKey(ctx, k); err != nil {
			testing.ContextLogf(ctx, "Failed to destroy key %s during CleanupTestingKeys: ", pkcs11Util.DumpKeyInfo(k))
			retErr = errors.Wrapf(err, "failed to destroy key %s during CleanupTestingKeys", pkcs11Util.DumpKeyInfo(k))
		}
	}

	return retErr
}
