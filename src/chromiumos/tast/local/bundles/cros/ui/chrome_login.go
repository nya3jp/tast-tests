// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeLogin,
		Desc:         "Checks that Chrome supports login",
		SoftwareDeps: []string{"chrome_login"},
	})
}

func ChromeLogin(s *testing.State) {
	ctx := s.Context()

	// Start listening for a "started" SessionStateChanged D-Bus signal from session_manager.
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to connect session_manager: ", err)
	}
	sw, err := sm.WatchSessionStateChanged(ctx, "started")
	if err != nil {
		s.Fatal("Failed to watch for D-Bus signals: ", err)
	}
	defer sw.Close(ctx)

	cr, err := chrome.New(ctx)
	if err != nil {
		cerr := err // save to pass to s.Fatal later

		saveFile := func(p string) error {
			sf, err := os.Open(p)
			if err != nil {
				return err
			}
			defer sf.Close()

			df, err := os.Create(filepath.Join(s.OutDir(), filepath.Base(p)))
			if err != nil {
				return err
			}
			defer df.Close()

			_, err = io.Copy(df, sf)
			return err
		}
		// TODO(crbug.com/850139): Stop collecting these files after fixing IsGuestSessionAllowed segfaults.
		ps, _ := filepath.Glob("/var/lib/whitelist/policy.*")
		for _, p := range append(ps, "/home/chronos/Local State") {
			if err = saveFile(p); err != nil {
				s.Errorf("Failed to save %s: %v", p, err)
			}
		}

		s.Fatal("Chrome login failed: ", cerr)
	}
	defer cr.Close(ctx)

	s.Log("Waiting for SessionStateChanged \"started\" D-Bus signal from session_manager")
	select {
	case <-sw.Signals:
		s.Log("Got SessionStateChanged signal")
	case <-ctx.Done():
		s.Fatal("Didn't get SessionStateChanged signal: ", ctx.Err())
	}

	conn, err := cr.NewConn(ctx, "")
	if err != nil {
		s.Fatal("Creating renderer failed: ", err)
	}
	defer conn.Close()

	const expected = "Hooray, it worked!"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, expected)
	}))
	defer server.Close()

	if err = conn.Navigate(ctx, server.URL); err != nil {
		s.Fatalf("Navigating to %s failed: %v", server.URL, err)
	}
	var actual string
	if err = conn.Eval(ctx, "document.documentElement.innerText", &actual); err != nil {
		s.Fatal("Getting page content failed: ", err)
	}
	s.Logf("Got content %q", actual)
	if actual != expected {
		s.Fatalf("Expected page content %q, got %q", expected, actual)
	}
}
