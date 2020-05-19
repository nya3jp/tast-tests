// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"os"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

const googleKeysDataPath = "/run/attestation/google_keys.data"

// AttestationLocalInfra enables/disables the local server implementation on DUT.
type AttestationLocalInfra struct {
	dc *hwsec.DaemonController
}

// NewAttestationLocalInfra creates a new AttestationLocalInfra instance, with dc used to control the D-Bus service daemons.
func NewAttestationLocalInfra(dc *hwsec.DaemonController) *AttestationLocalInfra {
	return &AttestationLocalInfra{dc}
}

// Enable enables the local test infra for attestation flow testing.
func (ali *AttestationLocalInfra) Enable(ctx context.Context) (lastErr error) {
	if err := ali.injectWellKnownGoogleKeys(ctx); err != nil {
		return errors.Wrap(err, "failed to inject well-known keys")
	}
	// Revert the key injection if other parts of this function fail.
	defer func() {
		if lastErr != nil {
			ali.injectWellKnownGoogleKeys(ctx)
		}
	}()
	if err := ali.enableFakePCAAgent(ctx); err != nil {
		return errors.Wrap(err, "failed to enable fake pca agent")
	}
	return nil
}

// Disable disables the local test infra for attestation flow testing.
func (ali *AttestationLocalInfra) Disable(ctx context.Context) (lastErr error) {
	// Defer the tasks that revert the effort by |Enable| individually to make sure they all get exercised even if errors happen in other tasks.
	defer func() {
		if err := ali.injectNormalGoogleKeys(ctx); err != nil {
			lastErr = errors.Wrap(err, "failed to inject well-known keys")
		}
	}()
	defer func() {
		if err := ali.disableFakePCAAgent(ctx); err != nil {
			lastErr = errors.Wrap(err, "failed to enable fake pca agent")
		}
	}()
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
			os.Remove(googleKeysDataPath)
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
			ali.dc.StartPCAAgent(ctx)
		}
	}()
	if err := ali.dc.StartFakePCAAgent(ctx); err != nil {
		return errors.Wrap(err, "failed to start fake pca agent")
	}
	return nil
}

// disableFakePCAAgent stops the fake pca agent and starts the normal one.
func (ali *AttestationLocalInfra) disableFakePCAAgent(ctx context.Context) (lastErr error) {
	// Even if we fail to stop fake pca agent, we still try to start the normal pca agent in best effort; thus, always try to start pca agent at the end.
	defer func() {
		if err := ali.dc.StartPCAAgent(ctx); err != nil {
			lastErr = errors.Wrap(err, "failed to start normal pca agent")
		}
	}()
	if err := ali.dc.StopFakePCAAgent(ctx); err != nil {
		return errors.Wrap(err, "failed to stop fake pca agent")
	}
	return nil
}
