// Copyright 2021 The Chromium OS Authors. All rights reserved.
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

type hwsecTPM1SimulatorFixture struct {
}

func (fixure *hwsecTPM1SimulatorFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	daemonController := helper.DaemonController()

	if err := daemonController.TryStopDaemons(ctx, hwsec.HighLevelTPMDaemons); err != nil {
		testing.ContextLog(ctx, "Failed to stop High-level TPM daemons, this is normal if they were not running: ", err)
	}

	if err := daemonController.TryStopDaemons(ctx, hwsec.LowLevelTPMDaemons); err != nil {
		testing.ContextLog(ctx, "Failed to stop Low-level TPM daemons, this is normal if they were not running: ", err)
	}

	if err := daemonController.TryStop(ctx, hwsec.TPMSimulatorDaemon); err != nil {
		testing.ContextLog(ctx, "Failed to stop TPM simulator, this is normal if it was not running: ", err)
	}

	if helper.WriteFile(ctx, "/mnt/stateful_partition/unencrypted/tpm2-simulator/tpm_executor_version", []byte("1")); err != nil {
		s.Error("Failed to write tpm_executor_version: ", err)
	}

	if _, err := cmdRunner.Run(ctx, "rm", "-rf", "/var/lib/tpm_manager/*"); err != nil {
		testing.ContextLog(ctx, "Failed to remove /var/lib/tpm_manager, this is normal if it was not existing: ", err)
	}

	if _, err := cmdRunner.Run(ctx, "rm", "-rf", "/home/.shadow/*"); err != nil {
		testing.ContextLog(ctx, "Failed to remove /home/.shadow/, this is normal if it was not existing: ", err)
	}

	if helper.WriteFile(ctx, "/var/lib/tpm_manager/force_allow_tpm", []byte("1")); err != nil {
		s.Error("Failed to write force_allow_tpm: ", err)
	}

	if rawOutput, err := cmdRunner.RunWithCombinedOutput(ctx, "crossystem", "clear_tpm_owner_request=1"); err != nil {
		s.Errorf("Failed to fire clear_tpm_owner_request, output: %q, error: %q", string(rawOutput), err)
	}

	if err := daemonController.Ensure(ctx, hwsec.TPMSimulatorDaemon); err != nil {
		s.Error("Failed to ensure TPM simulator: ", err)
	}

	if err := daemonController.Ensure(ctx, hwsec.TcsdDaemon); err != nil {
		s.Error("Failed to ensure tcsd: ", err)
	}

	highLevelTPMDaemons := []*hwsec.DaemonInfo{
		hwsec.TPMManagerDaemon,
		hwsec.ChapsDaemon,
		hwsec.CryptohomeDaemon,
	}

	if err := daemonController.EnsureDaemons(ctx, highLevelTPMDaemons); err != nil {
		s.Error("Failed to ensure High-level TPM daemons: ", err)
	}

	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Error("Failed to ensure TPM is ready: ", err)
	}
	return nil
}

func (fixure *hwsecTPM1SimulatorFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	daemonController := helper.DaemonController()

	if err := daemonController.TryStopDaemons(ctx, hwsec.HighLevelTPMDaemons); err != nil {
		testing.ContextLog(ctx, "Failed to stop High-level TPM daemons, this is normal if they were not running: ", err)
	}

	if err := daemonController.TryStopDaemons(ctx, hwsec.LowLevelTPMDaemons); err != nil {
		testing.ContextLog(ctx, "Failed to stop Low-level TPM daemons, this is normal if they were not running: ", err)
	}

	if err := daemonController.TryStop(ctx, hwsec.TPMSimulatorDaemon); err != nil {
		testing.ContextLog(ctx, "Failed to stop TPM simulator, this is normal if it was not running: ", err)
	}

	if _, err := cmdRunner.Run(ctx, "rm", "/mnt/stateful_partition/unencrypted/tpm2-simulator/tpm_executor_version"); err != nil {
		testing.ContextLog(ctx, "Failed to remove tpm_executor_version, this is normal if it was not existing: ", err)
	}

	if _, err := cmdRunner.Run(ctx, "rm", "-rf", "/var/lib/tpm_manager/*"); err != nil {
		testing.ContextLog(ctx, "Failed to remove /var/lib/tpm_manager, this is normal if it was not existing: ", err)
	}

	if _, err := cmdRunner.Run(ctx, "rm", "-rf", "/home/.shadow/*"); err != nil {
		testing.ContextLog(ctx, "Failed to remove /home/.shadow/, this is normal if it was not existing: ", err)
	}

	if rawOutput, err := cmdRunner.RunWithCombinedOutput(ctx, "crossystem", "clear_tpm_owner_request=1"); err != nil {
		s.Errorf("Failed to fire clear_tpm_owner_request, output: %q, error: %q", string(rawOutput), err)
	}

	if err := daemonController.Ensure(ctx, hwsec.TPMSimulatorDaemon); err != nil {
		s.Error("Failed to ensure TPM simulator: ", err)
	}

	if err := daemonController.EnsureDaemons(ctx, hwsec.LowLevelTPMDaemons); err != nil {
		s.Error("Failed to ensure Low-level TPM daemons: ", err)
	}

	if err := daemonController.EnsureDaemons(ctx, hwsec.HighLevelTPMDaemons); err != nil {
		s.Error("Failed to ensure High-level TPM daemons: ", err)
	}

	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Error("Failed to ensure TPM is ready: ", err)
	}
}

func (fixure *hwsecTPM1SimulatorFixture) Reset(ctx context.Context) error {
	return nil
}

func (fixure *hwsecTPM1SimulatorFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (fixure *hwsecTPM1SimulatorFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "hwsecTPM1Simulator",
		Desc: "Enable the TPM1 simulator",
		Contacts: []string{
			"yich@google.com",
			"cros-hwsec@chromium.org",
		},
		Impl:            &hwsecTPM1SimulatorFixture{},
		SetUpTimeout:    60 * time.Second,
		ResetTimeout:    60 * time.Second,
		PostTestTimeout: 60 * time.Second,
		TearDownTimeout: 60 * time.Second,
	})
}
