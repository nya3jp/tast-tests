// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	hwsecremote "chromiumos/tast/remote/hwsec"
)

// HwsecEnv groups all hwsec objects together for more convenient access.
type HwsecEnv struct {
	CmdRunner *hwsecremote.CmdRunnerRemote
	Helper    *hwsecremote.CmdHelperRemote
	Utility   *hwsec.CryptohomeClient
}

// NewHwsecEnv creates new hwsec objects and return them.
func NewHwsecEnv(dut *dut.DUT) (*HwsecEnv, error) {
	env := HwsecEnv{}
	env.CmdRunner = hwsecremote.NewCmdRunner(dut)
	helper, err := hwsecremote.NewHelper(env.CmdRunner, dut)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create hwsec remote helper")
	}
	env.Helper = helper
	env.Utility = env.Helper.CryptohomeClient()
	return &env, nil
}

// ClearTpm resets the TPM states before running the tests.
func ClearTpm(ctx context.Context, env *HwsecEnv) error {
	if err := env.Helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		return errors.Wrap(err, "failed to ensure resetting TPM")
	}
	if err := env.Helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		return errors.Wrap(err, "failed to wait for TPM to be owned")
	}

	return nil
}
