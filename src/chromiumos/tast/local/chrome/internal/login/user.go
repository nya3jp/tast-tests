// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"

	"chromiumos/tast/local/chrome/internal/config"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// loginUser logs in to a freshly-restarted Chrome instance.
// It waits for the login process to complete before returning.
func loginUser(ctx context.Context, cfg *config.Config, sess *driver.Session) error {
	conn, err := WaitForOOBEConnection(ctx, sess)
	if err != nil {
		return err
	}
	defer conn.Close()

	creds := cfg.Creds()

	testing.ContextLogf(ctx, "Logging in as user %q", creds.User)
	ctx, st := timing.Start(ctx, "login")
	defer st.End()

	switch cfg.LoginMode() {
	case config.FakeLogin:
		if err := conn.Call(ctx, nil, "Oobe.loginForTesting", creds.User, creds.Pass, creds.GAIAID, false); err != nil {
			return err
		}
	case config.GAIALogin:
		// GAIA login requires Internet connectivity.
		if err := shill.WaitForOnline(ctx); err != nil {
			return err
		}
		if err := performGAIALogin(ctx, cfg, sess, conn); err != nil {
			return err
		}
	}

	mountType := cryptohome.Permanent
	if cfg.EphemeralUser() {
		mountType = cryptohome.Ephemeral
	}
	if err = cryptohome.WaitForUserMountAndValidateType(ctx, cfg.NormalizedUser(), mountType); err != nil {
		return err
	}

	if cfg.SkipOOBEAfterLogin() {
		ctx, st := timing.Start(ctx, "wait_for_oobe_dismiss")
		defer st.End()
		return waitForPageWithPrefixToBeDismissed(ctx, sess, oobePrefix)
	}

	return nil
}
