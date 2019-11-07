// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"golang.org/x/sync/errgroup"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChromeSanity,
		Desc: "Sanity tests for the chrome support library",
		Contacts: []string{
			"nya@chromium.org",
			"tast-owners@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Timeout:      30 * time.Second,
	})
}

func ChromeSanity(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	testConcurrentTabs(ctx, s, cr)
}

// testConcurrentTabs opens and closes tabs concurrently and checks errors.
func testConcurrentTabs(ctx context.Context, s *testing.State, cr *chrome.Chrome) {
	const (
		timeout     = 10 * time.Second
		concurrency = 10
		pageText    = "OK"
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, pageText)
	}))
	defer srv.Close()

	done := make(chan struct{}) // closed on timeout
	go func() {
		testing.Sleep(ctx, timeout)
		close(done)
	}()

	// Start goroutines that opens and closes tabs concurrently.
	g, ctx := errgroup.WithContext(ctx)
	for i := 0; i < concurrency; i++ {
		g.Go(func() error {
			for {
				// Check if timeout is already passed.
				select {
				case <-done:
					return nil
				default:
				}

				if err := func() (retErr error) {
					conn, err := cr.NewConn(ctx, srv.URL)
					if err != nil {
						return errors.Wrap(err, "Chrome.NewConn failed")
					}
					defer func() {
						if err := conn.Close(); err != nil && retErr == nil {
							retErr = errors.Wrap(err, "Conn.Close failed")
						}
					}()

					var s string
					if err := conn.Eval(ctx, "document.documentElement.innerText", &s); err != nil {
						return errors.Wrap(err, "Conn.Eval failed")
					}
					if s != pageText {
						return errors.Errorf("unexpected page context: got %q, want %q", s, pageText)
					}

					if err := conn.CloseTarget(ctx); err != nil {
						return errors.Wrap(err, "Conn.CloseTarget failed")
					}
					return nil
				}(); err != nil {
					return err
				}
			}
		})
	}

	if err := g.Wait(); err != nil {
		s.Error("testConcurrentTabs failed: ", err)
	}
}
