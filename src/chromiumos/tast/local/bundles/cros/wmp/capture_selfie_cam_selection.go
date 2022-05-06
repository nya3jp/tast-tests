// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/wmp/wmputils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CaptureSelfieCamSelection,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that we can select cameras from the capture settings menu",
		Contacts: []string{
			"afakhry@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func CaptureSelfieCamSelection(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.EnableFeatures("CaptureModeSelfieCamera"), chrome.ExtraArgs("--use-fake-device-for-media-stream=device-count=2"),
		chrome.ExtraArgs("--ash-debug-shortcuts"), chrome.ExtraArgs("--ash-dev-shortcuts"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	screenRecordToggleButton := nodewith.ClassName("CaptureModeToggleButton").Name("Screen record")
	recordFullscreenToggleButton := nodewith.ClassName("CaptureModeToggleButton").Name("Record full screen")
	captureModeSettingsButton := nodewith.ClassName("CaptureModeToggleButton").Name("Settings")
	// Note that the first (n = 0) "Off" option is for audio input. We want the camera's at n = 1.
	cameraOffButton := nodewith.ClassName("CaptureModeOption").Name("Off").Nth(1)
	firstCamera := nodewith.ClassName("CaptureModeOption").Name("fake_device_0")
	secondCamera := nodewith.ClassName("CaptureModeOption").Name("fake_device_1")
	cameraPreviewWidget := nodewith.ClassName("CameraPreviewWidget")

	// Enter screen capture mode.
	if err := wmputils.EnsureCaptureModeActivated(tconn, true)(ctx); err != nil {
		s.Fatal("Failed to enable recording: ", err)
	}
	// Ensure case exit screen capture mode.
	defer wmputils.EnsureCaptureModeActivated(tconn, false)(cleanupCtx)

	ac := uiauto.New(tconn)
	if err := uiauto.Combine(
		"Select cameras from the settings menu",
		// The camera preview shows only in video recording mode.
		ac.LeftClick(screenRecordToggleButton),
		ac.LeftClick(recordFullscreenToggleButton),
		// Open settings menu.
		ac.LeftClick(captureModeSettingsButton),
		// Wait for the first camera option and click it. The camera preview shout show.
		ac.WaitUntilExists(firstCamera),
		ac.LeftClick(firstCamera),
		ac.WaitUntilExists(cameraPreviewWidget),
		// Click the "Off" button, the preview should be gone.
		ac.LeftClick(cameraOffButton),
		ac.WaitUntilGone(cameraPreviewWidget),
		// Now click the second camera option, the preview should show again.
		ac.LeftClick(secondCamera),
		ac.WaitUntilExists(cameraPreviewWidget),
	)(ctx); err != nil {
		s.Fatal("Failed to select cameras from the settings menu: ", err)
	}

}
