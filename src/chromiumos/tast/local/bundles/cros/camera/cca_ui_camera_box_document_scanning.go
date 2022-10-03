// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUICameraBoxDocumentScanning,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that CCA can scan document on preview via CameraBox",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:camerabox"},
		SoftwareDeps: []string{"camera_app", "chrome", "ondevice_document_scanner_rootfs_or_dlc", caps.BuiltinOrVividCamera},
		Data:         []string{"testing_rsa", "document_scene.jpg"},
		Vars:         []string{"chart"},
		Fixture:      "ccaLaunchedInCameraBox",
		Params: []testing.Param{{
			Name:      "back",
			ExtraAttr: []string{"camerabox_facing_back"},
			Val:       cca.FacingBack,
		}, {
			Name:      "front",
			ExtraAttr: []string{"camerabox_facing_front"},
			Val:       cca.FacingFront,
		}},
	})
}

// CCAUICameraBoxDocumentScanning tests that the detected document corners will be shown while under document scan mode.
func CCAUICameraBoxDocumentScanning(ctx context.Context, s *testing.State) {
	prepareChart := s.FixtValue().(cca.FixtureData).PrepareChart
	if err := prepareChart(ctx, s.RequiredVar("chart"), s.DataPath("testing_rsa"), s.DataPath("document_scene.jpg")); err != nil {
		s.Fatal("Failed to prepare chart: ", err)
	}
	s.FixtValue().(cca.FixtureData).SetDebugParams(cca.DebugParams{SaveScreenshotWhenFail: true})

	app := s.FixtValue().(cca.FixtureData).App()
	facing := s.Param().(cca.Facing)

	if curFacing, err := app.GetFacing(ctx); err != nil {
		s.Fatal("Failed to get facing: ", err)
	} else if curFacing != facing {
		if err := app.SwitchCamera(ctx); err != nil {
			s.Fatal("Failed to switch camera: ", err)
		}
		if err := app.CheckFacing(ctx, facing); err != nil {
			s.Fatalf("Failed to switch to the target camera %v: %v", facing, err)
		}
	}

	// For the devices with document mode enabled by default, the scan mode button should be visible
	// upon launching the app.
	if visible, err := app.Visible(ctx, cca.ScanModeButton); err != nil {
		s.Fatal("Failed to check visibility of scan mode button: ", err)
	} else if !visible {
		if err := app.EnableDocumentMode(ctx); err != nil {
			s.Fatal("Failed to enable scan mode: ", err)
		}
	}

	// Switch to scan mode.
	if err := app.SwitchMode(ctx, cca.Scan); err != nil {
		s.Fatal("Failed to switch to scan mode: ", err)
	}

	// Dismiss document dialog.
	if err := app.WaitForVisibleState(ctx, cca.DocumentDialogButton, true); err == nil {
		if err := app.Click(ctx, cca.DocumentDialogButton); err != nil {
			s.Fatal(err, "failed to click the document dialog button")
		}
	}

	// Verify that document corners are shown in the preview.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		result, err := app.HasClass(ctx, cca.DocumentCornerOverlay, "show-corner-indicator")
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to check class of the document scan overlay"))
		} else if !result {
			return errors.Wrap(err, "no document is found")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait for corner indicator show up: ", err)
	}
}
