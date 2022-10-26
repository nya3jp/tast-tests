// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/common/pkcs11/pkcs11test"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChapsECPerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Chaps performance test that includes key import, key sign operation performance measure",
		Contacts: []string{
			"zuan@chromium.org",
			"cros-hwsec@chromium.org",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "tpm2"},
		Timeout:      4 * time.Minute,
	})
}

// prepareECKeyWithOpenSSL generate an EC key pair in DER format and store in on the disk with OpenSSL.
func prepareECKeyWithOpenSSL(ctx context.Context, scratchpadPath string, runner hwsec.CmdRunner) (privKeyPath string, retErr error) {
	// Note that we are using openssl command here for reproducibility during debugging.
	privKeyPemPath := filepath.Join(scratchpadPath, "testkey2-priv.pem")
	privKeyPath = filepath.Join(scratchpadPath, "testkey2-priv.der")

	if _, err := runner.Run(ctx, "openssl", "ecparam", "-name", "prime256v1", "-genkey", "-noout", "-out", privKeyPemPath); err != nil {
		return "", errors.Wrap(err, "failed to create key with openssl")
	}

	// Convert the private key to DER format.
	if _, err := runner.Run(ctx, "openssl", "ec", "-inform", "pem", "-outform", "der", "-in", privKeyPemPath, "-out", privKeyPath); err != nil {
		return "", errors.Wrap(err, "failed to convert private key to DER format with OpenSSL")
	}

	return privKeyPath, nil
}

func ChapsECPerf(ctx context.Context, s *testing.State) {
	r := hwseclocal.NewCmdRunner()

	helper, err := hwseclocal.NewHelper(r)
	if err != nil {
		s.Fatal("Failed to create hwsec helper: ", err)
	}
	utility := helper.CryptohomeClient()

	pkcs11Util, err := pkcs11.NewChaps(ctx, r, utility)
	if err != nil {
		s.Fatal("Failed to create PKCS#11 Utility: ", err)
	}

	const scratchpadPath = "/tmp/ChapsECPerf"

	// Remove all keys/certs before the test as well.
	if err := pkcs11test.CleanupScratchpad(ctx, r, scratchpadPath); err != nil {
		s.Fatal("Failed to clean scratchpad before the start of test: ", err)
	}
	// Remove the user vault, if any is remaining from another test.
	util.CleanupUserMount(ctx, utility)

	// Prepare the scratchpad.
	f1, f2, err := pkcs11test.PrepareScratchpadAndTestFiles(ctx, r, scratchpadPath)
	if err != nil {
		s.Fatal("Failed to initialize the scratchpad space: ", err)
	}
	// Remove all keys/certs, if any at the end. i.e. Cleanup after ourselves.
	defer pkcs11test.CleanupScratchpad(ctx, r, scratchpadPath)

	// Create a user vault for the slot/token.
	// Mount the vault of the user, so that we can test user keys as well.
	if err := utility.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount user vault: ", err)
	}
	defer func() {
		if err := util.CleanupUserMount(ctx, utility); err != nil {
			s.Error("Cleanup failed: ", err)
		}
	}()

	// Generate one single EC private key.
	privECKeyPath, err := prepareECKeyWithOpenSSL(ctx, scratchpadPath, r)
	if err != nil {
		s.Fatal("Failed to create private key: ", err)
	}

	if err := utility.WaitForUserToken(ctx, util.FirstUsername); err != nil {
		s.Fatal("Failed to wait for user token: ", err)
	}

	// Get the slot ID for the user vault.
	username := util.FirstUsername
	slot, err := utility.GetTokenForUser(ctx, util.FirstUsername)
	if err != nil {
		s.Fatal("Failed to get user token slot ID: ", err)
	}

	// Time the generate operation.
	_, generateHwElapsed, err := util.GenerateKeysAndMeasure(ctx, pkcs11Util, pkcs11.GenECP256, slot, username, "33", util.GenerateHWTimes)
	// Note that we do not retain the generated keys because we don't need to test the signing performance.
	// Signing performance for imported and generated keys are the same, so we only test the imported variant.
	if err != nil {
		s.Fatal("Failed to generate hardware backed keys: ", err)
	}

	// Time the import operation for hw-backed keys.
	importedHwKeys, importHwElapsed, err := util.ImportKeysAndMeasure(ctx, pkcs11Util, privECKeyPath, slot, username, "11", util.ImportHWTimes, false)
	if err != nil {
		s.Fatal("Failed to import hardware backed keys: ", err)
	}
	// Note: We don't need to cleanup the imported keys here because it goes with the vault cleanup.

	// Time the import operation for sw-backed keys.
	importedSwKeys, importSwElapsed, err := util.ImportKeysAndMeasure(ctx, pkcs11Util, privECKeyPath, slot, username, "22", util.ImportSWTimes, true)
	if err != nil {
		s.Fatal("Failed to import software backed keys: ", err)
	}

	// Time the signing operation for hw-backed keys.
	signHwElapsed, err := util.SignAndMeasure(ctx, pkcs11Util, importedHwKeys[0], &pkcs11.ECDSASHA1, util.SignHWTimes, f1, f2)
	if err != nil {
		s.Fatal("Failed to sign with hardware backed keys: ", err)
	}

	// Time the signing operation for sw-backed keys.
	signSwElapsed, err := util.SignAndMeasure(ctx, pkcs11Util, importedSwKeys[0], &pkcs11.ECDSASHA1, util.SignSWTimes, f1, f2)
	if err != nil {
		s.Fatal("Failed to sign with software backed keys: ", err)
	}

	// Record the perf measurements.
	value := perf.NewValues()

	value.Set(perf.Metric{
		Name:      "chaps_generate_time",
		Variant:   "hwbacked_ec",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, generateHwElapsed.Seconds()/float64(util.GenerateHWTimes))
	value.Set(perf.Metric{
		Name:      "chaps_import_time",
		Variant:   "hwbacked_ec",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, importHwElapsed.Seconds()/float64(util.ImportHWTimes))
	value.Set(perf.Metric{
		Name:      "chaps_import_time",
		Variant:   "swbacked_ec",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, importSwElapsed.Seconds()/float64(util.ImportSWTimes))
	value.Set(perf.Metric{
		Name:      "chaps_sign_time",
		Variant:   "hwbacked_ec_sha1",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, signHwElapsed.Seconds()/float64(util.SignHWTimes))
	value.Set(perf.Metric{
		Name:      "chaps_sign_time",
		Variant:   "swbacked_ec_sha1",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, signSwElapsed.Seconds()/float64(util.SignSWTimes))

	value.Save(s.OutDir())
}
