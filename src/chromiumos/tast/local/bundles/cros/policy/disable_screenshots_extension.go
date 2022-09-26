// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

const disableScreenshotsExtensionHTML = "disable_screenshots_extension.html"

var extensionFiles = []string{
	"screen_shooter_extension/background.js",
	"screen_shooter_extension/content.js",
	"screen_shooter_extension/manifest.json",
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DisableScreenshotsExtension,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Behavior of the DisableScreenshots policy, check whether screenshot can be taken by chrome.tabs.captureVisibleTab extensions API",
		Contacts: []string{
			"poromov@google.com", // Policy owner
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         append(extensionFiles, disableScreenshotsExtensionHTML),
		// 2 minutes is the default local test timeout. Check localTestTimeout constant in tast/src/chromiumos/tast/internal/bundle/local.go.
		Timeout: chrome.ManagedUserLoginTimeout + 2*time.Minute,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.DisableScreenshots{}, pci.VerifiedFunctionalityJS),
		},
	})
}

func DisableScreenshotsExtension(ctx context.Context, s *testing.State) {
	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	extDir, err := ioutil.TempDir("", "screen_shooter_extension")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(extDir)

	if err := os.Chown(extDir, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
		s.Fatalf("Failed to chown %q dir: %v", extDir, err)
	}

	for _, file := range extensionFiles {
		dst := filepath.Join(extDir, filepath.Base(file))
		if err := fsutil.CopyFile(s.DataPath(file), dst); err != nil {
			s.Fatalf("Failed to copy %q file to %q: %v", file, extDir, err)
		}
	}

	fdms, err := fakedms.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}
	defer fdms.Stop(ctx)

	if err := fdms.WritePolicyBlob(policy.NewBlob()); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	cr, err := chrome.New(ctx,
		chrome.UnpackedExtension(extDir),
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.CustomLoginTimeout(chrome.ManagedUserLoginTimeout))
	if err != nil {
		s.Fatal("Failed to create Chrome instance: ", err)
	}
	defer cr.Close(ctx)

	for _, tc := range []struct {
		name      string
		value     []policy.Policy
		wantTitle string
	}{
		{
			name:      "true",
			value:     []policy.Policy{&policy.DisableScreenshots{Val: true}},
			wantTitle: "Taking screenshots has been disabled",
		},
		{
			name:      "false",
			value:     []policy.Policy{&policy.DisableScreenshots{Val: false}},
			wantTitle: "Screen capture allowed",
		},
		{
			name:      "unset",
			value:     []policy.Policy{},
			wantTitle: "Screen capture allowed",
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			// This is needed only for debugging purpose to understand why the test is flaky.
			// TODO(crbug:1159824): Remove once test will be stable.
			defer func(ctx context.Context) {
				if s.HasError() {
					if err := screenshot.Capture(ctx, filepath.Join(s.OutDir(), fmt.Sprintf("screenshot_%s.png", tc.name))); err != nil {
						s.Error("Failed to capture screenshot: ", err)
					}
				}
			}(cleanupCtx)

			// Minimum interval between captureVisibleTab requests is 1 second, so we
			// must sleep for 1 seconds to be able to take screenshot,
			// otherwise API will return an error.
			//
			// Please check MAX_CAPTURE_VISIBLE_TAB_CALLS_PER_SECOND constant in
			// chrome/common/extensions/api/tabs.json
			if err := testing.Sleep(ctx, time.Second); err != nil {
				s.Fatal("Failed to sleep: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, tc.value); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			conn, err := cr.NewConn(ctx, server.URL+"/"+disableScreenshotsExtensionHTML)
			if err != nil {
				s.Fatal("Failed to create a tab: ", err)
			}
			defer conn.Close()

			// Wait until page has desired title to avoid race conditions.
			if err := conn.WaitForExpr(ctx, `document.title === "Page Title"`); err != nil {
				s.Fatal("Failed to execute JS in extension: ", err)
			}

			// Here we check only `chrome.tabs.captureVisibleTab` extension API.
			if err := conn.Eval(ctx, `document.title = "captureVisibleTab"`, nil); err != nil {
				s.Fatal("Failed to execute JS in extension: ", err)
			}

			if err := keyboard.Accel(ctx, "Ctrl+Shift+Y"); err != nil {
				s.Fatal("Failed to press Ctrl+Shift+Y: ", err)
			}

			if err := conn.WaitForExpr(ctx, `document.title != "captureVisibleTab"`); err != nil {
				s.Fatal("Failed to execute JS in extension: ", err)
			}

			var title string
			if err := conn.Eval(ctx, `document.title`, &title); err != nil {
				s.Fatal("Failed to execute JS in extension: ", err)
			}
			if title != tc.wantTitle {
				s.Errorf("Unexpected title: get %q; want %q", title, tc.wantTitle)
			}
		})
	}
}
