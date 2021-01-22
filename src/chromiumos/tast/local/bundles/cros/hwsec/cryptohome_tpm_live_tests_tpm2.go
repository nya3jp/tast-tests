// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomeTPMLiveTestsTPM2,
		Desc: "Runs cryptohome's TPM live tests, which test TPM keys, PCR, and NVRAM functionality",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"yich@chromium.org",
		},
		SoftwareDeps: []string{"tpm2"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      10 * time.Minute,
	})
}

// CryptohomeTPMLiveTestsTPM2 would check cryptohome-tpm-live-test run as expect.
func CryptohomeTPMLiveTestsTPM2(ctx context.Context, s *testing.State) {
	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("Failed to create CmdRunner: ", err)
	}
	utility, err := hwsec.NewUtilityCryptohomeBinary(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create UtilityCryptohomeBinary: ", err)
	}
	helper, err := hwseclocal.NewHelper(utility)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	if err := hwseclocal.ResetTPMAndSystemStates(ctx); err != nil {
		s.Fatal("Failed to reset TPM or system states: ", err)
	}
	if err := cryptohome.CheckService(ctx); err != nil {
		s.Fatal("cryptohome D-Bus service didn't come back: ", err)
	}

	// Waits for TPM to be owned.
	if err := helper.EnsureTPMIsReadyAndBackupSecrets(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}

	if out, err := testexec.CommandContext(ctx, "cryptohome-tpm-live-test").CombinedOutput(); err != nil {
		logFile := filepath.Join(s.OutDir(), "tpm_live_test_output.txt")
		if writeErr := ioutil.WriteFile(logFile, out, 0644); writeErr != nil {
			s.Errorf("Failed to write to %s: %v", logFile, writeErr)
		}

		s.Fatal("TPM live test failed: ", err)
	}
}
