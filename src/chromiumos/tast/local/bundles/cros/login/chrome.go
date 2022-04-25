// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

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
		Func:         Chrome,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that Chrome supports login",
		Contacts:     []string{"cros-oac@google.com", "rsorokin@google.com", "chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:       1,
			ExtraAttr: []string{"group:mainline"},
			Timeout:   chrome.LoginTimeout + 45*time.Second,
		}, {
			Name:      "stress",
			Val:       50,
			ExtraAttr: []string{"group:stress"},
			Timeout:   50*chrome.LoginTimeout + 45*time.Second,
		}, {
			Name:    "forever",
			Val:     1000000,
			Timeout: 365 * 24 * time.Hour,
		}},
	})
}

func Chrome(ctx context.Context, s *testing.State) {
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

		testChromeLoginInSession(ctx, s, sm, server.URL, content)
	}
}

func testChromeLoginInSession(ctx context.Context, s *testing.State, sm *session.SessionManager, url, expected string) {
	testChromeLogin(ctx, s, sm, url, expected)

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

func testChromeLogin(ctx context.Context, s *testing.State, sm *session.SessionManager, url, expected string) {
	// Start listening for a "started" SessionStateChanged D-Bus signal from session_manager.
	sw, err := sm.WatchSessionStateChanged(ctx, "started")
	if err != nil {
		s.Fatal("Failed to watch for D-Bus signals: ", err)
	}
	defer sw.Close(ctx)

	cr, err := chrome.New(ctx, chrome.ExtraArgs("--vmodule=login_display_host*=4,oobe_ui=4"))
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
		s.Fatalf("Failed to connect to %v: %v", url, err)
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
}
