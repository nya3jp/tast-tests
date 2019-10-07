// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeLogin,
		Desc:         "Checks that Chrome supports login",
		Contacts:     []string{"chromeos-ui@google.com"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: 1,
		}, {
			Name:      "stress",
			Val:       50,
			ExtraAttr: []string{"group:stress"},
		}},
		Timeout: 50 * time.Minute,
	})
}

func ChromeLogin(ctx context.Context, s *testing.State) {
	sm := func() *session.SessionManager {
		// Set up the test environment. Should be done quickly.
		const setupTimeout = 30 * time.Second
		setupCtx, cancel := context.WithTimeout(ctx, setupTimeout)
		defer cancel()

		// Ensures login screen.
		if err := upstart.RestartJob(setupCtx, "ui"); err != nil {
			s.Fatal("Chrome logout failed: ", err)
		}

		sm, err := session.NewSessionManager(setupCtx)
		if err != nil {
			s.Fatal("Failed to connect session_manager: ", err)
		}
		return sm
	}()

	const content = "Hooray, it worked!"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, content)
	}))
	defer server.Close()

	numTrial := s.Param().(int)
	for i := 0; i < numTrial; i++ {
		if numTrial > 1 {
			s.Logf("Trial %d/%d", i+1, numTrial)
		}

		testChromeLogin(ctx, s, sm, server.URL, content)
	}
}

func testChromeLogin(ctx context.Context, s *testing.State, sm *session.SessionManager, url, expected string) {
	const timeoutPerRun = time.Minute
	ctx, cancel := context.WithTimeout(ctx, timeoutPerRun)
	defer cancel()

	func() {
		// Start listening for a "started" SessionStateChanged D-Bus signal from session_manager.
		sw, err := sm.WatchSessionStateChanged(ctx, "started")
		if err != nil {
			s.Fatal("Failed to watch for D-Bus signals: ", err)
		}
		defer sw.Close(ctx)

		cr, err := chrome.New(ctx)
		if err != nil {
			s.Fatal("Chrome login failed: ", err)
		}
		defer cr.Close(ctx)

		s.Log("Waiting for SessionStateChanged \"started\" D-Bus signal from session_manager")
		select {
		case <-sw.Signals:
			s.Log("Got SessionStateChanged signal")
		case <-ctx.Done():
			s.Fatal("Didn't get SessionStateChanged signal: ", ctx.Err())
		}

		conn, err := cr.NewConn(ctx, url)
		if err != nil {
			s.Fatal("Creating renderer failed: ", err)
		}
		defer conn.Close()

		if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
			s.Fatal("Waiting load failed: ", err)
		}

		var actual string
		if err := conn.Eval(ctx, "document.documentElement.innerText", &actual); err != nil {
			s.Fatal("Getting page content failed: ", err)
		}
		if actual != expected {
			s.Fatalf("Unexpected page content: got %q; want %q", actual, expected)
		}
	}()

	sw, err := sm.WatchSessionStateChanged(ctx, "stopped")
	if err != nil {
		s.Fatal("Failed to watch for D-Bus signals: ", err)
	}
	defer sw.Close(ctx)

	// Emulate logout.
	if err = upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Chrome logout failed: ", err)
	}

	s.Log("Waiting for SessionStateChanged \"stopped\" D-Bus signal from session_manager")
	select {
	case <-sw.Signals:
		s.Log("Got SessionStateChanged signal")
	case <-ctx.Done():
		s.Fatal("Didn't get SessionStateChanged signal: ", ctx.Err())
	}
}
