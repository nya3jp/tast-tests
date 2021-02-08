// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file implements the TPM clear tool in remote tast.
*/

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// TPMClearer clear the TPM via crossystem, this would work on both TPM1.2 and TPM2.0.
type TPMClearer struct {
	cmdRunner        hwsec.CmdRunner
	daemonController *hwsec.DaemonController
	dut              *dut.DUT
}

// NewTPMClearer creates a new TPMClearer object, where r is used to run the command internally.
func NewTPMClearer(cmdRunner hwsec.CmdRunner, daemonController *hwsec.DaemonController, dut *dut.DUT) *TPMClearer {
	return &TPMClearer{cmdRunner, daemonController, dut}
}

// ClearTPMStep1 backups the logs
func (tc *TPMClearer) ClearTPMStep1(ctx context.Context) error {
	// Copy logs before TPM reset. Ignore errors on failure.
	if outDir, ok := testing.ContextOutDir(ctx); ok {
		dateString := time.Now().Format(time.RFC3339)
		if err := tc.dut.GetFile(ctx, "/var/log/chrome/", filepath.Join(outDir, "chrome-"+dateString)); err != nil {
			testing.ContextLog(ctx, "Failed to copy chrome logs: ", err)
		}
	}

	return nil
}

// ClearTPMStep2 files the TPM clear request
func (tc *TPMClearer) ClearTPMStep2(ctx context.Context) error {
	// File clear TPM owner request to crossystem.
	if _, err := tc.cmdRunner.Run(ctx, "crossystem", "clear_tpm_owner_request=1"); err != nil {
		return errors.Wrap(err, "failed to file clear_tpm_owner_request")
	}

	return nil
}

// ClearTPMStep3 reboots and ensure every TPM daemon is up.
func (tc *TPMClearer) ClearTPMStep3(ctx context.Context) error {
	if err := tc.dut.Reboot(ctx); err != nil {
		return errors.Wrap(err, "failed to reboot")
	}

	// Wait for services.
	if err := tc.daemonController.WaitForAllDBusServices(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for hwsec D-Bus services to be ready")
	}

	return nil
}
