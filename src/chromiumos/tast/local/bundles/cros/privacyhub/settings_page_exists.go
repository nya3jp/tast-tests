// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package privacyhub contains tests for privacy hub
package privacyhub

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SettingsPageExists,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that PrivacyHub settings page exists",
		Contacts:     []string{"privacy-hub@google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Attr:         []string{"group:mainline", "informational"},
	})
}

func getSessionManager(ctx context.Context, s *testing.State) *session.SessionManager {
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
		s.Fatal("Failed to connect session manager: ", err)
	}
	return sm
}

type testFunc func(ctx context.Context, s *testing.State, sm *session.SessionManager)

func startChrome(ctx context.Context, s *testing.State, sm *session.SessionManager, extra ...string) *chrome.Chrome {
	// Start listening for a "started" SessionStateChanged D-Bus signal from session_manager.
	sw, err := sm.WatchSessionStateChanged(ctx, "started")
	if err != nil {
		s.Fatal("Failed to watch for D-Bus signals: ", err)
	}
	defer sw.Close(ctx)

	crExtraArgs := make([]string, 0, cap(extra))
	crExtraArgs = append(crExtraArgs, "--vmodule=login_display_host*=4,oobe_ui=4")
	crExtraArgs = append(crExtraArgs, extra...)
	cr, err := chrome.New(ctx, chrome.ExtraArgs(crExtraArgs...))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	s.Log("Waiting for SessionStateChanged \"started\" D-Bus signal from session_manager")
	select {
	case <-sw.Signals:
		s.Log("Got SessionStateChanged signal")
	case <-ctx.Done():
		s.Fatal("Didn't get SessionStateChanged signal: ", ctx.Err())
	}

	return cr
}

func openSettings(ctx context.Context, s *testing.State, cr *chrome.Chrome) *chrome.Conn {
	const url = "chrome://os-settings/"
	conn := func() *chrome.Conn {
		conn, err := apps.LaunchOSSettings(ctx, cr, url)
		if err != nil {
			s.Fatal("Failed to open page at ", url, ": ", err)
		}
		return conn
	}()
	if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		s.Fatal("Waiting load failed: ", err)
	}
	return conn
}

func privacyHubSubSectionExists(ctx context.Context, s *testing.State, sm *session.SessionManager, addFlag bool) bool {
	extraArgs := make([]string, 0, 1)
	if addFlag {
		extraArgs = append(extraArgs, "--enable-features=CrosPrivacyHub")
	}
	cr := startChrome(ctx, s, sm, extraArgs...)
	defer cr.Close(ctx)

	conn := openSettings(ctx, s, cr)
	defer conn.Close()

	var elements []string
	if err := webutil.EvalWithShadowPiercer(ctx, conn, `[...shadowPiercingQueryAll("div[aria-hidden=\"true\"]")].map(x=>x.innerHTML)`, &elements); err != nil {
		s.Fatal("Failed to read elements from Settings page: ", err)
	}

	var found bool = false
	for _, text := range elements {
		if strings.Contains(text, "Privacy Hub") {
			found = true
			break
		}
	}

	return found
}

func test(ctx context.Context, s *testing.State, sm *session.SessionManager) {

	if privacyHubSubSectionExists(ctx, s, sm, false) {
		s.Error("Privacy Hub sub section found without flag set")
	}

	if !privacyHubSubSectionExists(ctx, s, sm, true) {
		s.Error("Didn't find Privacy Hub sub section")
	}

}

func testInSession(ctx context.Context, s *testing.State, test testFunc) {
	sm := getSessionManager(ctx, s)

	test(ctx, s, sm)

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

func SettingsPageExists(ctx context.Context, s *testing.State) {
	testInSession(ctx, s, test)
}
