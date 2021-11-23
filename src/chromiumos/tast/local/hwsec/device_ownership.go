// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/session/ownership"
)

// SetUpVaultAndUserAsOwner will setup a user and its vault, and setup the policy to make the user the owner of the device.
// Caller of this assumes the responsibility of umounting/cleaning up the vault regardless of whether the function returned an error.
func SetUpVaultAndUserAsOwner(ctx context.Context, certpath, username, password, label string, utility *hwsec.CryptohomeClient) error {
	// We need the policy/ownership related stuff because we want to set the owner, so that we can create ephemeral mount.
	privKey, err := session.ExtractPrivKey(certpath)
	if err != nil {
		return errors.Wrap(err, "failed to parse PKCS #12 file")
	}

	if err := session.SetUpDevice(ctx); err != nil {
		return errors.Wrap(err, "failed to reset device ownership")
	}

	// Setup the owner policy.
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create session_manager binding")
	}
	if err := session.PrepareChromeForPolicyTesting(ctx, sm); err != nil {
		return errors.Wrap(err, "failed to prepare Chrome for testing")
	}

	// Pre-configure some owner settings, including initial key.
	settings := ownership.BuildTestSettings(username)
	if err := session.StoreSettings(ctx, sm, username, privKey, nil, settings); err != nil {
		return errors.Wrap(err, "failed to store settings")
	}

	// Start a new session, which will trigger the re-taking of ownership.
	wp, err := sm.WatchPropertyChangeComplete(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start watching PropertyChangeComplete signal")
	}
	defer wp.Close(ctx)
	ws, err := sm.WatchSetOwnerKeyComplete(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start watching SetOwnerKeyComplete signal")
	}
	defer ws.Close(ctx)

	// Now create the vault.
	if err := utility.MountVault(ctx, label, hwsec.NewPassAuthConfig(username, password), true, hwsec.NewVaultConfig()); err != nil {
		return errors.Wrap(err, "failed to create user vault for testing")
	}
	// Note: Caller of this method is responsible for cleaning up the

	if err = sm.StartSession(ctx, username, ""); err != nil {
		return errors.Wrapf(err, "failed to start new session for %s", username)
	}

	select {
	case <-wp.Signals:
	case <-ws.Signals:
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "timed out waiting for PropertyChangeComplete or SetOwnerKeyComplete signal")
	}

	return nil
}
