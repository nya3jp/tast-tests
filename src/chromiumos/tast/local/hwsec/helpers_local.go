// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file implements miscellaneous and unsorted helpers.
*/

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// CmdRunnerLocal implements CmdRunner for local test.
type CmdRunnerLocal struct {
	printLog bool
}

// NewCmdRunner creates a new command runner for local test.
func NewCmdRunner() (*CmdRunnerLocal, error) {
	return &CmdRunnerLocal{printLog: true}, nil
}

// NewLoglessCmdRunner creates a new command runner for local test, which wouldn't print logs.
func NewLoglessCmdRunner() (*CmdRunnerLocal, error) {
	return &CmdRunnerLocal{printLog: false}, nil
}

// Run implements the one of hwsec.CmdRunner.
func (r *CmdRunnerLocal) Run(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	if r.printLog {
		testing.ContextLogf(ctx, "Running: %s", shutil.EscapeSlice(append([]string{cmd}, args...)))
	}
	return testexec.CommandContext(ctx, cmd, args...).Output()
}

// CmdHelperLocal extends the function set of hwsec.CmdHelper
type CmdHelperLocal struct {
	hwsec.CmdHelper
}

// AttestationHelperLocal extends the function set of hwsec.AttestationHelper
type AttestationHelperLocal struct {
	hwsec.AttestationHelper
}

// FullHelperLocal is the full version of all kinds of helper
type FullHelperLocal struct {
	CmdHelperLocal
	AttestationHelperLocal
}

// NewHelper creates a new hwsec.CmdHelper instance that make use of the functions
// implemented by CmdRunnerLocal.
func NewHelper(r hwsec.CmdRunner) (*CmdHelperLocal, error) {
	helper := hwsec.NewCmdHelper(r)
	return &CmdHelperLocal{*helper}, nil
}

// NewAttestationHelper creates a new hwsec.AttestationHelper instance that make use of the functions
// implemented by AttestationHelperLocal.
func NewAttestationHelper(ctx context.Context) (*AttestationHelperLocal, error) {
	ac, err := NewAttestationDBus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	helper := hwsec.NewAttestationHelper(ac)
	return &AttestationHelperLocal{*helper}, nil
}

// NewFullHelper creates a new hwsec.FullHelper with a local AttestationClient.
func NewFullHelper(ctx context.Context, r hwsec.CmdRunner) (*FullHelperLocal, error) {
	cmdHelperLocal, err := NewHelper(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create CmdHelperLocal")
	}
	attestationHelperLocal, err := NewAttestationHelper(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create AttestationLocal")
	}
	return &FullHelperLocal{*cmdHelperLocal, *attestationHelperLocal}, nil
}

// EnsureTPMIsReadyAndBackupSecrets ensures TPM readiness and then backs up tpm manager local data so we can restore important secrets  if needed.
func (h *CmdHelperLocal) EnsureTPMIsReadyAndBackupSecrets(ctx context.Context, timeout time.Duration) error {
	if err := h.EnsureTPMIsReady(ctx, timeout); err != nil {
		return errors.Wrap(err, "failed to ensure TPM readiness")
	}
	if err := BackupTPMManagerDataIfIntact(ctx); err != nil {
		return errors.Wrap(err, "failed to backup tpm manager lacal data")
	}
	return nil
}
