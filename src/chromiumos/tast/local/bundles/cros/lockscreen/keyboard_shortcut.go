// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lockscreen

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KeyboardShortcut,
		Desc:         "Checks that screen-locking works by keyboard shortcut",
		Contacts:     []string{"chromeos-ui@google.com", "chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Attr:         []string{"group:mainline"},
	})
}

func KeyboardShortcut(ctx context.Context, s *testing.State) {
	const (
		username      = "testuser@gmail.com"
		password      = "good"
		wrongPassword = "bad"

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

	cr, err := chrome.New(ctx, chrome.FakeLogin(chrome.Creds{User: username, Pass: password}))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	conn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test API connection failed: ", err)
	}

	const accel = "Search+L"
	s.Log("Locking screen via ", accel)
	if err := kb.Accel(ctx, accel); err != nil {
		s.Fatalf("Typing %v failed: %v", accel, err)
	}
	s.Log("Waiting for Chrome to report that screen is locked")
	_, lockStage := timing.Start(ctx, "lock_screen") // don't assign to ctx; there's no child stage
	if st, err := lockscreen.WaitState(ctx, conn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, lockTimeout); err != nil {
		s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
	}
	lockStage.End()

	s.Log("Typing wrong password")
	if err := kb.Type(ctx, wrongPassword+"\n"); err != nil {
		s.Fatal("Typing wrong password failed: ", err)
	}
	s.Log("Waiting for lock screen to respond to wrong password (can block if TPM is busy)")
	if st, err := lockscreen.WaitState(ctx, conn, func(st lockscreen.State) bool { return !st.Locked || st.ReadyForPassword }, badAuthTimeout); err != nil {
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
	if st, err := lockscreen.WaitState(ctx, conn, func(st lockscreen.State) bool { return !st.Locked }, goodAuthTimeout); err != nil {
		s.Fatalf("Waiting for screen to be unlocked failed: %v (last status %+v)", err, st)
	}
	unlockStage.End()
}
