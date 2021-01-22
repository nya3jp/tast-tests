// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"os"

	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/internal/config"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// logInAsGuest logs in to a freshly-restarted Chrome instance as a guest user.
// Due to Chrome restart, sess might be unavailable upon returning from this
// function. This function waits for the login process to complete before
// returning.
func logInAsGuest(ctx context.Context, cfg *config.Config, sess *driver.Session) error {
	oobeConn, err := WaitForOOBEConnection(ctx, sess)
	if err != nil {
		return err
	}
	defer func() {
		if oobeConn != nil {
			oobeConn.Close()
		}
	}()

	testing.ContextLog(ctx, "Logging in as a guest user")
	ctx, st := timing.Start(ctx, "login_guest")
	defer st.End()

	// guestLoginForTesting() relaunches the browser. In advance,
	// remove the file at cdputil.DebuggingPortPath, which should be
	// recreated after the port gets ready.
	os.Remove(cdputil.DebuggingPortPath)

	if err := oobeConn.Exec(ctx, "Oobe.guestLoginForTesting()"); err != nil {
		return err
	}

	if err := cryptohome.WaitForUserMount(ctx, cfg.User); err != nil {
		return err
	}
	return nil
}
