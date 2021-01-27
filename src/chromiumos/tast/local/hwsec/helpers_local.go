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
type CmdHelperLocal interface {
	hwsec.CmdHelper
	EnsureTPMIsReadyAndBackupSecrets(ctx context.Context, timeout time.Duration) error
}

// FullHelperLocal extends the function set of hwsec.FullCmdHelper
type FullHelperLocal interface {
	hwsec.FullHelper
	CmdHelperLocal
}

type cmdHelperLocalImpl struct {
	hwsec.CmdHelper
}

type fullHelperLocalImpl struct {
	hwsec.FullHelper
	cmdHelperLocalImpl
}

// NewHelper creates a new hwsec.CmdHelper instance that make use of the functions
// implemented by CmdRunnerLocal.
func NewHelper(r hwsec.CmdRunner) (CmdHelperLocal, error) {
	helper := hwsec.NewCmdHelper(r)
	return &cmdHelperLocalImpl{helper}, nil
}

// NewFullHelper creates a new hwsec.FullHelper with a local AttestationClient.
func NewFullHelper(ctx context.Context, r hwsec.CmdRunner) (FullHelperLocal, error) {
	ac, err := NewAttestationClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	helper := hwsec.NewFullHelper(r, ac)
	return &fullHelperLocalImpl{helper, cmdHelperLocalImpl{helper}}, nil
}

// EnsureTPMIsReadyAndBackupSecrets ensures TPM readiness and then backs up tpm manager local data so we can restore important secrets  if needed.
func (h *cmdHelperLocalImpl) EnsureTPMIsReadyAndBackupSecrets(ctx context.Context, timeout time.Duration) error {
	if err := h.EnsureTPMIsReady(ctx, timeout); err != nil {
		return errors.Wrap(err, "failed to ensure TPM readiness")
	}
	if err := BackupTPMManagerDataIfIntact(ctx); err != nil {
		return errors.Wrap(err, "failed to backup tpm manager lacal data")
	}
	return nil
}
