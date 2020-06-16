// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/common/pkcs11/pkcs11test"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	libhwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChapsPerf,
		Desc: "Chaps performance test that includes key import, key sign operation performance measure",
		Attr: []string{"group:crosbolt", "crosbolt_perbuild"},
		Contacts: []string{
			"zuan@chromium.org",
			"cros-hwsec@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
	})
}

// prepareKeyWithOpenSSL generate an RSA key pair in DER format and store in on the disk with OpenSSL.
func prepareKeyWithOpenSSL(ctx context.Context, scratchpadPath string, runner hwsec.CmdRunner) (privKeyPath string, retErr error) {
	// Note that we are using openssl command here for reproducibility during debugging.
	privKeyPemPath := filepath.Join(scratchpadPath, "testkey1-priv.pem")
	certPemPath := filepath.Join(scratchpadPath, "testkey1-cert.pem")
	privKeyPath = filepath.Join(scratchpadPath, "testkey1-priv.der")

	if _, err := runner.Run(ctx, "openssl", "req", "-nodes", "-x509", "-sha1", "-newkey", "rsa:2048", "-keyout", privKeyPemPath, "-out", certPemPath, "-days", "365", "-subj", "/C=US/ST=CA/L=MTV/O=ChromiumOS/CN=chromiumos.example.com"); err != nil {
		return "", errors.Wrap(err, "failed to create key with openssl")
	}

	// Convert the private key to DER format.
	if _, err := runner.Run(ctx, "openssl", "pkcs8", "-inform", "pem", "-outform", "der", "-in", privKeyPemPath, "-out", privKeyPath, "-nocrypt"); err != nil {
		return "", errors.Wrap(err, "failed to convert private key to DER format with OpenSSL")
	}

	return privKeyPath, nil
}

// cleanupUserMount unmounts and removes the vault of util.FirstUsername.
func cleanupUserMount(ctx context.Context, cryptohomeUtil *hwsec.UtilityCryptohomeBinary) error {
	if _, err := cryptohomeUtil.Unmount(ctx, util.FirstUsername); err != nil {
		return errors.Wrap(err, "failed to unmount")
	}
	if _, err := cryptohomeUtil.RemoveVault(ctx, util.FirstUsername); err != nil {
		return errors.Wrap(err, "failed to remove vault")
	}
	return nil
}

// importKeysAndMeasure import the key specified by privKeyPath into token held by slot slot in chaps and import it times times. prefix should be a unique hex prefix between calls. It'll return the KeyInfo to the imported keys, the total duration and if an error occurred.
func importKeysAndMeasure(ctx context.Context, pkcs11Util *pkcs11.Chaps, privKeyPath string, slot int, prefix string, times int, softwareBacked bool) (importedKeys []*pkcs11.KeyInfo, importElapsed time.Duration, retErr error) {
	// Run hw-backed import once for warm up.
	if _, err := pkcs11Util.ImportPrivateKeyBySlot(ctx, privKeyPath, slot, fmt.Sprintf("%sABCD", prefix), softwareBacked); err != nil {
		return nil, importElapsed, errors.Wrap(err, "warmup for import failed")
	}

	// Time the import operation.
	// We run import many times because there's a large variance in import run time, and we want to reduce that variance.
	importStart := time.Now()
	for i := 0; i < times; i++ {
		objID := fmt.Sprintf("%s%04X", prefix, i)
		key, err := pkcs11Util.ImportPrivateKeyBySlot(ctx, privKeyPath, slot, objID, softwareBacked)
		if err != nil {
			return nil, importElapsed, errors.Wrap(err, "failed to import keys")
		}
		importedKeys = append(importedKeys, key)
	}
	importElapsed = time.Since(importStart)
	return importedKeys, importElapsed, nil
}

// signAndMeasure will sign the content pointed by tmpFile1 with mechanism and write the signature into tmpFile2 for times times. It'll return the total duration and if an error occurred.
func signAndMeasure(ctx context.Context, pkcs11Util *pkcs11.Chaps, key *pkcs11.KeyInfo, mechanism *pkcs11.MechanismInfo, times int, tmpFile1, tmpFile2 string) (signElapsed time.Duration, retErr error) {
	signHwStart := time.Now()
	for i := 0; i < times; i++ {
		// Note that we do not verify the signature here, it is checked by other tests.
		// We just assume Sign produces the correct signature if it returns no error.
		if err := pkcs11Util.Sign(ctx, key, tmpFile1, tmpFile2, mechanism); err != nil {
			return signElapsed, errors.Wrap(err, "failed to sign with key")
		}
	}
	signElapsed = time.Since(signHwStart)
	return signElapsed, nil
}

func ChapsPerf(ctx context.Context, s *testing.State) {
	const (
		importHwTimes = 16
		importSwTimes = 16
		signHwTimes   = 16
		signSwTimes   = 16
	)

	r, err := libhwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}

	utility, err := hwsec.NewUtilityCryptohomeBinary(r)
	if err != nil {
		s.Fatal("Cryptohome Utilty creation error: ", err)
	}

	pkcs11Util, err := pkcs11.NewChaps(ctx, r, utility)
	if err != nil {
		s.Fatal("Failed to create PKCS#11 Utility: ", err)
	}

	const scratchpadPath = "/tmp/ChapsPerf"

	// Remove all keys/certs before the test as well.
	if err := pkcs11test.CleanupScratchpad(ctx, r, scratchpadPath); err != nil {
		s.Fatal("Failed to clean scratchpad before the start of test: ", err)
	}
	// Remove the user vault, if any is remaining from another test.
	cleanupUserMount(ctx, utility)

	// Prepare the scratchpad.
	f1, f2, err := pkcs11test.PrepareScratchpadAndTestFiles(ctx, r, scratchpadPath)
	if err != nil {
		s.Fatal("Failed to initialize the scratchpad space: ", err)
	}
	// Remove all keys/certs, if any at the end. i.e. Cleanup after ourselves.
	defer pkcs11test.CleanupScratchpad(ctx, r, scratchpadPath)

	// Create a user vault for the slot/token.
	// Mount the vault of the user, so that we can test user keys as well.
	if err := utility.MountVault(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount user vault: ", err)
	}
	defer func() {
		if err := cleanupUserMount(ctx, utility); err != nil {
			s.Error("Cleanup failed: ", err)
		}
	}()

	// Generate one single private key.
	privKeyPath, err := prepareKeyWithOpenSSL(ctx, scratchpadPath, r)
	if err != nil {
		s.Fatal("Failed to create private key: ", err)
	}

	if err := utility.WaitForUserToken(ctx, util.FirstUsername); err != nil {
		s.Fatal("Failed to wait for user token: ", err)
	}

	// Get the slot ID for the user vault.
	slot, err := utility.GetTokenForUser(ctx, util.FirstUsername)
	if err != nil {
		s.Fatal("Failed to get user token slot ID: ", err)
	}

	// Time the import operation for hw-backed keys.
	importedHwKeys, importHwElapsed, err := importKeysAndMeasure(ctx, pkcs11Util, privKeyPath, slot, "11", importHwTimes, false)
	if err != nil {
		s.Fatal("Failed to import hardware backed keys: ", err)
	}
	// Note: We don't need to cleanup the imported keys here because it goes with the vault cleanup.

	// Time the import operation for sw-backed keys.
	importedSwKeys, importSwElapsed, err := importKeysAndMeasure(ctx, pkcs11Util, privKeyPath, slot, "22", importSwTimes, true)
	if err != nil {
		s.Fatal("Failed to import software backed keys: ", err)
	}

	// Time the signing operation for hw-backed keys.
	signHwElapsed, err := signAndMeasure(ctx, pkcs11Util, importedHwKeys[0], &pkcs11.SHA256RSAPKCS, signHwTimes, f1, f2)
	if err != nil {
		s.Fatal("Failed to sign with hardware backed keys: ", err)
	}

	// Time the signing operation for sw-backed keys.
	signSwElapsed, err := signAndMeasure(ctx, pkcs11Util, importedSwKeys[0], &pkcs11.SHA256RSAPKCS, signSwTimes, f1, f2)
	if err != nil {
		s.Fatal("Failed to sign with software backed keys: ", err)
	}

	// Record the perf measurements.
	value := perf.NewValues()

	value.Set(perf.Metric{
		Name:      "chaps_import_time",
		Variant:   "hwbacked_rsa",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, importHwElapsed.Seconds()/float64(importHwTimes))
	value.Set(perf.Metric{
		Name:      "chaps_import_time",
		Variant:   "swbacked_rsa",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, importSwElapsed.Seconds()/float64(importSwTimes))
	value.Set(perf.Metric{
		Name:      "chaps_sign_time",
		Variant:   "hwbacked_rsa_pkcs_sha256",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, signHwElapsed.Seconds()/float64(signHwTimes))
	value.Set(perf.Metric{
		Name:      "chaps_sign_time",
		Variant:   "swbacked_rsa_pkcs_sha256",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, signSwElapsed.Seconds()/float64(signSwTimes))

	value.Save(s.OutDir())
}
