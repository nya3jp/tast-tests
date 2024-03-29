// Copyright 2022 The ChromiumOS Authors
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
		Timeout:      5 * time.Minute,
	})
}

func CaptureSelfieCamSelection(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	// Start chrome and enable the selfie cam feature, as well as add 2 fake camera devices.
	cr, err := chrome.New(ctx, chrome.EnableFeatures("CaptureModeSelfieCamera"), chrome.ExtraArgs("--use-fake-device-for-media-stream=device-count=2"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	screenRecordToggleButton := nodewith.HasClass("IconButton").Name("Screen record")
	recordFullscreenToggleButton := nodewith.HasClass("IconButton").Name("Record full screen")
	captureModeSettingsButton := nodewith.HasClass("IconButton").Name("Settings")
	// Note that the first (n = 0) "Off" option is for audio input. We want the camera's at n = 1.
	cameraOffButton := nodewith.HasClass("CaptureModeOption").Name("Off").Nth(1)
	firstCamera := nodewith.HasClass("CaptureModeOption").Name("fake_device_0")
	secondCamera := nodewith.HasClass("CaptureModeOption").Name("fake_device_1")
	cameraPreviewWidget := nodewith.HasClass("CameraPreviewWidget")

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
