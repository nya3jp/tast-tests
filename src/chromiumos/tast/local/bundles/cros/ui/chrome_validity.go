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
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/browser/browserutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeValidity,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Validity tests for the chrome support library",
		Contacts: []string{
			"nya@chromium.org",
			"tast-owners@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      30 * time.Second,
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraAttr:         []string{"informational"},
			Val:               browser.TypeLacros,
		}},
	})
}

func ChromeValidity(ctx context.Context, s *testing.State) {
	bt := s.Param().(browser.Type)
	br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), bt)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}

	s.Log("Opening a new blank window to see if browserutil.CloseAboutBlank can close one")
	testing.Sleep(ctx, 3*time.Second)
	br.NewConn(ctx, chrome.BlankURL, browser.WithNewWindow())

	s.Log("browserutil.WaitForWindow starts")
	testing.Sleep(ctx, 3*time.Second)
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get the connection to the test API: ", err)
	}
	err = browserutil.WaitForWindow(ctx, tconn, bt, "")
	if err != nil {
		s.Fatal("Failed to wait for the browser to open: ", err)
	}

	s.Log("browserutil.CloseAboutBlank starts")
	testing.Sleep(ctx, 3*time.Second)
	browserutil.CloseAboutBlank(ctx, br, s.Param().(browser.Type))

	defer closeBrowser(ctx)
	testConcurrentTabs(ctx, s, br)
}

// testConcurrentTabs opens and closes tabs concurrently and checks errors.
func testConcurrentTabs(ctx context.Context, s *testing.State, br *browser.Browser) {
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
					conn, err := br.NewConn(ctx, srv.URL)

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
