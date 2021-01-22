// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/common/hwsec"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomeTPMLiveTestsTPM1,
		Desc: "Runs cryptohome's TPM live tests, which test TPM keys, PCR, and NVRAM functionality",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"yich@chromium.org",
		},
		SoftwareDeps: []string{"tpm1", "reboot"},
		Attr:         []string{"group:hwsec_destructive_func"},
		Timeout:      10 * time.Minute,
	})
}

// CryptohomeTPMLiveTestsTPM1 would check cryptohome-tpm-live-test running as expect.
func CryptohomeTPMLiveTestsTPM1(ctx context.Context, s *testing.State) {
	cmdRunner, err := hwsecremote.NewCmdRunner(s.DUT())
	if err != nil {
		s.Fatal("Failed to create CmdRunner: ", err)
	}

	utility, err := hwsec.NewUtilityCryptohomeBinary(cmdRunner)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}

	helper, err := hwsecremote.NewHelper(utility, cmdRunner, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	tpmManagerUtil, err := hwsec.NewUtilityTpmManagerBinary(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create UtilityTpmManagerBinary: ", err)
	}

	s.Log("Start resetting TPM if needed")
	if err := helper.EnsureTPMIsResetAndPowerwash(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")

	if _, err := tpmManagerUtil.TakeOwnership(ctx); err != nil {
		s.Fatal("Failed to take TPM ownership: ", err)
	}

	if out, err := cmdRunner.Run(ctx, "cryptohome-tpm-live-test"); err != nil {
		logFile := filepath.Join(s.OutDir(), "tpm_live_test_output.txt")
		if writeErr := ioutil.WriteFile(logFile, out, 0644); writeErr != nil {
			s.Errorf("Failed to write to %s: %v", logFile, writeErr)
		}

		s.Fatal("TPM live test failed: ", err)
	}

	// Clean the TPM up, so that the TPM state clobbered by the TPM live tests doesn't affect subsequent tests.
	if err := helper.EnsureTPMIsResetAndPowerwash(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
}
