// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIPreviewLongTakePhoto,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Preview Camera for 1 hour and take photo",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"camera_app", "chrome"},
		Fixture:      "ccaLaunched",
		Timeout:      62 * time.Minute, // Timeout for long duration.
	})
}

// CCAUIPreviewLongTakePhoto previews camera for long duration and takes photo.
func CCAUIPreviewLongTakePhoto(ctx context.Context, s *testing.State) {
	app := s.FixtValue().(cca.FixtureData).App()

	if err := app.MaximizeWindow(ctx); err != nil {
		s.Fatal("Failed to maximize window: ", err)
	}

	// Sleeping for 1 hour.
	if err := testing.Sleep(ctx, 60*time.Minute); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	// Checking if the camera is still active after the sleep.
	if err := app.WaitForState(ctx, "view-camera", true); err != nil {
		s.Fatal("Failed to wait for view-camera becomes true: ", err)
	}

	// Take photo using camera.
	if _, err := app.TakeSinglePhoto(ctx, cca.TimerOff); err != nil {
		s.Fatal("Failed to take photo: ", err)
	}
}
