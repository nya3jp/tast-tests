// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIPreview,
		Desc:         "Opens CCA and verifies the preview functions",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login", caps.BuiltinCamera},
		Data:         []string{"cca_ui.js"},
	})
}

func CCAUIPreview(ctx context.Context, s *testing.State) {
	cca.RunUITest(ctx, s, testPreview)
}

func testPreview(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.Conn, conn *cca.Conn) {
	if err := conn.CheckVideoIsActive(ctx); err != nil {
		s.Fatal("Failed to start preview: ", err)
	}
	s.Log("Preview started")

	testPreviewResize(ctx, s, conn)

	// TODO(shik): Add the missing preview tests in go/cca-test:
	// * Preview active after going back from gallery
	// * Preview active after taking picture
	// * Preview active after recording
	// * Preview active after suspend/resume
}

func testPreviewResize(ctx context.Context, s *testing.State, conn *cca.Conn) {
	runPreviewResizeTest(ctx, s, conn, func() {
		if err := conn.MaximizeWindow(ctx); err != nil {
			s.Fatal("Failed to maximize window: ", err)
		}
		s.Log("Window maximized")

		if err := conn.CheckVideoIsActive(ctx); err != nil {
			s.Fatal("Preview is inactive after maximize window: ", err)
		}
	})

	runPreviewResizeTest(ctx, s, conn, func() {
		if err := conn.FullscreenWindow(ctx); err != nil {
			s.Fatal("Failed to fullscreen window: ", err)
		}
		s.Log("Window fullscreened")

		if err := conn.CheckVideoIsActive(ctx); err != nil {
			s.Fatal("Preview is inactive after fullscreen window: ", err)
		}
	})

	runPreviewResizeTest(ctx, s, conn, func() {
		if err := conn.MinimizeWindow(ctx); err != nil {
			s.Fatal("Failed to minimize window: ", err)
		}
		s.Log("Window minimized")

		if err := conn.CheckVideoIsInactive(ctx); err != nil {
			s.Fatal("Preview is active after minimize window: ", err)
		}
	})
}

func runPreviewResizeTest(ctx context.Context, s *testing.State, conn *cca.Conn, resize func()) {
	if err := conn.RestoreWindow(ctx); err != nil {
		s.Fatal("Failed to restore window: ", err)
	}

	if err := conn.CheckVideoIsActive(ctx); err != nil {
		s.Fatal("Preview is inactive before resize window: ", err)
	}

	resize()

	if err := conn.RestoreWindow(ctx); err != nil {
		s.Fatal("Failed to restore window: ", err)
	}

	if err := conn.CheckVideoIsActive(ctx); err != nil {
		s.Fatal("Preview is inactive after restore window: ", err)
	}
}
