// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"os"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/filesnapshot"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const googleKeysDataPath = "/run/attestation/google_keys.data"

// AttestationLocalInfra enables/disables the local server implementation on DUT.
type AttestationLocalInfra struct {
	dc        *hwsec.DaemonController
	fpca      *FakePCAAgent
	snapshot  *filesnapshot.Snapshot
	dbStashed bool
}

// NewAttestationLocalInfra creates a new AttestationLocalInfra instance, with dc used to control the D-Bus service daemons.
func NewAttestationLocalInfra(dc *hwsec.DaemonController) *AttestationLocalInfra {
	return &AttestationLocalInfra{dc, nil, filesnapshot.NewSnapshot(), false}
}

// Enable enables the local test infra for attestation flow testing.
func (ali *AttestationLocalInfra) Enable(ctx context.Context) (lastErr error) {
	if err := ali.restoreTPMOwnerPasswordIfNeeded(ctx); err != nil {
		return errors.Wrap(err, "failed to restore tpm owner password")
	}
	if _, err := os.Stat(hwsec.AttestationDBPath); err == nil {
		// Note: we don't restart attestationd here because key injection that follows restarts attestationd already.
		if err := ali.snapshot.Stash(hwsec.AttestationDBPath); err != nil {
			return errors.Wrap(err, "failed to stash attestation database")
		}
		ali.dbStashed = true
	} else if !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to check stat of attestation database")
	}
	// Pop the stored snapshot of attestation database and restart attestationd if other parts of this function fails.
	defer func() {
		if lastErr != nil && ali.dbStashed {
			if err := ali.snapshot.Pop(hwsec.AttestationDBPath); err != nil {
				testing.ContextLog(ctx, "Failed to pop attestation datase back: ", err)
			}
			if err := ali.dc.RestartAttestation(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to restart attestation service after popping attestation database: ", err)
			}
		}
	}()
	if err := ali.injectWellKnownGoogleKeys(ctx); err != nil {
		return errors.Wrap(err, "failed to inject well-known keys")
	}
	// Revert the key injection if other parts of this function fails.
	defer func() {
		if lastErr != nil {
			if err := ali.injectNormalGoogleKeys(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to inject the normal key back: ", err)
			}
		}
	}()
	if err := ali.enableFakePCAAgent(ctx); err != nil {
		return errors.Wrap(err, "failed to enable fake pca agent")
	}
	return nil
}

// Disable disables the local test infra for attestation flow testing.
func (ali *AttestationLocalInfra) Disable(ctx context.Context) error {
	var lastErr error
	if ali.dbStashed {
		if err := ali.snapshot.Pop(hwsec.AttestationDBPath); err != nil {
			testing.ContextLog(ctx, "Failed to pop the snapshot of attestation database back: ", err)
			lastErr = errors.Wrap(err, "failed to pop the snapshot of attestation database back")
		}
	}
	if err := ali.injectNormalGoogleKeys(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to inject the normal key back: ", err)
		lastErr = errors.Wrap(err, "failed to inject the normal key back")
	}
	if err := ali.disableFakePCAAgent(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to disable fake pca agent: ", err)
		lastErr = errors.Wrap(err, "failed to disable fake pca agent")
	}
	return lastErr
}

// restoreTPMOwnerPasswordIfNeeded restores the owner password from the snapshot stored
// at the beginning of the entire test program if the owner password gets wiped already.
func (ali *AttestationLocalInfra) restoreTPMOwnerPasswordIfNeeded(ctx context.Context) error {
	hasOwnerPassword, err := isTPMLocalDataIntact(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check owner password")
	}
	if hasOwnerPassword {
		return nil
	}
	if err := RestoreTPMManagerData(ctx); err != nil {
		return errors.Wrap(err, "failed to restore tpm manager local data")
	}
	if err := ali.dc.RestartTPMManager(ctx); err != nil {
		return errors.Wrap(err, "failed to restart tpm manager")
	}
	hasOwnerPassword, err = isTPMLocalDataIntact(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check owner password")
	}
	if !hasOwnerPassword {
		return errors.Wrap(err, "no owner password after restoration")
	}
	return nil
}

// injectWellKnownGoogleKeys creates the well-known Google keys file and restarts attestation service.
func (ali *AttestationLocalInfra) injectWellKnownGoogleKeys(ctx context.Context) (lastErr error) {
	if _, err := os.Stat(googleKeysDataPath); os.IsNotExist(err) {
		if _, err := testexec.CommandContext(ctx, "attestation-injected-keys").Output(); err != nil {
			return errors.Wrap(err, "failed to create key file")
		}
	}
	defer func() {
		if lastErr != nil {
			if err := os.Remove(googleKeysDataPath); err != nil {
				testing.ContextLog(ctx, "Failed to remove the injected key database: ", err)
			}
		}
	}()
	if err := ali.dc.RestartAttestation(ctx); err != nil {
		return errors.Wrap(err, "failed to restart attestation")
	}
	return nil
}

// injectNormalGoogleKeys deletes the well-known Google keys file and restarts attestation service.
func (ali *AttestationLocalInfra) injectNormalGoogleKeys(ctx context.Context) error {
	if err := os.Remove(googleKeysDataPath); err != nil {
		return errors.Wrap(err, "failed to remove injected key file")
	}
	if err := ali.dc.RestartAttestation(ctx); err != nil {
		return errors.Wrap(err, "failed to restart attestation")
	}
	return nil
}

// enableFakePCAAgent stops the normal pca agent and starts the fake one.
func (ali *AttestationLocalInfra) enableFakePCAAgent(ctx context.Context) (lastErr error) {
	if err := ali.dc.StopPCAAgent(ctx); err != nil {
		return errors.Wrap(err, "failed to stop normal pca agent")
	}
	defer func() {
		if lastErr != nil {
			if err := ali.dc.StartPCAAgent(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to stop start normal pca agent: ", err)
			}
		}
	}()
	if ali.fpca == nil {
		ali.fpca = FakePCAAgentContext(ctx)
		if err := ali.fpca.Start(); err != nil {
			return errors.Wrap(err, "failed to start fake pca agent")
		}
	}
	return nil
}

// disableFakePCAAgent stops the fake pca agent and starts the normal one.
func (ali *AttestationLocalInfra) disableFakePCAAgent(ctx context.Context) error {
	var firstErr error
	if ali.fpca != nil {
		if err := ali.fpca.Stop(); err != nil {
			testing.ContextLog(ctx, "Failed to stop fake pca agent: ", err)
			firstErr = errors.Wrap(err, "failed to stop fake pca agent")
		} else {
			ali.fpca = nil
		}
	}
	if err := ali.dc.StartPCAAgent(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to start normal pca agent: ", err)
		if firstErr == nil {
			firstErr = errors.Wrap(err, "failed to start normal pca agent")
		}
	}
	return firstErr
}
