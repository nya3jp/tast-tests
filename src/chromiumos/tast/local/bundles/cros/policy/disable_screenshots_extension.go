// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

var extensionFiles = []string{
	"screen_shooter_extension/background.js",
	"screen_shooter_extension/content.js",
	"screen_shooter_extension/manifest.json",
}

func init() {
	testing.AddTest(&testing.Test{
		Func: DisableScreenshotsExtension,
		Desc: "Behavior of the DisableScreenshots policy, check whether screenshot can be taken by extensions APIs",
		Contacts: []string{
			"lamzin@google.com", // Test port author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         extensionFiles,
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

	if err := fdms.WritePolicyBlob(fakedms.NewPolicyBlob()); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	cr, err := chrome.New(ctx,
		chrome.UnpackedExtension(extDir),
		chrome.Auth(pre.Username, pre.Password, pre.GaiaID),
		chrome.DMSPolicy(fdms.URL))
	if err != nil {
		s.Fatal("Failed to create Chrome instance: ", err)
	}
	defer cr.Close(ctx)

	conn, err := cr.NewConn(ctx, "https://google.com")
	if err != nil {
		s.Fatal("Failed to create a tab: ", err)
	}
	defer conn.Close()

	for _, tc := range []struct {
		name      string
		value     []policy.Policy
		wantTitle string
	}{
		{
			name:      "true",
			value:     []policy.Policy{&policy.DisableScreenshots{Val: true}},
			wantTitle: "screen capture not allowed",
		},
		{
			name:      "false",
			value:     []policy.Policy{&policy.DisableScreenshots{Val: false}},
			wantTitle: "screen capture allowed",
		},
		{
			name:      "unset",
			value:     []policy.Policy{},
			wantTitle: "screen capture allowed",
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, tc.value); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Here we check only `chrome.tabs.captureVisibleTab` extension API.
			// TODO(crbug.com/839630, crbug.com/817497): check whether DisableScreenshots should affect
			// `chrome.tabCapture.capture` and `chrome.desktopCapture.chooseDesktopMedia` extension APIs.
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
