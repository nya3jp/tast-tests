// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"io"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/internal/config"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/syslog"
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
	case config.GAIALogin, config.SAMLLogin:
		// GAIA login requires Internet connectivity.
		if err := shill.WaitForOnline(ctx); err != nil {
			return err
		}
		if err := performGAIALogin(ctx, cfg, sess, conn); err != nil {
			return err
		}
	}

	if cfg.WaitForCryptohome() {
		mountType := cryptohome.Permanent
		if cfg.EphemeralUser() {
			mountType = cryptohome.Ephemeral
		}

		// Shorten deadline to reserve time for reading the log.
		shortenCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
		defer cancel()

		if err = cryptohome.WaitForUserMountAndValidateType(shortenCtx, cfg.NormalizedUser(), mountType); err != nil {
			if cfg.LoginMode() == config.GAIALogin {
				// Backup the original error.
				origErr := err

				// Check the error message from the server side.
				logReader, err := syslog.NewLineReader(ctx, syslog.ChromeLogFile, true, nil)
				if err != nil {
					return errors.Wrapf(origErr, "could not get Chrome log reader: %v", err)
				}
				defer logReader.Close()

				for {
					line, err := logReader.ReadLine()
					if err != nil {
						if err != io.EOF {
							return errors.Wrapf(origErr, "failed to read file %v: %v", syslog.ChromeLogFile, err)
						}

						// Could not find server side authentication error.
						// Return the error directly.
						return origErr
					}
					if strings.HasSuffix(line, "Got authentication error\n") {
						break
					}
				}

				// Skip two lines after authentication error to identify the exact authentication error:
				//   Got authentication error
				//   net_error: net::OK (skip)
				//   response body: { (skip)
				//    "error": "rate_limit_exceeded"
				//   }
				for i := 0; i < 2; i++ {
					if _, err := logReader.ReadLine(); err != nil {
						return errors.Wrapf(origErr, "failed to skip the lines after authentication error: %v", err)
					}
				}

				// Read the third line after the authentication error.
				line, err := logReader.ReadLine()
				if err != nil {
					return errors.Wrapf(origErr, "failed to read the authentication error: %v", err)
				}
				authErr := strings.TrimSpace(line)
				return errors.Errorf("authentication error: %v", authErr)
			}
			return err
		}
	}

	if cfg.SkipOOBEAfterLogin() {
		ctx, st := timing.Start(ctx, "wait_for_oobe_dismiss")
		defer st.End()
		return waitForPageWithPrefixToBeDismissed(ctx, sess, oobePrefix)
	}

	return nil
}
