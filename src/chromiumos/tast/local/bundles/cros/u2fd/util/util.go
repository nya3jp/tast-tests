// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
)

// SetUpUserPIN sets up a test user with a specific PIN.
func SetUpUserPIN(ctx context.Context, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter, PIN, password string, autosubmit bool) (*chrome.TestConn, error) {
	user := cr.NormalizedUser()
	if mounted, err := cryptohome.IsMounted(ctx, user); err != nil {
		return nil, errors.Wrapf(err, "failed to check mounted vault for %q", user)
	} else if !mounted {
		return nil, errors.Wrapf(err, "no mounted vault for %q", user)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "getting test API connection failed")
	}

	// Set up PIN through a connection to the Settings page.
	settings, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch Settings app")
	}

	if err := settings.EnablePINUnlock(cr, password, PIN, autosubmit)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to enable PIN unlock")
	}

	if err := verifyPINUnlock(ctx, tconn, keyboard, PIN, autosubmit); err != nil {
		return nil, errors.Wrap(err, "PIN unlock doesn't work so IsUvpaa will be false")
	}

	return tconn, nil
}

func verifyPINUnlock(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter, PIN string, autosubmit bool) error {
	// Lock the screen.
	if err := lockscreen.Lock(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to lock the screen")
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
		return errors.Wrapf(err, "waiting for screen to be locked failed (last status %+v)", st)
	}

	// Enter and submit the PIN to unlock the DUT.
	if err := lockscreen.EnterPIN(ctx, tconn, keyboard, PIN); err != nil {
		return errors.Wrap(err, "failed to enter PIN")
	}

	if !autosubmit {
		if err := lockscreen.SubmitPINOrPassword(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to submit PIN")
		}
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return !st.Locked }, 30*time.Second); err != nil {
		return errors.Wrapf(err, "waiting for screen to be unlocked failed (last status %+v)", st)
	}
	return nil
}
