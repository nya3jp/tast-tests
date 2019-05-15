// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeLogin,
		Desc:         "Checks that Chrome supports login",
		Contacts:     []string{"derat@chromium.org"},
		SoftwareDeps: []string{"chrome"},
	})
}

func ChromeLogin(ctx context.Context, s *testing.State) {
	// Start listening for a "started" SessionStateChanged D-Bus signal from session_manager.
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to connect session_manager: ", err)
	}
	func() {
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

		const expected = "Hooray, it worked!"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, expected)
		}))
		defer server.Close()

		conn, err := cr.NewConn(ctx, server.URL)
		if err != nil {
			s.Fatal("Creating renderer failed: ", err)
		}
		defer conn.Close()

		if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
			s.Fatal("Waiting load failed: ", err)
		}

		var actual string
		if err = conn.Eval(ctx, "document.documentElement.innerText", &actual); err != nil {
			s.Fatal("Getting page content failed: ", err)
		}
		s.Logf("Got content %q", actual)
		if actual != expected {
			s.Fatalf("Expected page content %q, got %q", expected, actual)
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
