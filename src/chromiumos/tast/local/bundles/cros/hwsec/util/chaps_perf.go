// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/errors"
)

// Utilities and helper functions used by chaps performance test are placed here.

const (
	// ImportHWTimes is the number of times we'll run the hw-backed key import during performance test.
	ImportHWTimes = 16
	// ImportSWTimes is the number of times we'll run the sw-backed key import during performance test.
	ImportSWTimes = 16
	// SignHWTimes is the number of times we'll sign with hw-backed key during performance test.
	SignHWTimes = 16
	// SignSWTimes is the number of times we'll sign with sw-backed key during performance test.
	SignSWTimes = 16
	// GenerateHWTimes is the number of times we'll run the hw-backed key generation during performance test.
	GenerateHWTimes = 16
)

// CleanupUserMount unmounts and removes the vault of util.FirstUsername.
func CleanupUserMount(ctx context.Context, cryptohome *hwsec.CryptohomeClient) error {
	if _, err := cryptohome.Unmount(ctx, FirstUsername); err != nil {
		return errors.Wrap(err, "failed to unmount")
	}
	if _, err := cryptohome.RemoveVault(ctx, FirstUsername); err != nil {
		return errors.Wrap(err, "failed to remove vault")
	}
	return nil
}

// GenerateKeysAndMeasure generate Elliptic curve based keys and measure the time taken.
func GenerateKeysAndMeasure(ctx context.Context, pkcs11Util *pkcs11.Chaps, keyType string, slot int, username, prefix string, times int) (generatedKeys []*pkcs11.KeyInfo, generateElapsed time.Duration, retErr error) {
	// Run generation once for warm up.
	if _, err := pkcs11Util.CreateGeneratedKeyBySlot(ctx, keyType, slot, username, fmt.Sprintf("%sABCD", prefix)); err != nil {
		return nil, 0, errors.Wrap(err, "warmup for generate failed")
	}

	// Time the generate operations.
	// We run generate many times because there's a large variance in run time, and we want to reduce that variance.
	generateStart := time.Now()
	for i := 0; i < times; i++ {
		objID := fmt.Sprintf("%s%04X", prefix, i)
		key, err := pkcs11Util.CreateGeneratedKeyBySlot(ctx, keyType, slot, username, objID)
		if err != nil {
			return nil, generateElapsed, errors.Wrap(err, "failed to generate keys")
		}
		generatedKeys = append(generatedKeys, key)
	}
	generateElapsed = time.Since(generateStart)
	return generatedKeys, generateElapsed, nil
}

// ImportKeysAndMeasure import the key specified by privKeyPath into token held by slot slot in chaps and import it times times. prefix should be a unique hex prefix between calls. It'll return the KeyInfo to the imported keys, the total duration and if an error occurred.
func ImportKeysAndMeasure(ctx context.Context, pkcs11Util *pkcs11.Chaps, privKeyPath string, slot int, username, prefix string, times int, softwareBacked bool) (importedKeys []*pkcs11.KeyInfo, importElapsed time.Duration, retErr error) {
	// Run hw-backed import once for warm up.
	opts := pkcs11.ImportPrivateKeyOptions{
		PrivKeyPath:         privKeyPath,
		Slot:                slot,
		Username:            username,
		ObjID:               fmt.Sprintf("%sABCD", prefix),
		ForceSoftwareBacked: softwareBacked,
	}
	if _, err := pkcs11Util.ImportPrivateKeyBySlot(ctx, opts); err != nil {
		return nil, 0, errors.Wrap(err, "warmup for import failed")
	}

	// Time the import operation.
	// We run import many times because there's a large variance in import run time, and we want to reduce that variance.
	importStart := time.Now()
	for i := 0; i < times; i++ {
		objID := fmt.Sprintf("%s%04X", prefix, i)
		opts := pkcs11.ImportPrivateKeyOptions{
			PrivKeyPath:         privKeyPath,
			Slot:                slot,
			Username:            username,
			ObjID:               objID,
			ForceSoftwareBacked: softwareBacked,
		}
		key, err := pkcs11Util.ImportPrivateKeyBySlot(ctx, opts)
		if err != nil {
			return nil, importElapsed, errors.Wrap(err, "failed to import keys")
		}
		importedKeys = append(importedKeys, key)
	}
	importElapsed = time.Since(importStart)
	return importedKeys, importElapsed, nil
}

// SignAndMeasure will sign the content pointed by tmpFile1 with mechanism and write the signature into tmpFile2 for times times. It'll return the total duration and if an error occurred.
func SignAndMeasure(ctx context.Context, pkcs11Util *pkcs11.Chaps, key *pkcs11.KeyInfo, mechanism *pkcs11.MechanismInfo, times int, tmpFile1, tmpFile2 string) (signElapsed time.Duration, retErr error) {
	signHwStart := time.Now()
	for i := 0; i < times; i++ {
		// Note that we do not verify the signature here, it is checked by other tests.
		// We just assume Sign produces the correct signature if it returns no error.
		if err := pkcs11Util.Sign(ctx, key, tmpFile1, tmpFile2, mechanism); err != nil {
			return 0, errors.Wrap(err, "failed to sign with key")
		}
	}
	signElapsed = time.Since(signHwStart)
	return signElapsed, nil
}
