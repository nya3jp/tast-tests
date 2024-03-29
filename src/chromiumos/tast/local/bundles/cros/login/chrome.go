// Copyright 2017 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

type chromeTestParams struct {
	numTrial int
	opts     []chrome.Option
	bt       browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Chrome,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that Chrome supports login",
		Contacts: []string{
			"bohdanty@google.com",
			"rrsilva@google.com",
			"cros-oac@google.com",
			"chromeos-sw-engprod@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: chromeTestParams{
				numTrial: 1,
				bt:       browser.TypeAsh},
			ExtraAttr: []string{"group:mainline"},
			Timeout:   chrome.LoginTimeout + 45*time.Second,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val: chromeTestParams{
				numTrial: 1,
				bt:       browser.TypeLacros},
			ExtraAttr: []string{"group:mainline", "informational"},
			Timeout:   chrome.LoginTimeout + 45*time.Second,
		}, {
			Name: "auth_factor_experiment_on",
			Val: chromeTestParams{
				numTrial: 1,
				opts:     []chrome.Option{chrome.EnableFeatures("UseAuthFactors")},
				bt:       browser.TypeAsh},
			ExtraAttr: []string{"group:mainline", "informational"},
			Timeout:   chrome.LoginTimeout + 45*time.Second,
		}, {
			Name:              "auth_factor_experiment_on_lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val: chromeTestParams{
				numTrial: 1,
				opts:     []chrome.Option{chrome.EnableFeatures("UseAuthFactors")},
				bt:       browser.TypeLacros},
			ExtraAttr: []string{"group:mainline", "informational"},
			Timeout:   chrome.LoginTimeout + 45*time.Second,
		}, {
			Name: "auth_factor_experiment_off",
			Val: chromeTestParams{
				numTrial: 1,
				opts:     []chrome.Option{chrome.DisableFeatures("UseAuthFactors")},
				bt:       browser.TypeAsh},
			ExtraAttr: []string{"group:mainline", "informational"},
			Timeout:   chrome.LoginTimeout + 45*time.Second,
		}, {
			Name:              "auth_factor_experiment_off_lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val: chromeTestParams{
				numTrial: 1,
				opts:     []chrome.Option{chrome.DisableFeatures("UseAuthFactors")},
				bt:       browser.TypeLacros},
			ExtraAttr: []string{"group:mainline", "informational"},
			Timeout:   chrome.LoginTimeout + 45*time.Second,
		}, {
			Name: "stress",
			Val: chromeTestParams{
				numTrial: 50,
				bt:       browser.TypeAsh},
			ExtraAttr: []string{"group:stress"},
			Timeout:   50*chrome.LoginTimeout + 45*time.Second,
		}, {
			Name:              "stress_lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val: chromeTestParams{
				numTrial: 50,
				bt:       browser.TypeLacros},
			ExtraAttr: []string{"group:stress"},
			Timeout:   50*chrome.LoginTimeout + 45*time.Second,
		}, {
			Name: "forever",
			Val: chromeTestParams{
				numTrial: 1000000,
				bt:       browser.TypeAsh},
			Timeout: 365 * 24 * time.Hour,
		}, {
			Name:              "forever_lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val: chromeTestParams{
				numTrial: 1000000,
				bt:       browser.TypeLacros},
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

	// Added a slash to url because when using browserfixt the given url must match
	// exactly the URL that Chrome ends up associating with the tab.
	localWebURL := server.URL + "/"

	params := s.Param().(chromeTestParams)
	for i := 0; i < params.numTrial; i++ {
		if params.numTrial > 1 {
			s.Logf("Trial %d/%d", i+1, params.numTrial)
		}

		testChromeLoginInSession(ctx, s, sm, &params, localWebURL, content)
	}
}

func testChromeLoginInSession(ctx context.Context, s *testing.State, sm *session.SessionManager, params *chromeTestParams, url, expected string) {
	testChromeLogin(ctx, s, sm, params, url, expected)

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

func testChromeLogin(ctx context.Context, s *testing.State, sm *session.SessionManager, params *chromeTestParams, url, expected string) {
	// Reserve a few seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 10*time.Second)
	defer cancel()

	// Start listening for a "started" SessionStateChanged D-Bus signal from session_manager.
	sw, err := sm.WatchSessionStateChanged(ctx, "started")
	if err != nil {
		s.Fatal("Failed to watch for D-Bus signals: ", err)
	}
	defer sw.Close(cleanupCtx)

	// Connect to a fresh ash-chrome instance (cr).
	opts := append([]chrome.Option{chrome.ExtraArgs("--vmodule=login_display_host*=4,oobe_ui=4")}, params.opts...)
	cr, err := browserfixt.NewChrome(ctx, params.bt, lacrosfixt.NewConfig(), opts...)
	if err != nil {
		s.Fatalf("Chrome login failed with %v browser: %v", params.bt, err)
	}
	defer cr.Close(cleanupCtx)

	s.Log("Waiting for SessionStateChanged \"started\" D-Bus signal from session_manager")
	select {
	case <-sw.Signals:
		s.Log("Got SessionStateChanged signal")
	case <-ctx.Done():
		s.Fatal("Didn't get SessionStateChanged signal: ", ctx.Err())
	}

	// Opening single web content.
	conn, _, closeBrowser, err := browserfixt.SetUpWithURL(ctx, cr, params.bt, url)
	if err != nil {
		s.Fatal("Failed to open a new connection: ", err)
	}
	defer closeBrowser(cleanupCtx)
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
