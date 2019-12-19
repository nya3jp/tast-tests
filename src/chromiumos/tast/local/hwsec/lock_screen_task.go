// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type stressTaskLockScreen struct {
	ctx    context.Context
	conn   *chrome.TestConn
	passwd string
}

// NewStressTaskLockScreen creates a new |stressTaskLockScreen| that implements the task of lock-the-unlock-screen task.
func NewStressTaskLockScreen(ctx context.Context, conn *chrome.TestConn, passwd string) *stressTaskLockScreen {
	return &stressTaskLockScreen{ctx, conn, passwd}
}

// RunTask implements the one of StressTaskRunner.
func (r *stressTaskLockScreen) RunTask(ctx context.Context) error {
	const (
		lockTimeout     = 30 * time.Second
		goodAuthTimeout = 30 * time.Second
	)
	kb, err := input.VirtualKeyboard(r.ctx)
	if err != nil {
		return errors.Wrap(err, "failed creating virtual keyboard")
	}
	defer kb.Close()

	if err != nil {
		errors.Wrap(err, "getting test API connection failed")
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
		err := testing.Poll(r.ctx, func(ctx context.Context) error {
			if err := r.conn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.loginStatus)()`, &st); err != nil {
				return err
			} else if !f(st) {
				return errors.New("wrong status")
			}
			return nil
		}, &testing.PollOptions{Timeout: timeout})
		return st, err
	}

	const accel = "Search+L"
	if err := kb.Accel(r.ctx, accel); err != nil {
		return errors.Wrap(err, "failed to lock screen by typeing "+accel)
	}

	// Waits for Chrome to report that screen is locked.
	if st, err := waitStatus(func(st lockState) bool { return st.Locked && st.Ready }, lockTimeout); err != nil {
		return errors.Wrapf(err, "waiting for screen to be locked failed with last status %+v", st)
	}

	// Unlocking screen by typing correct password.
	if err := kb.Type(r.ctx, r.passwd+"\n"); err != nil {
		return errors.Wrap(err, "failed to type password")
	}

	// Waiting for Chrome to report that screen is unlocked.
	if st, err := waitStatus(func(st lockState) bool { return !st.Locked }, goodAuthTimeout); err != nil {
		return errors.Wrapf(err, "waiting for screen to be unlocked failed with last status %+v", st)
	}
	return nil
}
