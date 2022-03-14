// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package compat provides a reusable body for testing user vaults compatibility
// across versions.
package compat

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/lsbrelease"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/remote/updateutil"
	"chromiumos/tast/testing"
)

// VaultType represents a type of vault to be tested against.
type VaultType int64

const (
	// NoneVaultType is uninitialized value guard.
	NoneVaultType VaultType = iota
	// EcryptfsVaultType represents ecryptfs.
	EcryptfsVaultType
	// FscryptV1VaultType represents fscryptv1.
	FscryptV1VaultType
	// DefaultVaultType represents default type, which depends on the board.
	DefaultVaultType
)

const (
	initTimeoutN2MCompat       = 3 * time.Minute
	preUpdateTimeoutN2MCompat  = 3 * time.Minute
	postUpdateTimeoutN2MCompat = 3 * time.Minute
	sleepTimeN2MCompat         = 10 * time.Second
	// TotalTestTime is the time the test can be taking.
	TotalTestTime   = initTimeoutN2MCompat + preUpdateTimeoutN2MCompat + postUpdateTimeoutN2MCompat + 2*sleepTimeN2MCompat + updateutil.UpdateTimeout
	userName        = "foo@bar.baz"
	userPassword    = "secret"
	testFile        = "file"
	testFileContent = "content"
)

func prepareVault(ctx context.Context, dut *dut.DUT, utility *hwsec.CryptohomeClient, vtype VaultType, create bool, username, password string) error {
	// None is a wrong type
	if vtype == NoneVaultType || vtype > DefaultVaultType {
		return errors.Errorf("unsupported type: %v", vtype)
	}

	// To create V1, we need to negate the flag enabling v2.
	if vtype == FscryptV1VaultType && create {
		if err := dut.Conn().CommandContext(ctx, "initctl", "stop", "cryptohomed").Run(); err != nil {
			return errors.Wrap(err, "can't stop cryptohomed to change arguments")
		}
		testing.Sleep(ctx, sleepTimeN2MCompat)
		if err := dut.Conn().CommandContext(ctx, "initctl", "start", "cryptohomed", "CRYPTOHOMED_ARGS=--negate_fscrypt_v2_for_test").Run(); err != nil {
			return errors.Wrap(err, "can't start cryptohomed with changed arguments")
		}
		testing.Sleep(ctx, sleepTimeN2MCompat)
	}

	config := hwsec.NewVaultConfig()
	if vtype == EcryptfsVaultType {
		config.Ecryptfs = true
	}
	if err := utility.MountVault(ctx, "password", hwsec.NewPassAuthConfig(username, password), create, config); err != nil {
		return errors.Wrap(err, "failed to create user vault for testing")
	}
	return nil
}

// NToMVaultCompatImpl is a body of the cross version vault compatibility test.
func NToMVaultCompatImpl(ctx context.Context, s *testing.State, vtype VaultType) {
	cmdRunner := hwsecremote.NewCmdRunner(s.DUT())
	helper, herr := hwsecremote.NewHelper(cmdRunner, s.DUT())
	if herr != nil {
		s.Fatal("Failed to create hwsec local helper: ", herr)
	}
	utility := helper.CryptohomeClient()

	// Reserve time for deferred calls.
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	initCtx, cancel := context.WithTimeout(ctx, initTimeoutN2MCompat)
	defer cancel()

	// Resets the TPM states before running the tests.
	if err := helper.EnsureTPMAndSystemStateAreReset(initCtx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	if err := helper.EnsureTPMIsReady(initCtx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}
	if _, err := utility.RemoveVault(initCtx, userName); err != nil {
		s.Fatal("Failed to remove user vault: ", err)
	}

	// Limit the timeout for the preparation steps.
	preCtx, cancel := context.WithTimeout(ctx, preUpdateTimeoutN2MCompat)
	defer cancel()

	lsbContent := map[string]string{
		lsbrelease.Board:     "",
		lsbrelease.Version:   "",
		lsbrelease.Milestone: "",
	}

	err := updateutil.FillFromLSBRelease(preCtx, s.DUT(), s.RPCHint(), lsbContent)
	if err != nil {
		s.Fatal("Failed to get all the required information from lsb-release: ", err)
	}

	board := lsbContent[lsbrelease.Board]
	originalVersion := lsbContent[lsbrelease.Version]

	milestoneN, err := strconv.Atoi(lsbContent[lsbrelease.Milestone])
	if err != nil {
		s.Fatalf("Failed to convert milestone to integer %s: %v", lsbContent[lsbrelease.Milestone], err)
	}
	milestoneM := milestoneN - 1 // Target milestone.

	// Find the latest stable release for milestone M.
	paygen, err := updateutil.LoadPaygenFromGS(preCtx)
	if err != nil {
		s.Fatal("Failed to load paygen data: ", err)
	}

	filtered := paygen.FilterBoardChannelDeltaType(board, "stable", "OMAHA").FilterMilestone(milestoneM)
	latest, err := filtered.FindLatest()
	if err != nil {
		s.Fatalf("Failed to find the latest canary release for milestone %d and board %s: %v", milestoneM, board, err)
	}
	rollbackVersion := latest.ChromeOSVersion

	builderPath := fmt.Sprintf("%s-release/R%d-%s", board, milestoneM, rollbackVersion)

	// Update the DUT.
	s.Logf("Starting update from %s to %s", originalVersion, rollbackVersion)
	if err := updateutil.UpdateFromGS(ctx, s.DUT(), s.OutDir(), s.RPCHint(), builderPath); err != nil {
		s.Fatalf("Failed to update DUT to image for %q from GS: %v", builderPath, err)
	}

	// Limit the timeout for the verification steps.
	postCtx, cancel := context.WithTimeout(ctx, postUpdateTimeoutN2MCompat)
	defer cancel()

	// Reboot the DUT.
	s.Log("Rebooting the DUT after the update")
	if err := s.DUT().Reboot(postCtx); err != nil {
		s.Fatal("Failed to reboot the DUT after update: ", err)
	}

	// Check the image version.
	version, err := updateutil.ImageVersion(postCtx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to read DUT image version after the update: ", err)
	}
	s.Logf("The DUT image version after the update is %s", version)
	if version != rollbackVersion {
		if version == originalVersion {
			// Rollback is not needed here, the test execution can stop.
			s.Fatal("The image version did not change after the update")
		}
		s.Errorf("Unexpected image version after the update; got %s, want %s", version, rollbackVersion)
	}

	// Create vault
	defer utility.RemoveVault(ctxForCleanUp, userName)
	{
		s.Log("Creating test vault")
		if err := prepareVault(ctx, s.DUT(), utility, vtype /*create=*/, true, userName, userPassword); err != nil {
			s.Fatal("Can't create vault: ", err)
		}
		defer utility.UnmountAll(ctxForCleanUp)

		if err := hwsec.WriteUserTestContent(ctx, utility, cmdRunner, userName, testFile, testFileContent); err != nil {
			s.Fatal("Failed to write user test content: ", err)
		}
	}

	// Restore original image version with rollback.
	s.Log("Restoring the original device image")
	if err := s.DUT().Conn().CommandContext(postCtx, "update_engine_client", "--rollback", "--nopowerwash", "--follow").Run(); err != nil {
		s.Error("Failed to rollback the DUT: ", err)
	}

	// Reboot the DUT.
	s.Log("Rebooting the DUT after the rollback")
	if err := s.DUT().Reboot(postCtx); err != nil {
		s.Fatal("Failed to reboot the DUT after rollback: ", err)
	}

	// Check the image version.
	version, err = updateutil.ImageVersion(postCtx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to read DUT image version after the update: ", err)
	}
	s.Logf("The DUT image version after the rollback is %s", version)
	if version != originalVersion {
		s.Errorf("Image version is not the original after the restoration; got %s, want %s", version, originalVersion)
	}

	// Create vault
	{
		s.Log("Verifying vault")
		if err := prepareVault(ctx, s.DUT(), utility, vtype /*create=*/, false, userName, userPassword); err != nil {
			s.Fatal("Can't mount vault: ", err)
		}
		defer utility.UnmountAll(ctxForCleanUp)

		// User vault should already exist and shouldn't be recreated.
		if content, err := hwsec.ReadUserTestContent(ctx, utility, cmdRunner, userName, testFile); err != nil {
			s.Fatal("Failed to read user test content: ", err)
		} else if !bytes.Equal(content, []byte(testFileContent)) {
			s.Fatalf("Unexpected test file content: got %q, want %q", string(content), testFileContent)
		}
	}
}
