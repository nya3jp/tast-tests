// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KeyboardShortcutViewerPainting,
		Desc:         "Checks that keyboard shortcut viewer paints",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

func KeyboardShortcutViewerPainting(ctx context.Context, s *testing.State) {
	const (
		// Must match one of ash's SHOW_SHORTCUT_VIEWER accelerators.
		ksvAccel = "Ctrl+Alt+/"
		// Client name of KSV of Window Service. Must match the service name
		// defined in shortcut_viewer.mojom.
		wsClientName = "shortcut_viewer_app"
		// Time out in milliseconds to wait for KSV to paint.
		timeOutMs = 10000
	)

	cr, err := chrome.New(ctx, chrome.ExtraArgs([]string{"--use-test-config"}))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	s.Log("Invoking ksv via accelerator")
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer ew.Close()
	ew.Accel(ctx, ksvAccel)

	s.Log("Checking ksv screen")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	expr := fmt.Sprintf(
		`new Promise((resolve, reject) => {
			chrome.autotestPrivate.ensureWindowServiceClientHasDrawnWindow(
				'%v', %v,
				(success) => {
					if (chrome.runtime.lastError === undefined) {
						if (success)
							resolve();
						else
							reject(new Error('KSV failed to draw any windows.'));
					} else {
						reject(new Error(chrome.runtime.lastError.message));
					}
				});
		})`, wsClientName, timeOutMs)
	if err := tconn.EvalPromise(ctx, expr, nil); err != nil {
		s.Fatal("Failed to check KSV painting: ", err)
	}
}
