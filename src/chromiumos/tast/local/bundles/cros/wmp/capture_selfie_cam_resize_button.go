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
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CaptureSelfieCamResizeButton,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies the visibility change and resize behavior of the selfie cam's resize button",
		Contacts: []string{
			"michelefan@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
	})
}

func CaptureSelfieCamResizeButton(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	// Start chrome and enable the selfie cam feature and add 1 fake camera device.
	cr, err := chrome.New(ctx, chrome.EnableFeatures("CaptureModeSelfieCamera"), chrome.ExtraArgs("--use-fake-device-for-media-stream"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	screenRecordToggleButton := nodewith.HasClass("CaptureModeToggleButton").Name("Screen record")
	recordFullscreenToggleButton := nodewith.HasClass("CaptureModeToggleButton").Name("Record full screen")
	captureModeSettingsButton := nodewith.HasClass("CaptureModeToggleButton").Name("Settings")
	camera := nodewith.HasClass("CaptureModeOption").Name("fake_device_0")
	cameraPreviewWidget := nodewith.HasClass("CameraPreviewWidget")
	cameraPreviewResizeButton := nodewith.HasClass("CameraPreviewResizeButton")

	// Enter screen capture mode.
	if err := wmputils.EnsureCaptureModeActivated(tconn, true)(ctx); err != nil {
		s.Fatal("Failed to enable recording: ", err)
	}

	// Ensure case exit screen capture mode.
	defer wmputils.EnsureCaptureModeActivated(tconn, false)(cleanupCtx)

	ac := uiauto.New(tconn)

	if err := uiauto.Combine(
		"Select cameras from the settings menu",
		ac.LeftClick(screenRecordToggleButton),
		ac.LeftClick(recordFullscreenToggleButton),
		// Open the settings menu.
		ac.LeftClick(captureModeSettingsButton),
		// Wait for the fake camera option and click on it. The camera preview should show up.
		ac.LeftClick(camera),
		ac.WaitUntilExists(cameraPreviewWidget),
	)(ctx); err != nil {
		s.Fatal("Failed to select camera from the settings menu: ", err)
	}

	cameraPreviewLocation, err := ac.Location(ctx, cameraPreviewWidget)
	if err != nil {
		s.Fatal("Failed to get the camera preview location: ", err)
	}

	if err := uiauto.Combine(
		"Move the mouse to the camera preview",
		mouse.Move(tconn, cameraPreviewLocation.CenterPoint(), 0),
		ac.WaitUntilExists(cameraPreviewResizeButton),
	)(ctx); err != nil {
		s.Fatal("Resize button of the camera preview failed to show on mouse hover: ", err)
	}

	cameraPreviewResizeButtonLocation, err := ac.Location(ctx, cameraPreviewResizeButton)
	if err != nil {
		s.Fatal("Failed to get the location of the preview resize button: ", err)
	}

	if err := uiauto.Combine(
		"Resize the camera preview",
		mouse.Move(tconn, cameraPreviewResizeButtonLocation.CenterPoint(), 0),
		ac.LeftClick(cameraPreviewResizeButton),
	)(ctx); err != nil {
		s.Fatal("Failed to resize the camera preview: ", err)
	}

	cameraPreviewCollapsedLocation, err := ac.Location(ctx, cameraPreviewWidget)
	if cameraPreviewCollapsedLocation == cameraPreviewLocation {
		s.Fatal("Camera preview size didn't change correctly")
	}

	outsidePoint := coords.NewPoint(cameraPreviewCollapsedLocation.Left-1, cameraPreviewCollapsedLocation.Top-1)
	if err := uiauto.Combine(
		"Move mouse outside of the camera preview bounds",
		mouse.Move(tconn, outsidePoint, 0),
		ac.WaitUntilGone(cameraPreviewResizeButton),
	)(ctx); err != nil {
		s.Fatal("Resize button failed to disappear after the predefined period since the mouse existed the preview: ", err)
	}
}
