// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wmp/wmputils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CaptureSelfieCamDragToSnap,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that we can select cameras from the capture settings menu",
		Contacts: []string{
			"conniekxu@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
	})
}

func CaptureSelfieCamDragToSnap(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	// Start chrome and enable the selfie cam feature, as well as add one fake camera devices.
	cr, err := chrome.New(ctx, chrome.EnableFeatures("CaptureModeSelfieCamera"), chrome.ExtraArgs("--use-fake-device-for-media-stream=device-count=1"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

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
		// Wait for the camera option and click it. The camera preview should show.
		ac.LeftClick(camera),
		ac.WaitUntilExists(cameraPreviewWidget),
	)(ctx); err != nil {
		s.Fatal("Failed to select the camera from the settings menu: ", err)
	}

	screens, err := display.GetInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the display info: ", err)
	}

	// Get the capture surface bounds. Since the capture source is fullscreen, the current capture surface bounds should be the current screen's work area.
	captureSurfaceBounds := screens[0].WorkArea

	captureSurfaceCenter := captureSurfaceBounds.CenterPoint()
	captureSurfaceWidth := captureSurfaceBounds.Width
	captureSurfaceHeight := captureSurfaceBounds.Height

	// Divide the capture surface into four rects through its center point.
	captureSurfaceTopLeft := coords.NewRect(0, 0, captureSurfaceWidth/2, captureSurfaceHeight/2)
	captureSurfaceTopRight := coords.NewRect(captureSurfaceCenter.X, 0, captureSurfaceWidth/2, captureSurfaceHeight/2)
	captureSurfaceBottomLeft := coords.NewRect(0, captureSurfaceCenter.Y, captureSurfaceWidth/2, captureSurfaceHeight/2)
	captureSurfaceBottomRight := coords.NewRect(captureSurfaceCenter.X, captureSurfaceCenter.Y, captureSurfaceWidth/2, captureSurfaceHeight/2)

	leftToTheCenter := captureSurfaceCenter.X - 10
	rightToTheCenter := captureSurfaceCenter.X + 10
	upToTheCenter := captureSurfaceCenter.Y - 10
	belowToTheCenter := captureSurfaceCenter.Y + 10

	// Drag camera preview to the four areas above and verify it's snapped to the correct position.
	if err = dragCameraPreviewTo(ctx, tconn, coords.NewPoint(leftToTheCenter, upToTheCenter), captureSurfaceTopLeft); err != nil {
		s.Fatal("Failed to drag the first window to the starting position: ", err)
	}

	if err = dragCameraPreviewTo(ctx, tconn, coords.NewPoint(rightToTheCenter, upToTheCenter), captureSurfaceTopRight); err != nil {
		s.Fatal("Failed to drag the first window to the starting position: ", err)
	}

	if err = dragCameraPreviewTo(ctx, tconn, coords.NewPoint(leftToTheCenter, belowToTheCenter), captureSurfaceBottomLeft); err != nil {
		s.Fatal("Failed to drag the first window to the starting position: ", err)
	}

	if err = dragCameraPreviewTo(ctx, tconn, coords.NewPoint(rightToTheCenter, belowToTheCenter), captureSurfaceBottomRight); err != nil {
		s.Fatal("Failed to drag the first window to the starting position: ", err)
	}
}

func dragCameraPreviewTo(ctx context.Context, tconn *chrome.TestConn, targetPoint coords.Point, targetRect coords.Rect) error {
	ac := uiauto.New(tconn)
	cameraPreviewWidget := nodewith.HasClass("CameraPreviewWidget")
	cameraPreviewLoc, err := ac.Location(ctx, cameraPreviewWidget)
	if err != nil {
		return errors.Wrap(err, "failed to get the location of the camera preview")
	}

	if err := uiauto.Combine("Drag camera preview to the target point and then release the drag",
		mouse.Move(tconn, cameraPreviewLoc.CenterPoint(), 0),
		mouse.Press(tconn, mouse.LeftButton),
		mouse.Move(tconn, targetPoint, 1*time.Second),
		mouse.Release(tconn, mouse.LeftButton),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to drag camera preview to the target point")
	}

	// Get camera preview's updated location after it's snapped.
	cameraPreviewLoc, err = ac.Location(ctx, cameraPreviewWidget)
	if err != nil {
		return errors.Wrap(err, "failed to get the updated location after the camera preview is snapped")
	}

	if !targetRect.Contains(*cameraPreviewLoc) {
		return errors.New("Camera preview is not snapped to correct position where it's supposed to be")
	}

	return nil
}
