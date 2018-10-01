// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
	"context"
	"syscall"
	"time"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SupervisedUserCrash,
		Desc:         "Sign in, then crash.",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
	})
}

func SupervisedUserCrash(s *testing.State) {
	killChrome := func() {
		pid, err := chrome.GetRootPID()
		if err != nil {
			s.Fatal("Failed to find Chrome's root PID: ", err)
		}
		if err = syscall.Kill(pid, syscall.SIGKILL); err != nil {
			s.Fatal("Failed to kill Chrome.")
		}
	}

	ctx := s.Context()
	testRun := func(withARC bool) {
		sm, err := session.NewSessionManager(ctx)
		if err != nil {
			s.Fatal("Failed to connect session manager: ", err)
		}

		var cr *chrome.Chrome
		if withARC {
			cr, err = chrome.New(ctx, chrome.ARCEnabled())
		} else {
			cr, err = chrome.New(ctx)
		}
		if err != nil {
			s.Fatal("Failed to login Chrome: ", err)
		}
		defer cr.Close(ctx)

		// Tell session_manager that we're going all the way through
		// creating a supervised user.
		if err = sm.HandleSupervisedUserCreationStarting(ctx); err != nil {
			s.Fatal("Failed supervised user creation D-Bus call: ", err)
		}
		if err = sm.HandleSupervisedUserCreationFinished(ctx); err != nil {
			s.Fatal("Failed supervised user creation D-Bus call: ", err)
		}

		// Crashing the browser should not end the session, as creating
		// the user is finished.
		killChrome()

		// We should still be able to talk to the session_manager,
		// and it should indicate that we're still inside a user
		// session.
		if state, err := sm.RetrieveSessionState(ctx); err != nil {
			s.Fatal("Failed to retrieve session state: ", err)
		} else if state != "started" {
			s.Fatal("Session should not have ended: ", state)
		}

		// Start watching to stop signal before the session gets killed.
		watcher, err := sm.WatchSessionStateChanged(ctx, "stopped")
		if err != nil {
			s.Fatal("")
		}
		defer watcher.Close(ctx)

		// Tell session_manager that a supervised user is being set up,
		// and kill it in the middle. Session should die.
		if err = sm.HandleSupervisedUserCreationStarting(ctx); err != nil {
			s.Fatal("Failed supervise user creation D-Bus call: ", err)
		}
		killChrome()

		ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
		defer cancel()
		select {
		case <-watcher.Signals:
		case <-ctx.Done():
			s.Error("Timed out. SessionState didn't switch to \"stopped\"")
		}
	}

	testRun(false /* without ARC */)
	testRun(true /* with ARC */)
}
