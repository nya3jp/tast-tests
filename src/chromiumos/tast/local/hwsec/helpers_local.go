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
)

// CmdHelperLocalImpl implements the helper functions for CmdHelperLocal
type CmdHelperLocalImpl struct {
	h *hwsec.CmdHelper
}

// CmdHelperLocal extends the function set of hwsec.CmdHelper
type CmdHelperLocal struct {
	hwsec.CmdHelper
	CmdHelperLocalImpl
}

// AttestationHelperLocal extends the function set of hwsec.AttestationHelper
type AttestationHelperLocal struct {
	hwsec.AttestationHelper
}

// FullHelperLocal extends the function set of hwsec.FullHelper
type FullHelperLocal struct {
	hwsec.FullHelper
	CmdHelperLocalImpl
}

// NewHelper creates a new hwsec.CmdHelper instance that make use of the functions
// implemented by CmdRunnerLocal.
func NewHelper(r hwsec.CmdRunner) (*CmdHelperLocal, error) {
	helper := hwsec.NewCmdHelper(r)
	return &CmdHelperLocal{*helper, CmdHelperLocalImpl{helper}}, nil
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
	ac, err := NewAttestationDBus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	cmdHelper := hwsec.NewCmdHelper(r)
	attestationHelper := hwsec.NewAttestationHelper(ac)
	helper := hwsec.NewFullHelper(cmdHelper, attestationHelper)
	return &FullHelperLocal{*helper, CmdHelperLocalImpl{&helper.CmdHelper}}, nil
}

// EnsureTPMIsReadyAndBackupSecrets ensures TPM readiness and then backs up tpm manager local data so we can restore important secrets  if needed.
func (h *CmdHelperLocalImpl) EnsureTPMIsReadyAndBackupSecrets(ctx context.Context, timeout time.Duration) error {
	if err := h.h.EnsureTPMIsReady(ctx, timeout); err != nil {
		return errors.Wrap(err, "failed to ensure TPM readiness")
	}
	if err := BackupTPMManagerDataIfIntact(ctx); err != nil {
		return errors.Wrap(err, "failed to backup tpm manager lacal data")
	}
	return nil
}
