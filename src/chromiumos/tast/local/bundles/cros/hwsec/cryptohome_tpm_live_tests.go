// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomeTPMLiveTests,
		Desc: "Runs cryptohome's TPM live tests, which test TPM keys, PCR, and NVRAM functionality",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"garryxiao@chromium.org",
		},
		SoftwareDeps: []string{"tpm2"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      10 * time.Minute,
	})
}

func CryptohomeTPMLiveTests(ctx context.Context, s *testing.State) {
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

	// Waits for TPM to be owned.
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}

	if _, err := cmdRunner.Run(ctx, "cryptohome-tpm-live-test", "--tpm2_use_system_owner_password"); err != nil {
		s.Fatal("TPM live test failed: ", err)
	}
}
