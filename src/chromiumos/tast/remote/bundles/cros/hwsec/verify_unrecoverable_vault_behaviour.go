// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/remote/bundles/cros/hwsec/util"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: VerifyUnrecoverableVaultBehaviour,
		Desc: "Verifies that the vault is destroyed if unrecoverable",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"dlunev@chromium.org",
		},
		SoftwareDeps: []string{"reboot", "tpm"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      5 * time.Minute,
	})
}

func VerifyUnrecoverableVaultBehaviour(ctx context.Context, s *testing.State) {
	cmdRunner, err := hwsecremote.NewCmdRunner(s.DUT())
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}

	helper, err := hwsecremote.NewHelper(cmdRunner, s.DUT())
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	utility := helper.CryptohomeClient()

	// Resets the TPM states before running the tests.
	if err := helper.EnsureTPMIsReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}
	if _, err := utility.RemoveVault(ctx, util.FirstUsername); err != nil {
		s.Fatal("Failed to remove user vault: ", err)
	}

	s.Log("Phase 1: mounts vault for the test user")

	if err := utility.MountVault(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create user vault: ", err)
	}
	if err := utility.CheckTPMWrappedUserKeyset(ctx, util.FirstUsername); err != nil {
		s.Fatal("Check user keyset failed: ", err)
	}
	if err := hwsec.WriteUserTestContent(ctx, utility, cmdRunner, util.FirstUsername, util.TestFileName1, util.TestFileContent); err != nil {
		s.Fatal("Failed to write user test content: ", err)
	}
	if _, err := utility.Unmount(ctx, util.FirstUsername); err != nil {
		s.Fatal("Failed to remove user vault: ", err)
	}

	s.Log("Phase 2: reboot and try mount user vault")

	// Reboot
	if err := helper.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot: ", err)
	}
	if err := utility.MountVault(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount user vault: ", err)
	}
	if err := utility.CheckTPMWrappedUserKeyset(ctx, util.FirstUsername); err != nil {
		s.Fatal("Check user keyset failed: ", err)
	}

	// User vault should already exist and shouldn't be destroyed.
	if content, err := hwsec.ReadUserTestContent(ctx, utility, cmdRunner, util.FirstUsername, util.TestFileName1); err != nil {
		s.Fatal("Failed to read user test content: ", err)
	} else if !bytes.Equal(content, []byte(util.TestFileContent)) {
		s.Fatalf("Unexpected test file content: got %q, want %q", string(content), util.TestFileContent)
	}

	if _, err := utility.Unmount(ctx, util.FirstUsername); err != nil {
		s.Fatal("Failed to remove user vault: ", err)
	}

	s.Log("Phase 3: destroy the keyset and see the vault destroyed upon mount")

	hash, err := utility.GetSanitizedUsername(ctx, util.FirstUsername, false)
	if err != nil {
		s.Fatal("Failed to get username's hash: ", err)
	}
	userShadowDir := "/home/.shadow/" + hash
	userKeysetFile := userShadowDir + "/master.0"

	// Remove the keyset file to make decryption fail
	if _, err := cmdRunner.Run(ctx, "rm", "-rf", userKeysetFile); err != nil {
		s.Fatal("Failed to remove the keyset file: ", err)
	}
	// Mount with no valid keyset shall vail...
	if err := utility.MountVault(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, true, hwsec.NewVaultConfig()); err == nil {
		s.Fatal("Mount was expected to fail but succeeded")
	}
	// .. and erase the vaul
	cmdBinOutput, err := cmdRunner.Run(ctx, "sh", "-c", fmt.Sprintf("[ -d %q ] && echo Dir; true", userShadowDir))
	if err != nil {
		s.Fatal("Failed to query directory existence: ", err)
	}
	cmdOutput := strings.TrimSpace(string(cmdBinOutput))
	if cmdOutput == "Dir" {
		s.Fatal("Did not remove the unrecoverable vault")
	}
}
