// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

type testParams struct {
	testName      string
	needsTpmReset bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomeTPMLiveTests,
		Desc: "Runs cryptohome's TPM live tests, which test TPM keys, PCR, and NVRAM functionality",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"yich@google.com",
		},
		SoftwareDeps: []string{"tpm", "reboot"},
		Attr:         []string{"group:hwsec_destructive_func"},
		Timeout:      15 * time.Minute,
		Params: []testing.Param{{
			Name: "tpm_ecc_auth_block_test",
			Val: testParams{
				testName:      "tpm_ecc_auth_block_test",
				needsTpmReset: false,
			},
		}, {
			Name: "tpm_bound_to_pcr_auth_block_test",
			Val: testParams{
				testName:      "tpm_bound_to_pcr_auth_block_test",
				needsTpmReset: false,
			},
		}, {
			Name: "tpm_not_bound_to_pcr_auth_block_test",
			Val: testParams{
				testName:      "tpm_not_bound_to_pcr_auth_block_test",
				needsTpmReset: false,
			},
		}, {
			Name: "decryption_key_test",
			Val: testParams{
				testName:      "decryption_key_test",
				needsTpmReset: false,
			},
		}, {
			Name: "seal_with_current_user_test",
			Val: testParams{
				testName:      "seal_with_current_user_test",
				needsTpmReset: false,
			},
		}, {
			Name: "signature_sealed_secret_test",
			Val: testParams{
				testName:      "signature_sealed_secret_test",
				needsTpmReset: true,
			},
		}, {
			Name: "recovery_tpm_backend_test",
			Val: testParams{
				testName:      "recovery_tpm_backend_test",
				needsTpmReset: true,
			},
		}},
	})
}

// CryptohomeTPMLiveTests would check cryptohome-tpm-live-test running as expect.
func CryptohomeTPMLiveTests(ctx context.Context, s *testing.State) {
	cmdRunner := hwsecremote.NewCmdRunner(s.DUT())

	helper, err := hwsecremote.NewHelper(cmdRunner, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	tpmManager := helper.TPMManagerClient()

	s.Log("Start resetting TPM if needed")
	if s.Param().(testParams).needsTpmReset {
		if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
			s.Fatal("Failed to ensure resetting TPM: ", err)
		}
		s.Log("TPM is confirmed to be reset")
	}

	ctxForResetTPM := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Minute)
	if s.Param().(testParams).needsTpmReset {
		defer cancel()
		defer func(ctx context.Context) {
			// Clean the TPM up, so that the TPM state clobbered by the TPM live tests doesn't affect subsequent tests.
			if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
				s.Fatal("Failed to ensure resetting TPM: ", err)
			}
		}(ctxForResetTPM)
	}

	if _, err := tpmManager.TakeOwnership(ctx); err != nil {
		s.Fatal("Failed to take TPM ownership: ", err)
	}

	if out, err := cmdRunner.Run(ctx, "cryptohome-tpm-live-test", "--test="+s.Param().(testParams).testName); err != nil {
		logFile := filepath.Join(s.OutDir(), "tpm_live_test_output.txt")
		if writeErr := ioutil.WriteFile(logFile, out, 0644); writeErr != nil {
			s.Errorf("Failed to write to %s: %v", logFile, writeErr)
		}

		s.Fatal(s.Param().(testParams).testName+" from TPM live test failed: ", err)
	}
}
