// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/media/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIMultiCamera,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Opens CCA and verifies the multi-camera related use cases",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Fixture:      "ccaLaunched",
	})
}

func CCAUIMultiCamera(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(cca.FixtureData).Chrome
	tb := s.FixtValue().(cca.FixtureData).TestBridge()
	app := s.FixtValue().(cca.FixtureData).App()
	numCameras, err := app.GetNumOfCameras(ctx)
	if err != nil {
		s.Fatal("Can't get number of cameras: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)

	checkFacing := func() {
		// If it is a VM, there is no need to check the camera facing since it don't
		// have any builtin cameras.
		if vm.IsRunningOnVM() {
			return
		}

		// CCA should open back camera as default if the device is under tablet
		// mode and open front camera as default for clamshell mode.
		var isTabletMode bool
		if err := tconn.Eval(ctx,
			`tast.promisify(chrome.autotestPrivate.isTabletModeEnabled)()`,
			&isTabletMode); err != nil {
			s.Fatal("Failed to recognize device mode: ", err)
		}
		var defaultFacing cca.Facing
		if isTabletMode {
			defaultFacing = cca.FacingBack
		} else {
			defaultFacing = cca.FacingFront
		}

		if err := app.CheckCameraFacing(ctx, defaultFacing); err != nil {
			s.Fatal("Failed to open default camera facing: ", err)
		}
	}

	checkFacing()

	if numCameras > 1 {
		// Set grid option.
		gridEnabled, err := app.ToggleOption(ctx, cca.GridOption)
		if err != nil {
			s.Fatal("Toggle grid option failed: ", err)
		}

		// Switch camera.
		if err := app.SwitchCamera(ctx); err != nil {
			s.Fatal("Switch camera failed: ", err)
		}

		// Verify that grid option state is persistent.
		if err := app.CheckGridOption(ctx, gridEnabled); err != nil {
			s.Fatal("Check grid option failed: ", err)
		}
	} else if numCameras == 1 {
		if err := app.CheckVisible(ctx, cca.SwitchDeviceButton, false); err != nil {
			s.Fatal("Check switch button failed: ", err)
		}
	} else {
		s.Fatal("No camera found")
	}

	if err := app.Restart(ctx, tb); err != nil {
		var errJS *cca.ErrJS
		if errors.As(err, &errJS) {
			s.Error("There are JS errors when running CCA: ", err)
		} else {
			s.Fatal("Failed to restart CCA: ", err)
		}
	}

	// CCA should still open default camera regardless of what was opened last
	// time.
	checkFacing()
}
