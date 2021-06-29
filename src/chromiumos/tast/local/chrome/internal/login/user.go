// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"

	"chromiumos/tast/errors"
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

	if err = cryptohome.WaitForUserMount(ctx, cfg.NormalizedUser()); err != nil {
		return err
	}

	if cfg.SkipOOBEAfterLogin() {
		ctx, st := timing.Start(ctx, "wait_for_oobe_dismiss")
		defer st.End()
		testing.ContextLog(ctx, "Waiting for OOBE to be dismissed")
		if err = testing.Poll(ctx, func(ctx context.Context) error {
			if t, err := getFirstOOBETarget(ctx, sess); err != nil {
				// This is likely Chrome crash. So there's no chance that
				// waiting for the dismiss succeeds later. Quit the polling now.
				return testing.PollBreak(err)
			} else if t != nil {
				return errors.Errorf("%s target still exists", oobePrefix)
			}
			return nil
		}, pollOpts); err != nil {
			return errors.Wrap(sess.Watcher().ReplaceErr(err), "OOBE not dismissed")
		}
	}

	return nil
}
