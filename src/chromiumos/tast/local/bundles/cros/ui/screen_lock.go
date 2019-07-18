// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenLock,
		Desc:         "Checks that screen-locking works in Chrome",
		Contacts:     []string{"chromeos-ui@google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
	})
}

func ScreenLock(ctx context.Context, s *testing.State) {
	const (
		username      = "testuser@gmail.com"
		password      = "good"
		wrongPassword = "bad"
		gaiaID        = "1234"

		lockTimeout     = 30 * time.Second
		goodAuthTimeout = 30 * time.Second
		// Attempting to unlock with the wrong password can block for up to ~3 minutes
		// if the TPM is busy doing RSA keygen: https://crbug.com/937626
		badAuthTimeout = 3 * time.Minute
	)

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed creating virtual keyboard: ", err)
	}
	defer kb.Close()

	cr, err := chrome.New(ctx, chrome.Auth(username, password, gaiaID))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	conn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test API connection failed: ", err)
	}

	// lockState contains a subset of the state returned by chrome.autotestPrivate.loginStatus.
	type lockState struct {
		Locked bool `json:"isScreenLocked"`
		Ready  bool `json:"isReadyForPassword"`
	}

	// waitStatus repeatedly calls chrome.autotestPrivate.loginStatus and passes the returned
	// state to f until it returns true or timeout is reached. The last-seen state is returned.
	waitStatus := func(f func(st lockState) bool, timeout time.Duration) (lockState, error) {
		var st lockState
		err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := conn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.loginStatus)()`, &st); err != nil {
				return err
			} else if !f(st) {
				return errors.New("wrong status")
			}
			return nil
		}, &testing.PollOptions{Timeout: timeout})
		return st, err
	}

	const accel = "Search+L"
	s.Log("Locking screen via ", accel)
	if err := kb.Accel(ctx, accel); err != nil {
		s.Fatalf("Typing %v failed: %v", accel, err)
	}
	s.Log("Waiting for Chrome to report that screen is locked")
	_, lockStage := timing.Start(ctx, "lock_screen") // don't assign to ctx; there's no child stage
	if st, err := waitStatus(func(st lockState) bool { return st.Locked && st.Ready }, lockTimeout); err != nil {
		s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
	}
	lockStage.End()

	s.Log("Typing wrong password")
	if err := kb.Type(ctx, wrongPassword+"\n"); err != nil {
		s.Fatal("Typing wrong password failed: ", err)
	}
	s.Log("Waiting for lock screen to respond to wrong password (can block if TPM is busy)")
	if st, err := waitStatus(func(st lockState) bool { return !st.Locked || st.Ready }, badAuthTimeout); err != nil {
		s.Fatalf("Waiting for response to wrong password failed: %v (last status %+v)", err, st)
	} else if !st.Locked {
		s.Fatalf("Was able to unlock screen by typing wrong password: %+v", st)
	}

	s.Log("Unlocking screen by typing correct password")
	if err := kb.Type(ctx, password+"\n"); err != nil {
		s.Fatal("Typing correct password failed: ", err)
	}
	s.Log("Waiting for Chrome to report that screen is unlocked")
	_, unlockStage := timing.Start(ctx, "unlock_screen") // don't assign to ctx; there's no child stage
	if st, err := waitStatus(func(st lockState) bool { return !st.Locked }, goodAuthTimeout); err != nil {
		s.Fatalf("Waiting for screen to be unlocked failed: %v (last status %+v)", err, st)
	}
	unlockStage.End()
}
