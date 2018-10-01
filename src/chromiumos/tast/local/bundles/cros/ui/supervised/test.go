// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package supervised

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func killChrome() error {
	pid, err := chrome.GetRootPID()
	if err != nil {
		return err
	}
	if err = syscall.Kill(pid, syscall.SIGKILL); err != nil {
		return fmt.Errorf("failed to kill Chrome: %v", err)
	}
	return nil
}

// RunTest exersises the session_manager states on crashing during
// supervised user creation.
func RunTest(ctx context.Context, s *testing.State) {
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to connect to session manager: ", err)
	}

	// Tell session_manager that we're going all the way through creating
	// a supervised user.
	if err = sm.HandleSupervisedUserCreationStarting(ctx); err != nil {
		s.Fatal("Failed supervised user creation starting D-Bus call: ", err)
	}
	if err = sm.HandleSupervisedUserCreationFinished(ctx); err != nil {
		s.Fatal("Failed supervised user creation finishing D-Bus call: ", err)
	}

	// Crashing the browser should not end the session, as creating
	// the user is finished.
	if err = killChrome(); err != nil {
		s.Fatal("Failed to crash Chrome after user creation: ", err)
	}

	// We should still be able to talk to the session_manager, and it
	// should indicate that we're still inside a usersession.
	if state, err := sm.RetrieveSessionState(ctx); err != nil {
		s.Fatal("Failed to retrieve session state: ", err)
	} else if state != "started" {
		s.Fatalf("Session has state %q instead of \"started\"", state)
	}

	// Start watching to stop signal before the session gets killed.
	watcher, err := sm.WatchSessionStateChanged(ctx, "stopped")
	if err != nil {
		s.Fatal("Failed to start watching SessionStateChanged signal: ", err)
	}
	defer watcher.Close(ctx)

	// Tell session_manager that a supervised user is being set up,
	// and kill it in the middle. Session should die.
	if err = sm.HandleSupervisedUserCreationStarting(ctx); err != nil {
		s.Fatal("Failed supervised user creation D-Bus call: ", err)
	}
	if err = killChrome(); err != nil {
		s.Fatal("Failed to crash Chrome during user creation: ", err)
	}

	select {
	case <-watcher.Signals:
	case <-time.After(1 * time.Minute):
		s.Error("Timed out. SessionState didn't switch to \"stopped\"")
	}
}
